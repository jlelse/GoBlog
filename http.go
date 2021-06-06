package main

import (
	"compress/flate"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/caddyserver/certmagic"
	"github.com/dchest/captcha"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	servertiming "github.com/mitchellh/go-server-timing"
	"golang.org/x/net/context"
)

const (
	contentType = "Content-Type"

	charsetUtf8Suffix = "; charset=utf-8"

	contentTypeHTML          = "text/html"
	contentTypeXML           = "text/xml"
	contentTypeJSON          = "application/json"
	contentTypeWWWForm       = "application/x-www-form-urlencoded"
	contentTypeMultipartForm = "multipart/form-data"
	contentTypeAS            = "application/activity+json"
	contentTypeRSS           = "application/rss+xml"
	contentTypeATOM          = "application/atom+xml"
	contentTypeJSONFeed      = "application/feed+json"

	contentTypeHTMLUTF8 = contentTypeHTML + charsetUtf8Suffix
	contentTypeXMLUTF8  = contentTypeXML + charsetUtf8Suffix
	contentTypeJSONUTF8 = contentTypeJSON + charsetUtf8Suffix
	contentTypeASUTF8   = contentTypeAS + charsetUtf8Suffix

	userAgent    = "User-Agent"
	appUserAgent = "GoBlog"
)

func (a *goBlog) startServer() (err error) {
	// Start
	a.d = &dynamicHandler{}
	// Set basic middlewares
	var finalHandler http.Handler = a.d
	if a.cfg.Server.PublicHTTPS || a.cfg.Server.SecurityHeaders {
		finalHandler = a.securityHeaders(finalHandler)
	}
	finalHandler = servertiming.Middleware(finalHandler, nil)
	finalHandler = middleware.Heartbeat("/ping")(finalHandler)
	finalHandler = middleware.Compress(flate.DefaultCompression)(finalHandler)
	finalHandler = middleware.Recoverer(finalHandler)
	if a.cfg.Server.Logging {
		finalHandler = a.logMiddleware(finalHandler)
	}
	// Create routers that don't change
	if err = a.buildStaticHandlersRouters(); err != nil {
		return err
	}
	// Load router
	if err = a.reloadRouter(); err != nil {
		return err
	}
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
	addShutdownFunc(shutdownServer(s, "main server"))
	if a.cfg.Server.PublicHTTPS {
		// Configure
		certmagic.Default.Storage = &certmagic.FileStorage{Path: "data/https"}
		certmagic.DefaultACME.Email = a.cfg.Server.LetsEncryptMail
		certmagic.DefaultACME.CA = certmagic.LetsEncryptProductionCA
		// Start HTTP server for redirects
		httpServer := &http.Server{
			Addr:         ":http",
			Handler:      http.HandlerFunc(redirectToHttps),
			ReadTimeout:  5 * time.Minute,
			WriteTimeout: 5 * time.Minute,
		}
		addShutdownFunc(shutdownServer(httpServer, "http server"))
		go func() {
			if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Println("Failed to start HTTP server:", err.Error())
			}
		}()
		// Start HTTPS
		s.Addr = ":https"
		hosts := []string{a.cfg.Server.publicHostname}
		if a.cfg.Server.shortPublicHostname != "" {
			hosts = append(hosts, a.cfg.Server.shortPublicHostname)
		}
		listener, e := certmagic.Listen(hosts)
		if e != nil {
			return e
		}
		if err = s.Serve(listener); err != nil && err != http.ErrServerClosed {
			return err
		}
	} else {
		s.Addr = ":" + strconv.Itoa(a.cfg.Server.Port)
		if err = s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return err
		}
	}
	return nil
}

func shutdownServer(s *http.Server, name string) func() {
	return func() {
		toc, c := context.WithTimeout(context.Background(), 5*time.Second)
		_ = s.Shutdown(toc)
		c()
		log.Println("Stopped server:", name)
	}
}

