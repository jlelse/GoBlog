package main

import (
	"compress/flate"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/caddyserver/certmagic"
	"github.com/dchest/captcha"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	servertiming "github.com/mitchellh/go-server-timing"
)

const (
	contentType = "Content-Type"

	charsetUtf8Suffix = "; charset=utf-8"

	contentTypeHTML          = "text/html"
	contentTypeJSON          = "application/json"
	contentTypeWWWForm       = "application/x-www-form-urlencoded"
	contentTypeMultipartForm = "multipart/form-data"
	contentTypeAS            = "application/activity+json"
	contentTypeRSS           = "application/rss+xml"
	contentTypeATOM          = "application/atom+xml"
	contentTypeJSONFeed      = "application/feed+json"

	contentTypeHTMLUTF8 = contentTypeHTML + charsetUtf8Suffix
	contentTypeJSONUTF8 = contentTypeJSON + charsetUtf8Suffix
	contentTypeASUTF8   = contentTypeAS + charsetUtf8Suffix

	userAgent    = "User-Agent"
	appUserAgent = "GoBlog"
)

var d *dynamicHandler

func startServer() (err error) {
	// Start
	d = &dynamicHandler{}
	// Set basic middlewares
	var finalHandler http.Handler = d
	if appConfig.Server.PublicHTTPS || appConfig.Server.SecurityHeaders {
		finalHandler = securityHeaders(finalHandler)
	}
	finalHandler = servertiming.Middleware(finalHandler, nil)
	finalHandler = middleware.Heartbeat("/ping")(finalHandler)
	finalHandler = middleware.Compress(flate.DefaultCompression)(finalHandler)
	finalHandler = middleware.Recoverer(finalHandler)
	if appConfig.Server.Logging {
		finalHandler = logMiddleware(finalHandler)
	}
	// Create routers that don't change
	err = buildStaticHandlersRouters()
	if err != nil {
		return
	}
	// Load router
	err = reloadRouter()
	if err != nil {
		return
	}
	// Start Onion service
	if appConfig.Server.Tor {
		go func() {
			torErr := startOnionService(finalHandler)
			log.Println("Tor failed:", torErr.Error())
		}()
	}
	// Start HTTP(s) server
	localAddress := ":" + strconv.Itoa(appConfig.Server.Port)
	if appConfig.Server.PublicHTTPS {
		certmagic.Default.Storage = &certmagic.FileStorage{Path: "data/https"}
		certmagic.DefaultACME.Agreed = true
		certmagic.DefaultACME.Email = appConfig.Server.LetsEncryptMail
		certmagic.DefaultACME.CA = certmagic.LetsEncryptProductionCA
		hosts := []string{appConfig.Server.publicHostname}
		if appConfig.Server.shortPublicHostname != "" {
			hosts = append(hosts, appConfig.Server.shortPublicHostname)
		}
		err = certmagic.HTTPS(hosts, finalHandler)
	} else {
		err = http.ListenAndServe(localAddress, finalHandler)
	}
	return
}

func reloadRouter() error {
	h, err := buildDynamicRouter()
	if err != nil {
		return err
	}
	d.swapHandler(h)
	purgeCache()
	return nil
}

const (
	paginationPath = "/page/{page:[0-9-]+}"
	feedPath       = ".{feed:rss|json|atom}"
)

var (
	privateMode        = false
	privateModeHandler = []func(http.Handler) http.Handler{}

	captchaHandler http.Handler

	micropubRouter, indieAuthRouter, webmentionsRouter, notificationsRouter, activitypubRouter, editorRouter, commentsRouter, searchRouter *chi.Mux

	setBlogMiddlewares     = map[string]func(http.Handler) http.Handler{}
	sectionMiddlewares     = map[string]func(http.Handler) http.Handler{}
	taxonomyMiddlewares    = map[string]func(http.Handler) http.Handler{}
	photosMiddlewares      = map[string]func(http.Handler) http.Handler{}
	searchMiddlewares      = map[string]func(http.Handler) http.Handler{}
	customPagesMiddlewares = map[string]func(http.Handler) http.Handler{}
	commentsMiddlewares    = map[string]func(http.Handler) http.Handler{}
)

