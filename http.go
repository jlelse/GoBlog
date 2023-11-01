package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/dchest/captcha"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/justinas/alice"
	"github.com/klauspost/compress/flate"
	"github.com/samber/lo"
	"go.goblog.app/app/pkgs/httpcompress"
	"go.goblog.app/app/pkgs/maprouter"
	"go.goblog.app/app/pkgs/plugintypes"
	"golang.org/x/net/context"
)

const (
	contentType  = "Content-Type"
	userAgent    = "User-Agent"
	appUserAgent = "GoBlog"

	blogKey contextKey = "blog"
	pathKey contextKey = "httpPath"
)

func (a *goBlog) startServer() (err error) {
	log.Println("Start server(s)...")
	// Load router
	a.reloadRouter()
	// Set basic middlewares
	h := alice.New()
	h = h.Append(middleware.Heartbeat("/ping"))
	if a.cfg.Server.Logging {
		h = h.Append(a.logMiddleware)
	}
	h = h.Append(middleware.Recoverer, httpcompress.Compress(flate.BestCompression))
	if a.cfg.Server.SecurityHeaders {
		h = h.Append(a.securityHeaders)
	}
	// Add plugin middlewares
	middlewarePlugins := lo.Map(a.getPlugins(pluginMiddlewareType), func(item any, index int) plugintypes.Middleware { return item.(plugintypes.Middleware) })
	sort.Slice(middlewarePlugins, func(i, j int) bool {
		// Sort with descending prio
		return middlewarePlugins[i].Prio() > middlewarePlugins[j].Prio()
	})
	for _, plugin := range middlewarePlugins {
		h = h.Append(plugin.Handler)
	}
	// Finally...
	finalHandler := h.ThenFunc(func(w http.ResponseWriter, r *http.Request) {
		a.d.ServeHTTP(w, r)
	})
	// Start Onion service
	if a.cfg.Server.Tor {
		go func() {
			if err := a.startOnionService(finalHandler); err != nil {
				log.Println("Tor failed:", err.Error())
			}
		}()
	}
	// Start server
	if a.cfg.Server.HttpsRedirect {
		go func() {
			// Start HTTP server for redirects
			h := http.Handler(http.HandlerFunc(a.redirectToHttps))
			if m := a.getAutocertManager(); m != nil {
				h = m.HTTPHandler(h)
			}
			httpServer := &http.Server{
				Addr:              ":80",
				Handler:           h,
				ReadHeaderTimeout: 1 * time.Minute,
				ReadTimeout:       5 * time.Minute,
				WriteTimeout:      5 * time.Minute,
			}
			a.shutdown.Add(shutdownServer(httpServer, "http server"))
			if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Println("Failed to start HTTP server:", err.Error())
			}
		}()
	}
	s := &http.Server{
		Handler:           finalHandler,
		ReadHeaderTimeout: 1 * time.Minute,
		ReadTimeout:       5 * time.Minute,
		WriteTimeout:      5 * time.Minute,
	}
	a.shutdown.Add(shutdownServer(s, "main server"))
	s.Addr = ":" + strconv.Itoa(a.cfg.Server.Port)
	if a.cfg.Server.PublicHTTPS {
		s.TLSConfig = a.getAutocertManager().TLSConfig()
		err = s.ListenAndServeTLS("", "")
	} else if a.cfg.Server.manualHttps {
		err = s.ListenAndServeTLS(a.cfg.Server.HttpsCert, a.cfg.Server.HttpsKey)
	} else {
		err = s.ListenAndServe()
	}
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

func shutdownServer(s *http.Server, name string) func() {
	return func() {
		toc, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		if err := s.Shutdown(toc); err != nil {
			log.Printf("Error on server shutdown (%v): %v", name, err)
		}
		log.Println("Stopped server:", name)
	}
}

func (*goBlog) redirectToHttps(w http.ResponseWriter, r *http.Request) {
	requestHost, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		requestHost = r.Host
	}
	w.Header().Set("Connection", "close")
	http.Redirect(w, r, fmt.Sprintf("https://%s%s", requestHost, r.URL.RequestURI()), http.StatusMovedPermanently)
}

const (
	paginationPath = "/page/{page:[0-9-]+}"
	feedPath       = ".{feed:(rss|json|atom|min\\.rss|min\\.json|min\\.atom)}"
)

func (a *goBlog) reloadRouter() {
	a.d = a.buildRouter()
}