func redirectToHttps(w http.ResponseWriter, r *http.Request) {
	requestHost, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		requestHost = r.Host
	}
	w.Header().Set("Connection", "close")
	http.Redirect(w, r, fmt.Sprintf("https://%s%s", requestHost, r.URL.RequestURI()), http.StatusMovedPermanently)
}

func (a *goBlog) reloadRouter() error {
	h, err := a.buildDynamicRouter()
	if err != nil {
		return err
	}
	a.d.swapHandler(h)
	a.cache.purge()
	return nil
}

const (
	paginationPath = "/page/{page:[0-9-]+}"
	feedPath       = ".{feed:rss|json|atom}"
)

func (a *goBlog) buildStaticHandlersRouters() error {
	if pm := a.cfg.PrivateMode; pm != nil && pm.Enabled {
		a.privateMode = true
		a.privateModeHandler = append(a.privateModeHandler, a.authMiddleware)
	} else {
		a.privateMode = false
		a.privateModeHandler = []func(http.Handler) http.Handler{}
	}

	a.captchaHandler = captcha.Server(500, 250)

	a.micropubRouter = chi.NewRouter()
	a.micropubRouter.Use(a.checkIndieAuth)
	a.micropubRouter.Get("/", a.serveMicropubQuery)
	a.micropubRouter.Post("/", a.serveMicropubPost)
	a.micropubRouter.Post(micropubMediaSubPath, a.serveMicropubMedia)

	a.indieAuthRouter = chi.NewRouter()
	a.indieAuthRouter.Get("/", a.indieAuthRequest)
	a.indieAuthRouter.With(a.authMiddleware).Post("/accept", a.indieAuthAccept)
	a.indieAuthRouter.Post("/", a.indieAuthVerification)
	a.indieAuthRouter.Get("/token", a.indieAuthToken)
	a.indieAuthRouter.Post("/token", a.indieAuthToken)

	a.webmentionsRouter = chi.NewRouter()
	if wm := a.cfg.Webmention; wm != nil && !wm.DisableReceiving {
		a.webmentionsRouter.Post("/", a.handleWebmention)
		a.webmentionsRouter.Group(func(r chi.Router) {
			// Authenticated routes
			r.Use(a.authMiddleware)
			r.Get("/", a.webmentionAdmin)
			r.Get(paginationPath, a.webmentionAdmin)
			r.Post("/delete", a.webmentionAdminDelete)
			r.Post("/approve", a.webmentionAdminApprove)
			r.Post("/reverify", a.webmentionAdminReverify)
		})
	}

	a.notificationsRouter = chi.NewRouter()
	a.notificationsRouter.Use(a.authMiddleware)
	a.notificationsRouter.Get("/", a.notificationsAdmin)
	a.notificationsRouter.Get(paginationPath, a.notificationsAdmin)
	a.notificationsRouter.Post("/delete", a.notificationsAdminDelete)

	if ap := a.cfg.ActivityPub; ap != nil && ap.Enabled {
		a.activitypubRouter = chi.NewRouter()
		a.activitypubRouter.Post("/inbox/{blog}", a.apHandleInbox)
		a.activitypubRouter.Post("/{blog}/inbox", a.apHandleInbox)
	}

	a.editorRouter = chi.NewRouter()
	a.editorRouter.Use(a.authMiddleware)
	a.editorRouter.Get("/", a.serveEditor)
	a.editorRouter.Post("/", a.serveEditorPost)

	a.commentsRouter = chi.NewRouter()
	a.commentsRouter.Use(a.privateModeHandler...)
	a.commentsRouter.With(a.cache.cacheMiddleware, noIndexHeader).Get("/{id:[0-9]+}", a.serveComment)
	a.commentsRouter.With(a.captchaMiddleware).Post("/", a.createComment)
	a.commentsRouter.Group(func(r chi.Router) {
		// Admin
		r.Use(a.authMiddleware)
		r.Get("/", a.commentsAdmin)
		r.Get(paginationPath, a.commentsAdmin)
		r.Post("/delete", a.commentsAdminDelete)
	})

	a.searchRouter = chi.NewRouter()
	a.searchRouter.Use(a.privateModeHandler...)
	a.searchRouter.Use(a.cache.cacheMiddleware)
	a.searchRouter.Get("/", a.serveSearch)
	a.searchRouter.Post("/", a.serveSearch)
	searchResultPath := "/" + searchPlaceholder
	a.searchRouter.Get(searchResultPath, a.serveSearchResult)
	a.searchRouter.Get(searchResultPath+feedPath, a.serveSearchResult)
	a.searchRouter.Get(searchResultPath+paginationPath, a.serveSearchResult)

	a.setBlogMiddlewares = map[string]func(http.Handler) http.Handler{}
	a.sectionMiddlewares = map[string]func(http.Handler) http.Handler{}
	a.taxonomyMiddlewares = map[string]func(http.Handler) http.Handler{}
	a.photosMiddlewares = map[string]func(http.Handler) http.Handler{}
	a.searchMiddlewares = map[string]func(http.Handler) http.Handler{}
	a.customPagesMiddlewares = map[string]func(http.Handler) http.Handler{}
	a.commentsMiddlewares = map[string]func(http.Handler) http.Handler{}

	for blog, blogConfig := range a.cfg.Blogs {
		sbm := middleware.WithValue(blogContextKey, blog)
		a.setBlogMiddlewares[blog] = sbm

		blogPath := a.blogPath(blog)

		for _, section := range blogConfig.Sections {
			if section.Name != "" {
				secPath := blogPath + "/" + section.Name
				a.sectionMiddlewares[secPath] = middleware.WithValue(indexConfigKey, &indexConfig{
					path:    secPath,
					section: section,
				})
			}
		}

		for _, taxonomy := range blogConfig.Taxonomies {
			if taxonomy.Name != "" {
				taxPath := blogPath + "/" + taxonomy.Name
				a.taxonomyMiddlewares[taxPath] = middleware.WithValue(taxonomyContextKey, taxonomy)
			}
		}

		if blogConfig.Photos != nil && blogConfig.Photos.Enabled {
			a.photosMiddlewares[blog] = middleware.WithValue(indexConfigKey, &indexConfig{
				path:            blogPath + blogConfig.Photos.Path,
				parameter:       blogConfig.Photos.Parameter,
				title:           blogConfig.Photos.Title,
				description:     blogConfig.Photos.Description,
				summaryTemplate: templatePhotosSummary,
			})
		}

		if blogConfig.Search != nil && blogConfig.Search.Enabled {
			a.searchMiddlewares[blog] = middleware.WithValue(pathContextKey, blogPath+blogConfig.Search.Path)
		}

		for _, cp := range blogConfig.CustomPages {
			a.customPagesMiddlewares[cp.Path] = middleware.WithValue(customPageContextKey, cp)
		}

		if commentsConfig := blogConfig.Comments; commentsConfig != nil && commentsConfig.Enabled {
			a.commentsMiddlewares[blog] = middleware.WithValue(pathContextKey, blogPath+"/comment")
		}
	}

	return nil
}