func buildStaticHandlersRouters() error {
	if pm := appConfig.PrivateMode; pm != nil && pm.Enabled {
		privateMode = true
		privateModeHandler = append(privateModeHandler, authMiddleware)
	}

	captchaHandler = captcha.Server(500, 250)

	micropubRouter = chi.NewRouter()
	micropubRouter.Use(checkIndieAuth)
	micropubRouter.Get("/", serveMicropubQuery)
	micropubRouter.Post("/", serveMicropubPost)
	micropubRouter.Post(micropubMediaSubPath, serveMicropubMedia)

	indieAuthRouter = chi.NewRouter()
	indieAuthRouter.Get("/", indieAuthRequest)
	indieAuthRouter.With(authMiddleware).Post("/accept", indieAuthAccept)
	indieAuthRouter.Post("/", indieAuthVerification)
	indieAuthRouter.Get("/token", indieAuthToken)
	indieAuthRouter.Post("/token", indieAuthToken)

	webmentionsRouter = chi.NewRouter()
	webmentionsRouter.Post("/", handleWebmention)
	webmentionsRouter.Group(func(r chi.Router) {
		// Authenticated routes
		r.Use(authMiddleware)
		r.Get("/", webmentionAdmin)
		r.Get(paginationPath, webmentionAdmin)
		r.Post("/delete", webmentionAdminDelete)
		r.Post("/approve", webmentionAdminApprove)
	})

	notificationsRouter = chi.NewRouter()
	notificationsRouter.Use(authMiddleware)
	notificationsRouter.Get("/", notificationsAdmin)
	notificationsRouter.Get(paginationPath, notificationsAdmin)

	if ap := appConfig.ActivityPub; ap != nil && ap.Enabled {
		activitypubRouter = chi.NewRouter()
		activitypubRouter.Post("/inbox/{blog}", apHandleInbox)
		activitypubRouter.Post("/{blog}/inbox", apHandleInbox)
	}

	editorRouter = chi.NewRouter()
	editorRouter.Use(authMiddleware)
	editorRouter.Get("/", serveEditor)
	editorRouter.Post("/", serveEditorPost)

	commentsRouter = chi.NewRouter()
	commentsRouter.Use(privateModeHandler...)
	commentsRouter.With(cacheMiddleware).Get("/{id:[0-9]+}", serveComment)
	commentsRouter.With(captchaMiddleware).Post("/", createComment)
	commentsRouter.Group(func(r chi.Router) {
		// Admin
		r.Use(authMiddleware)
		r.Get("/", commentsAdmin)
		r.Get(paginationPath, commentsAdmin)
		r.Post("/delete", commentsAdminDelete)
	})

	searchRouter = chi.NewRouter()
	searchRouter.Use(privateModeHandler...)
	searchRouter.Use(cacheMiddleware)
	searchRouter.Get("/", serveSearch)
	searchRouter.Post("/", serveSearch)
	searchResultPath := "/" + searchPlaceholder
	searchRouter.Get(searchResultPath, serveSearchResult)
	searchRouter.Get(searchResultPath+feedPath, serveSearchResult)
	searchRouter.Get(searchResultPath+paginationPath, serveSearchResult)

	for blog, blogConfig := range appConfig.Blogs {
		sbm := middleware.WithValue(blogContextKey, blog)
		setBlogMiddlewares[blog] = sbm

		blogPath := blogPath(blog)

		for _, section := range blogConfig.Sections {
			if section.Name != "" {
				secPath := blogPath + "/" + section.Name
				sectionMiddlewares[secPath] = middleware.WithValue(indexConfigKey, &indexConfig{
					path:    secPath,
					section: section,
				})
			}
		}

		for _, taxonomy := range blogConfig.Taxonomies {
			if taxonomy.Name != "" {
				taxPath := blogPath + "/" + taxonomy.Name
				taxonomyMiddlewares[taxPath] = middleware.WithValue(taxonomyContextKey, taxonomy)
			}
		}

		if blogConfig.Photos != nil && blogConfig.Photos.Enabled {
			photosMiddlewares[blog] = middleware.WithValue(indexConfigKey, &indexConfig{
				path:            blogPath + blogConfig.Photos.Path,
				parameter:       blogConfig.Photos.Parameter,
				title:           blogConfig.Photos.Title,
				description:     blogConfig.Photos.Description,
				summaryTemplate: templatePhotosSummary,
			})
		}

		if blogConfig.Search != nil && blogConfig.Search.Enabled {
			searchMiddlewares[blog] = middleware.WithValue(pathContextKey, blogPath+blogConfig.Search.Path)
		}

		for _, cp := range blogConfig.CustomPages {
			customPagesMiddlewares[cp.Path] = middleware.WithValue(customPageContextKey, cp)
		}

		if commentsConfig := blogConfig.Comments; commentsConfig != nil && commentsConfig.Enabled {
			commentsMiddlewares[blog] = middleware.WithValue(pathContextKey, blogPath+"/comment")
		}
	}

	return nil
}

var (
	taxValueMiddlewares = map[string]func(http.Handler) http.Handler{}
)