func (a *goBlog) buildRouter() http.Handler {
	mapRouter := &maprouter.MapRouter{
		Handlers: map[string]http.Handler{},
	}
	if shn := a.cfg.Server.shortPublicHostname; shn != "" {
		mapRouter.Handlers[shn] = http.HandlerFunc(a.redirectShortDomain)
	}
	if mhn := a.cfg.Server.mediaHostname; mhn != "" && !a.isPrivate() {
		mr := chi.NewMux()

		mr.Use(middleware.RedirectSlashes)
		mr.Use(middleware.CleanPath)

		mr.Group(a.mediaFilesRouter)

		mapRouter.Handlers[mhn] = mr
	}

	// Default router
	r := chi.NewMux()

	// Basic middleware
	r.Use(fixHTTPHandler)
	r.Use(middleware.RedirectSlashes)
	r.Use(middleware.CleanPath)

	// Tor
	if a.cfg.Server.Tor {
		r.Use(a.addOnionLocation)
	}

	// Cache
	if cache := a.cfg.Cache; cache != nil && !cache.Enable {
		r.Use(middleware.NoCache)
	}

	// No Index Header
	if a.isPrivate() {
		r.Use(noIndexHeader)
	}

	// Login and captcha middleware
	r.Use(a.checkIsLogin)
	r.Use(a.checkIsCaptcha)

	// Login
	r.Group(a.loginRouter)

	// Micropub
	r.Route(micropubPath, a.micropubRouter)

	// IndieAuth
	r.Group(a.indieAuthRouter)

	// ActivityPub and stuff
	r.Group(a.activityPubRouter)

	// Webmentions
	r.Route(webmentionPath, a.webmentionsRouter)

	// Notifications
	r.Route(notificationsPath, a.notificationsRouter)

	// Assets
	r.Group(a.assetsRouter)

	// Static files
	r.Group(a.staticFilesRouter)

	// Media files
	r.Route("/m", a.mediaFilesRouter)

	// Profile image
	r.Group(a.profileImageRouter)

	// Other routes
	r.Route("/-", a.otherRoutesRouter)

	// Captcha
	r.Handle("/captcha/*", captcha.Server(500, 250))

	// Blogs
	for blog, blogConfig := range a.cfg.Blogs {
		r.Group(a.blogRouter(blog, blogConfig))
	}

	// Sitemap
	r.With(a.privateModeHandler, cacheLoggedIn, a.cacheMiddleware).Get(sitemapPath, a.serveSitemap)

	// IndexNow
	if a.indexNowEnabled() {
		if inkey := a.indexNowKey(); len(inkey) > 0 {
			r.With(cacheLoggedIn, a.cacheMiddleware).Get("/"+string(inkey)+".txt", a.serveIndexNow)
		}
	}

	// Robots.txt
	r.With(cacheLoggedIn, a.cacheMiddleware).Get(robotsTXTPath, a.serveRobotsTXT)

	// Favicon
	if !hasStaticPath("favicon.ico") {
		r.With(a.cacheMiddleware).Get("/favicon.ico", a.serve404)
	}

	r.NotFound(a.servePostsAliasesRedirects())

	r.MethodNotAllowed(a.serveNotAllowed)

	mapRouter.DefaultHandler = r
	return alice.New(headAsGetHandler).Then(mapRouter)
}

func (a *goBlog) servePostsAliasesRedirects() http.HandlerFunc {
	// Private mode
	alicePrivate := alice.New(a.privateModeHandler)
	// Return handler func
	return func(w http.ResponseWriter, r *http.Request) {
		// Only allow GET requests
		if r.Method != http.MethodGet {
			a.serveNotAllowed(w, r)
			return
		}
		// Check if post or alias
		path := r.URL.Path
		row, err := a.db.QueryRow(`
		-- normal posts
		select 'post', status, visibility, 200 from posts where path = @path
		union all
		-- short paths
		select 'alias', path, '', 301 from shortpath where printf('/s/%x', id) = @path
		union all
		-- post aliases
		select 'alias', path, '', 302 from post_parameters where parameter = 'aliases' and value = @path
		union all
		-- deleted posts
		select 'deleted', '', '', 410 from deleted where path = @path
		-- just select the first result
		limit 1
		`, sql.Named("path", path))
		if err != nil {
			a.serveError(w, r, err.Error(), http.StatusInternalServerError)
			return
		}
		var pathType, value1, value2 string
		var status int
		err = row.Scan(&pathType, &value1, &value2, &status)
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				// Error
				a.serveError(w, r, err.Error(), http.StatusInternalServerError)
				return
			}
			// No result, continue...
		} else {
			// Found post or alias
			switch pathType {
			case "post":
				// Check status
				switch postStatus(value1) {
				case statusPublished:
					// Check visibility
					switch postVisibility(value2) {
					case visibilityPublic, visibilityUnlisted:
						alicePrivate.Append(a.checkActivityStreamsRequest, a.cacheMiddleware).ThenFunc(a.servePost).ServeHTTP(w, r)
					default: // private, etc.
						alice.New(a.authMiddleware).ThenFunc(a.servePost).ServeHTTP(w, r)
					}
					return
				case statusPublishedDeleted:
					if a.isLoggedIn(r) {
						a.servePost(w, r)
						return
					}
					alicePrivate.Append(a.cacheMiddleware).ThenFunc(a.serve410).ServeHTTP(w, r)
					return
				default: // draft, scheduled, etc.
					alice.New(a.authMiddleware).ThenFunc(a.servePost).ServeHTTP(w, r)
					return
				}
			case "alias":
				// Is alias, redirect
				alicePrivate.Append(cacheLoggedIn, a.cacheMiddleware).ThenFunc(func(w http.ResponseWriter, r *http.Request) {
					http.Redirect(w, r, value1, status)
				}).ServeHTTP(w, r)
				return
			case "deleted":
				// Is deleted, serve 410
				alicePrivate.Append(a.cacheMiddleware).ThenFunc(a.serve410).ServeHTTP(w, r)
				return
			}
		}
		// No post, check template assets (dynamically registered), regex redirects or serve 404 error
		alice.New(a.cacheMiddleware, a.checkTemplateAssets, a.checkRegexRedirects).ThenFunc(a.serve404).ServeHTTP(w, r)
	}
}

func (a *goBlog) getAppRouter() http.Handler {
	for {
		// Wait until router is ready
		if a.d != nil {
			break
		}
		time.Sleep(time.Millisecond * 100)
	}
	return a.d
}