var (
	taxValueMiddlewares = map[string]func(http.Handler) http.Handler{}
)

func (a *goBlog) buildDynamicRouter() (*chi.Mux, error) {
	r := chi.NewRouter()

	// Basic middleware
	r.Use(a.redirectShortDomain)
	r.Use(middleware.RedirectSlashes)
	r.Use(middleware.CleanPath)
	r.Use(middleware.GetHead)
	if !a.cfg.Cache.Enable {
		r.Use(middleware.NoCache)
	}

	// No Index Header
	if a.privateMode {
		r.Use(noIndexHeader)
	}

	// Login middleware etc.
	r.Use(a.checkIsLogin)
	r.Use(a.checkIsCaptcha)
	r.Use(a.checkLoggedIn)

	// Logout
	r.With(a.authMiddleware).Get("/login", serveLogin)
	r.With(a.authMiddleware).Get("/logout", a.serveLogout)

	// Micropub
	r.Mount(micropubPath, a.micropubRouter)

	// IndieAuth
	r.Mount("/indieauth", a.indieAuthRouter)

	// ActivityPub and stuff
	if ap := a.cfg.ActivityPub; ap != nil && ap.Enabled {
		r.Mount("/activitypub", a.activitypubRouter)
		r.With(a.cache.cacheMiddleware).Get("/.well-known/webfinger", a.apHandleWebfinger)
		r.With(a.cache.cacheMiddleware).Get("/.well-known/host-meta", handleWellKnownHostMeta)
		r.With(a.cache.cacheMiddleware).Get("/.well-known/nodeinfo", a.serveNodeInfoDiscover)
		r.With(a.cache.cacheMiddleware).Get("/nodeinfo", a.serveNodeInfo)
	}

	// Webmentions
	r.Mount(webmentionPath, a.webmentionsRouter)

	// Notifications
	r.Mount(notificationsPath, a.notificationsRouter)

	// Posts
	pp, err := a.db.allPostPaths(statusPublished)
	if err != nil {
		return nil, err
	}
	r.Group(func(r chi.Router) {
		r.Use(a.privateModeHandler...)
		r.Use(a.checkActivityStreamsRequest, a.cache.cacheMiddleware)
		for _, path := range pp {
			r.Get(path, a.servePost)
		}
	})

	// Drafts
	dp, err := a.db.allPostPaths(statusDraft)
	if err != nil {
		return nil, err
	}
	r.Group(func(r chi.Router) {
		r.Use(a.authMiddleware)
		for _, path := range dp {
			r.Get(path, a.servePost)
		}
	})

	// Post aliases
	allPostAliases, err := a.db.allPostAliases()
	if err != nil {
		return nil, err
	}
	r.Group(func(r chi.Router) {
		r.Use(a.privateModeHandler...)
		r.Use(a.cache.cacheMiddleware)
		for _, path := range allPostAliases {
			r.Get(path, a.servePostAlias)
		}
	})

	// Assets
	for _, path := range a.allAssetPaths() {
		r.Get(path, a.serveAsset)
	}

	// Static files
	for _, path := range allStaticPaths() {
		r.Get(path, a.serveStaticFile)
	}

	// Media files
	r.With(a.privateModeHandler...).Get(`/m/{file:[0-9a-fA-F]+(\.[0-9a-zA-Z]+)?}`, a.serveMediaFile)

	// Captcha
	r.Handle("/captcha/*", a.captchaHandler)

	// Short paths
	r.With(a.privateModeHandler...).With(a.cache.cacheMiddleware).Get("/s/{id:[0-9a-fA-F]+}", a.redirectToLongPath)

	for blog, blogConfig := range a.cfg.Blogs {
		blogPath := a.blogPath(blog)

		sbm := a.setBlogMiddlewares[blog]

		// Sections
		r.Group(func(r chi.Router) {
			r.Use(a.privateModeHandler...)
			r.Use(a.cache.cacheMiddleware, sbm)
			for _, section := range blogConfig.Sections {
				if section.Name != "" {
					secPath := blogPath + "/" + section.Name
					r.Group(func(r chi.Router) {
						r.Use(a.sectionMiddlewares[secPath])
						r.Get(secPath, a.serveIndex)
						r.Get(secPath+feedPath, a.serveIndex)
						r.Get(secPath+paginationPath, a.serveIndex)
					})
				}
			}
		})

		// Taxonomies
		for _, taxonomy := range blogConfig.Taxonomies {
			if taxonomy.Name != "" {
				taxPath := blogPath + "/" + taxonomy.Name
				taxValues, err := a.db.allTaxonomyValues(blog, taxonomy.Name)
				if err != nil {
					return nil, err
				}
				r.Group(func(r chi.Router) {
					r.Use(a.privateModeHandler...)
					r.Use(a.cache.cacheMiddleware, sbm)
					r.With(a.taxonomyMiddlewares[taxPath]).Get(taxPath, a.serveTaxonomy)
					for _, tv := range taxValues {
						r.Group(func(r chi.Router) {
							vPath := taxPath + "/" + urlize(tv)
							if _, ok := taxValueMiddlewares[vPath]; !ok {
								taxValueMiddlewares[vPath] = middleware.WithValue(indexConfigKey, &indexConfig{
									path:     vPath,
									tax:      taxonomy,
									taxValue: tv,
								})
							}
							r.Use(taxValueMiddlewares[vPath])
							r.Get(vPath, a.serveIndex)
							r.Get(vPath+feedPath, a.serveIndex)
							r.Get(vPath+paginationPath, a.serveIndex)
						})
					}
				})
			}
		}

		// Photos
		if blogConfig.Photos != nil && blogConfig.Photos.Enabled {
			r.Group(func(r chi.Router) {
				r.Use(a.privateModeHandler...)
				r.Use(a.cache.cacheMiddleware, sbm, a.photosMiddlewares[blog])
				photoPath := blogPath + blogConfig.Photos.Path
				r.Get(photoPath, a.serveIndex)
				r.Get(photoPath+feedPath, a.serveIndex)
				r.Get(photoPath+paginationPath, a.serveIndex)
			})
		}

		// Search
		if blogConfig.Search != nil && blogConfig.Search.Enabled {
			searchPath := blogPath + blogConfig.Search.Path
			r.With(sbm, a.searchMiddlewares[blog]).Mount(searchPath, a.searchRouter)
		}

		// Stats
		if blogConfig.BlogStats != nil && blogConfig.BlogStats.Enabled {
			statsPath := blogPath + blogConfig.BlogStats.Path
			r.Group(func(r chi.Router) {
				r.Use(a.privateModeHandler...)
				r.Use(a.cache.cacheMiddleware, sbm)
				r.Get(statsPath, a.serveBlogStats)
				r.Get(statsPath+".table.html", a.serveBlogStatsTable)
			})
		}

		// Date archives
		r.Group(func(r chi.Router) {
			r.Use(a.privateModeHandler...)
			r.Use(a.cache.cacheMiddleware, sbm)

			yearRegex := `/{year:x|\d\d\d\d}`
			monthRegex := `/{month:x|\d\d}`
			dayRegex := `/{day:\d\d}`

			yearPath := blogPath + yearRegex
			r.Get(yearPath, a.serveDate)
			r.Get(yearPath+feedPath, a.serveDate)
			r.Get(yearPath+paginationPath, a.serveDate)

			monthPath := yearPath + monthRegex
			r.Get(monthPath, a.serveDate)
			r.Get(monthPath+feedPath, a.serveDate)
			r.Get(monthPath+paginationPath, a.serveDate)

			dayPath := monthPath + dayRegex
			r.Get(dayPath, a.serveDate)
			r.Get(dayPath+feedPath, a.serveDate)
			r.Get(dayPath+paginationPath, a.serveDate)
		})

		// Blog
		if !blogConfig.PostAsHome {
			r.Group(func(r chi.Router) {
				r.Use(a.privateModeHandler...)
				r.Use(sbm)
				r.With(a.checkActivityStreamsRequest, a.cache.cacheMiddleware).Get(blogConfig.Path, a.serveHome)
				r.With(a.cache.cacheMiddleware).Get(blogConfig.Path+feedPath, a.serveHome)
				r.With(a.cache.cacheMiddleware).Get(blogPath+paginationPath, a.serveHome)
			})
		}

		// Custom pages
		for _, cp := range blogConfig.CustomPages {
			scp := a.customPagesMiddlewares[cp.Path]
			if cp.Cache {
				r.With(a.privateModeHandler...).With(a.cache.cacheMiddleware, sbm, scp).Get(cp.Path, a.serveCustomPage)
			} else {
				r.With(a.privateModeHandler...).With(sbm, scp).Get(cp.Path, a.serveCustomPage)
			}
		}

		// Random post
		if rp := blogConfig.RandomPost; rp != nil && rp.Enabled {
			randomPath := rp.Path
			if randomPath == "" {
				randomPath = "/random"
			}
			r.With(a.privateModeHandler...).With(sbm).Get(blogPath+randomPath, a.redirectToRandomPost)
		}

		// Editor
		r.With(sbm).Mount(blogPath+"/editor", a.editorRouter)

		// Comments
		if commentsConfig := blogConfig.Comments; commentsConfig != nil && commentsConfig.Enabled {
			commentsPath := blogPath + "/comment"
			r.With(sbm, a.commentsMiddlewares[blog]).Mount(commentsPath, a.commentsRouter)
		}

		// Blogroll
		if brConfig := blogConfig.Blogroll; brConfig != nil && brConfig.Enabled {
			brPath := blogPath + brConfig.Path
			r.Group(func(r chi.Router) {
				r.Use(a.privateModeHandler...)
				r.Use(a.cache.cacheMiddleware, sbm)
				r.Get(brPath, a.serveBlogroll)
				r.Get(brPath+".opml", a.serveBlogrollExport)
			})
		}
	}

	// Sitemap
	r.With(a.privateModeHandler...).With(a.cache.cacheMiddleware).Get(sitemapPath, a.serveSitemap)

	// Robots.txt - doesn't need cache, because it's too simple
	if !a.privateMode {
		r.Get("/robots.txt", a.serveRobotsTXT)
	} else {
		r.Get("/robots.txt", servePrivateRobotsTXT)
	}

	// Check redirects, then serve 404
	r.With(a.cache.cacheMiddleware, a.checkRegexRedirects).NotFound(a.serve404)

	r.MethodNotAllowed(a.serveNotAllowed)

	return r, nil
}