func buildDynamicRouter() (*chi.Mux, error) {
	r := chi.NewRouter()

	// Basic middleware
	r.Use(redirectShortDomain)
	r.Use(middleware.RedirectSlashes)
	r.Use(middleware.CleanPath)
	r.Use(middleware.GetHead)
	if !appConfig.Cache.Enable {
		r.Use(middleware.NoCache)
	}

	// No Index Header
	if privateMode {
		r.Use(noIndexHeader)
	}

	// Login middleware etc.
	r.Use(checkIsLogin)
	r.Use(checkIsCaptcha)
	r.Use(checkLoggedIn)

	// Logout
	r.With(authMiddleware).Get("/login", serveLogin)
	r.With(authMiddleware).Get("/logout", serveLogout)

	// Micropub
	r.Mount(micropubPath, micropubRouter)

	// IndieAuth
	r.Mount("/indieauth", indieAuthRouter)

	// ActivityPub and stuff
	if ap := appConfig.ActivityPub; ap != nil && ap.Enabled {
		r.Mount("/activitypub", activitypubRouter)
		r.With(cacheMiddleware).Get("/.well-known/webfinger", apHandleWebfinger)
		r.With(cacheMiddleware).Get("/.well-known/host-meta", handleWellKnownHostMeta)
		r.With(cacheMiddleware).Get("/.well-known/nodeinfo", serveNodeInfoDiscover)
		r.With(cacheMiddleware).Get("/nodeinfo", serveNodeInfo)
	}

	// Webmentions
	r.Mount(webmentionPath, webmentionsRouter)

	// Notifications
	r.Mount(notificationsPath, notificationsRouter)

	// Posts
	pp, err := allPostPaths(statusPublished)
	if err != nil {
		return nil, err
	}
	r.Group(func(r chi.Router) {
		r.Use(privateModeHandler...)
		r.Use(checkActivityStreamsRequest, cacheMiddleware)
		for _, path := range pp {
			r.Get(path, servePost)
		}
	})

	// Drafts
	dp, err := allPostPaths(statusDraft)
	if err != nil {
		return nil, err
	}
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware)
		for _, path := range dp {
			r.Get(path, servePost)
		}
	})

	// Post aliases
	allPostAliases, err := allPostAliases()
	if err != nil {
		return nil, err
	}
	r.Group(func(r chi.Router) {
		r.Use(privateModeHandler...)
		r.Use(cacheMiddleware)
		for _, path := range allPostAliases {
			r.Get(path, servePostAlias)
		}
	})

	// Assets
	for _, path := range allAssetPaths() {
		r.Get(path, serveAsset)
	}

	// Static files
	for _, path := range allStaticPaths() {
		r.Get(path, serveStaticFile)
	}

	// Media files
	r.With(privateModeHandler...).Get(`/m/{file:[0-9a-fA-F]+(\.[0-9a-zA-Z]+)?}`, serveMediaFile)

	// Captcha
	r.Handle("/captcha/*", captchaHandler)

	// Short paths
	r.With(privateModeHandler...).With(cacheMiddleware).Get("/s/{id:[0-9a-fA-F]+}", redirectToLongPath)

	for blog, blogConfig := range appConfig.Blogs {
		blogPath := blogPath(blog)

		sbm := setBlogMiddlewares[blog]

		// Sections
		r.Group(func(r chi.Router) {
			r.Use(privateModeHandler...)
			r.Use(cacheMiddleware, sbm)
			for _, section := range blogConfig.Sections {
				if section.Name != "" {
					secPath := blogPath + "/" + section.Name
					r.Group(func(r chi.Router) {
						r.Use(sectionMiddlewares[secPath])
						r.Get(secPath, serveIndex)
						r.Get(secPath+feedPath, serveIndex)
						r.Get(secPath+paginationPath, serveIndex)
					})
				}
			}
		})

		// Taxonomies
		for _, taxonomy := range blogConfig.Taxonomies {
			if taxonomy.Name != "" {
				taxPath := blogPath + "/" + taxonomy.Name
				taxValues, err := allTaxonomyValues(blog, taxonomy.Name)
				if err != nil {
					return nil, err
				}
				r.Group(func(r chi.Router) {
					r.Use(privateModeHandler...)
					r.Use(cacheMiddleware, sbm)
					r.With(taxonomyMiddlewares[taxPath]).Get(taxPath, serveTaxonomy)
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
							r.Get(vPath, serveIndex)
							r.Get(vPath+feedPath, serveIndex)
							r.Get(vPath+paginationPath, serveIndex)
						})
					}
				})
			}
		}

		// Photos
		if blogConfig.Photos != nil && blogConfig.Photos.Enabled {
			r.Group(func(r chi.Router) {
				r.Use(privateModeHandler...)
				r.Use(cacheMiddleware, sbm, photosMiddlewares[blog])
				photoPath := blogPath + blogConfig.Photos.Path
				r.Get(photoPath, serveIndex)
				r.Get(photoPath+feedPath, serveIndex)
				r.Get(photoPath+paginationPath, serveIndex)
			})
		}

		// Search
		if blogConfig.Search != nil && blogConfig.Search.Enabled {
			searchPath := blogPath + blogConfig.Search.Path
			r.With(sbm, searchMiddlewares[blog]).Mount(searchPath, searchRouter)
		}

		// Stats
		if blogConfig.BlogStats != nil && blogConfig.BlogStats.Enabled {
			statsPath := blogPath + blogConfig.BlogStats.Path
			r.With(privateModeHandler...).With(cacheMiddleware, sbm).Get(statsPath, serveBlogStats)
		}

		// Date archives
		r.Group(func(r chi.Router) {
			r.Use(privateModeHandler...)
			r.Use(cacheMiddleware, sbm)

			yearRegex := `/{year:x|\d\d\d\d}`
			monthRegex := `/{month:x|\d\d}`
			dayRegex := `/{day:\d\d}`

			yearPath := blogPath + yearRegex
			r.Get(yearPath, serveDate)
			r.Get(yearPath+feedPath, serveDate)
			r.Get(yearPath+paginationPath, serveDate)

			monthPath := yearPath + monthRegex
			r.Get(monthPath, serveDate)
			r.Get(monthPath+feedPath, serveDate)
			r.Get(monthPath+paginationPath, serveDate)

			dayPath := monthPath + dayRegex
			r.Get(dayPath, serveDate)
			r.Get(dayPath+feedPath, serveDate)
			r.Get(dayPath+paginationPath, serveDate)
		})

		// Blog
		if !blogConfig.PostAsHome {
			r.Group(func(r chi.Router) {
				r.Use(privateModeHandler...)
				r.Use(sbm)
				r.With(checkActivityStreamsRequest, cacheMiddleware).Get(blogConfig.Path, serveHome)
				r.With(cacheMiddleware).Get(blogConfig.Path+feedPath, serveHome)
				r.With(cacheMiddleware).Get(blogPath+paginationPath, serveHome)
			})
		}

		// Custom pages
		for _, cp := range blogConfig.CustomPages {
			scp := customPagesMiddlewares[cp.Path]
			if cp.Cache {
				r.With(privateModeHandler...).With(cacheMiddleware, sbm, scp).Get(cp.Path, serveCustomPage)
			} else {
				r.With(privateModeHandler...).With(sbm, scp).Get(cp.Path, serveCustomPage)
			}
		}

		// Random post
		if rp := blogConfig.RandomPost; rp != nil && rp.Enabled {
			randomPath := rp.Path
			if randomPath == "" {
				randomPath = "/random"
			}
			r.With(privateModeHandler...).With(sbm).Get(blogPath+randomPath, redirectToRandomPost)
		}

		// Editor
		r.With(sbm).Mount(blogPath+"/editor", editorRouter)

		// Comments
		if commentsConfig := blogConfig.Comments; commentsConfig != nil && commentsConfig.Enabled {
			commentsPath := blogPath + "/comment"
			r.With(sbm, commentsMiddlewares[blog]).Mount(commentsPath, commentsRouter)
		}
	}

	// Sitemap
	r.With(privateModeHandler...).With(cacheMiddleware).Get(sitemapPath, serveSitemap)

	// Robots.txt - doesn't need cache, because it's too simple
	if !privateMode {
		r.Get("/robots.txt", serveRobotsTXT)
	} else {
		r.Get("/robots.txt", servePrivateRobotsTXT)
	}

	// Check redirects, then serve 404
	r.With(cacheMiddleware, checkRegexRedirects).NotFound(serve404)

	r.MethodNotAllowed(serveNotAllowed)

	return r, nil
}

func blogPath(blog string) string {
	blogPath := appConfig.Blogs[blog].Path
	if blogPath == "/" {
		return ""
	}
	return blogPath
}

const blogContextKey requestContextKey = "blog"
const pathContextKey requestContextKey = "httpPath"

var cspDomains = ""

func refreshCSPDomains() {
	cspDomains = ""
	if mp := appConfig.Micropub.MediaStorage; mp != nil && mp.MediaURL != "" {
		if u, err := url.Parse(mp.MediaURL); err == nil {
			cspDomains += " " + u.Hostname()
		}
	}
	if len(appConfig.Server.CSPDomains) > 0 {
		cspDomains += " " + strings.Join(appConfig.Server.CSPDomains, " ")
	}
}

func securityHeaders(next http.Handler) http.Handler {
	refreshCSPDomains()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Strict-Transport-Security", "max-age=31536000;")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("X-Xss-Protection", "1; mode=block")
		w.Header().Set("Content-Security-Policy", "default-src 'self'"+cspDomains)
		if appConfig.Server.Tor && torAddress != "" {
			w.Header().Set("Onion-Location", fmt.Sprintf("http://%v%v", torAddress, r.URL.Path))
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
