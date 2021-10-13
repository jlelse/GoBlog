package main

import (
	"compress/flate"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/dchest/captcha"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/justinas/alice"
	"go.goblog.app/app/pkgs/maprouter"
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
	a.d, err = a.buildRouter()
	if err != nil {
		return err
	}
	// Set basic middlewares
	h := alice.New()
	if a.cfg.Server.Logging {
		h = h.Append(a.logMiddleware)
	}
	h = h.Append(middleware.Recoverer, middleware.Compress(flate.DefaultCompression), middleware.Heartbeat("/ping"))
	if a.httpsConfigured(false) {
		h = h.Append(a.securityHeaders)
	}
	finalHandler := h.Then(a.d)
	// Start Onion service
	if a.cfg.Server.Tor {
		go func() {
			if err := a.startOnionService(finalHandler); err != nil {
				log.Println("Tor failed:", err.Error())
			}
		}()
	}
	// Start server
	s := &http.Server{
		Handler:      finalHandler,
		ReadTimeout:  5 * time.Minute,
		WriteTimeout: 5 * time.Minute,
	}
	a.shutdown.Add(shutdownServer(s, "main server"))
	if a.cfg.Server.PublicHTTPS || a.cfg.Server.TailscaleHTTPS {
		go func() {
			// Start HTTP server for redirects
			httpServer := &http.Server{
				Addr:         ":80",
				Handler:      http.HandlerFunc(a.redirectToHttps),
				ReadTimeout:  5 * time.Minute,
				WriteTimeout: 5 * time.Minute,
			}
			a.shutdown.Add(shutdownServer(httpServer, "http server"))
			if err := a.listenAndServe(httpServer); err != nil && err != http.ErrServerClosed {
				log.Println("Failed to start HTTP server:", err.Error())
			}
		}()
		// Start HTTPS
		s.Addr = ":443"
		if err = a.listenAndServe(s); err != nil && err != http.ErrServerClosed {
			return err
		}
	} else {
		s.Addr = ":" + strconv.Itoa(a.cfg.Server.Port)
		if err = a.listenAndServe(s); err != nil && err != http.ErrServerClosed {
			return err
		}
	}
	return nil
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
	feedPath       = ".{feed:rss|json|atom}"
)

func (a *goBlog) buildRouter() (http.Handler, error) {
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
		mr.Use(middleware.GetHead)

		mr.Group(a.mediaFilesRouter)

		mapRouter.Handlers[mhn] = mr
	}

	// Default router
	r := chi.NewMux()

	// Basic middleware
	r.Use(fixHTTPHandler)
	r.Use(middleware.RedirectSlashes)
	r.Use(middleware.CleanPath)
	r.Use(middleware.GetHead)

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
	r.Route("/indieauth", a.indieAuthRouter)

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

	// Captcha
	r.Handle("/captcha/*", captcha.Server(500, 250))

	// Short paths
	r.With(a.privateModeHandler, cacheLoggedIn, a.cacheMiddleware).Get("/s/{id:[0-9a-fA-F]+}", a.redirectToLongPath)

	// Blogs
	for blog, blogConfig := range a.cfg.Blogs {
		r.Group(a.blogRouter(blog, blogConfig))
	}

	// Sitemap
	r.With(a.privateModeHandler, cacheLoggedIn, a.cacheMiddleware).Get(sitemapPath, a.serveSitemap)

	// Robots.txt
	r.With(cacheLoggedIn, a.cacheMiddleware).Get(robotsTXTPath, a.serveRobotsTXT)

	r.NotFound(a.servePostsAliasesRedirects())

	r.MethodNotAllowed(a.serveNotAllowed)

	mapRouter.DefaultHandler = r
	return mapRouter, nil
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
		row, err := a.db.queryRow(`
		select 'post', status from posts where path = @path
		union all
		select 'alias', path from post_parameters where parameter = 'aliases' and value = @path
		union all
		select 'deleted', '' from deleted where path = @path
		limit 1
		`, sql.Named("path", path))
		if err != nil {
			a.serveError(w, r, err.Error(), http.StatusInternalServerError)
			return
		}
		var postAliasType, value string
		err = row.Scan(&postAliasType, &value)
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				// Error
				a.serveError(w, r, err.Error(), http.StatusInternalServerError)
				return
			}
			// No result, continue...
		} else {
			// Found post or alias
			switch postAliasType {
			case "post":
				// Is post, check status
				switch postStatus(value) {
				case statusPublished, statusUnlisted:
					alicePrivate.Append(a.checkActivityStreamsRequest, a.cacheMiddleware).ThenFunc(a.servePost).ServeHTTP(w, r)
					return
				case statusDraft, statusPrivate:
					alice.New(a.authMiddleware).ThenFunc(a.servePost).ServeHTTP(w, r)
					return
				}
			case "alias":
				// Is alias, redirect
				alicePrivate.Append(cacheLoggedIn, a.cacheMiddleware).ThenFunc(func(w http.ResponseWriter, r *http.Request) {
					http.Redirect(w, r, value, http.StatusFound)
				}).ServeHTTP(w, r)
				return
			case "deleted":
				// Is deleted, serve 410
				alicePrivate.Append(a.cacheMiddleware).ThenFunc(func(w http.ResponseWriter, r *http.Request) {
					a.serve410(w, r)
				}).ServeHTTP(w, r)
				return
			}
		}
		// No post, check regex redirects or serve 404 error
		alice.New(a.cacheMiddleware, a.checkRegexRedirects).ThenFunc(a.serve404).ServeHTTP(w, r)
	}
}