func (a *goBlog) blogPath(blog string) string {
	blogPath := a.cfg.Blogs[blog].Path
	if blogPath == "/" {
		return ""
	}
	return blogPath
}

const blogContextKey requestContextKey = "blog"
const pathContextKey requestContextKey = "httpPath"

var cspDomains = ""

func (a *goBlog) refreshCSPDomains() {
	cspDomains = ""
	if mp := a.cfg.Micropub.MediaStorage; mp != nil && mp.MediaURL != "" {
		if u, err := url.Parse(mp.MediaURL); err == nil {
			cspDomains += " " + u.Hostname()
		}
	}
	if len(a.cfg.Server.CSPDomains) > 0 {
		cspDomains += " " + strings.Join(a.cfg.Server.CSPDomains, " ")
	}
}

func (a *goBlog) securityHeaders(next http.Handler) http.Handler {
	a.refreshCSPDomains()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Strict-Transport-Security", "max-age=31536000;")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("X-Xss-Protection", "1; mode=block")
		w.Header().Set("Content-Security-Policy", "default-src 'self'"+cspDomains)
		if a.cfg.Server.Tor && a.torAddress != "" {
			w.Header().Set("Onion-Location", fmt.Sprintf("http://%v%v", a.torAddress, r.RequestURI))
		}
		next.ServeHTTP(w, r)
	})
}

func noIndexHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Robots-Tag", "noindex")
		next.ServeHTTP(w, r)
	})
}

type dynamicHandler struct {
	router *chi.Mux
	mutex  sync.RWMutex
}

func (d *dynamicHandler) swapHandler(h *chi.Mux) {
	d.mutex.Lock()
	d.router = h
	d.mutex.Unlock()
}

func (d *dynamicHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Fix to use Path routing instead of RawPath routing in Chi
	r.URL.RawPath = ""
	// Serve request
	d.mutex.RLock()
	router := d.router
	d.mutex.RUnlock()
	router.ServeHTTP(w, r)
}
