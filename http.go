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
	"time"

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

	micropubRouter      *chi.Mux
	indieAuthRouter     *chi.Mux
	webmentionsRouter   *chi.Mux
	notificationsRouter *chi.Mux
	activitypubRouter   *chi.Mux

	editorRouters  = map[string]*chi.Mux{}
	commentRouters = map[string]*chi.Mux{}
	searchRouters  = map[string]*chi.Mux{}
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
	notificationsHandler := notificationsAdmin(notificationsPath)
	notificationsRouter.Get("/", notificationsHandler)
	notificationsRouter.Get(paginationPath, notificationsHandler)

	if ap := appConfig.ActivityPub; ap != nil && ap.Enabled {
		activitypubRouter = chi.NewRouter()
		activitypubRouter.Post("/inbox/{blog}", apHandleInbox)
		activitypubRouter.Post("/{blog}/inbox", apHandleInbox)
	}

	for blog, blogConfig := range appConfig.Blogs {
		blogPath := blogPath(blogConfig)

		editorRouter := chi.NewRouter()
		editorRouter.Use(authMiddleware)
		editorRouter.Get("/", serveEditor(blog))
		editorRouter.Post("/", serveEditorPost(blog))
		editorRouters[blog] = editorRouter

		if commentsConfig := blogConfig.Comments; commentsConfig != nil && commentsConfig.Enabled {
			commentsPath := blogPath + "/comment"
			commentRouter := chi.NewRouter()
			commentRouter.Use(privateModeHandler...)
			commentRouter.With(cacheMiddleware).Get("/{id:[0-9]+}", serveComment(blog))
			commentRouter.With(captchaMiddleware).Post("/", createComment(blog, commentsPath))
			// Admin
			commentRouter.Group(func(r chi.Router) {
				r.Use(authMiddleware)
				handler := commentsAdmin(blog, commentsPath)
				r.Get("/", handler)
				r.Get(paginationPath, handler)
				r.Post("/delete", commentsAdminDelete)
			})
			commentRouters[blog] = commentRouter
		}

		if blogConfig.Search != nil && blogConfig.Search.Enabled {
			searchPath := blogPath + blogConfig.Search.Path
			searchRouter := chi.NewRouter()
			searchRouter.Use(privateModeHandler...)
			searchRouter.Use(cacheMiddleware)
			handler := serveSearch(blog, searchPath)
			searchRouter.Get("/", handler)
			searchRouter.Post("/", handler)
			searchResultPath := "/" + searchPlaceholder
			resultHandler := serveSearchResults(blog, searchPath+searchResultPath)
			searchRouter.Get(searchResultPath, resultHandler)
			searchRouter.Get(searchResultPath+feedPath, resultHandler)
			searchRouter.Get(searchResultPath+paginationPath, resultHandler)
			searchRouters[blog] = searchRouter
		}
	}

	return nil
}

func buildDynamicRouter() (*chi.Mux, error) {
	startTime := time.Now()

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
	r.Use(checkActivityStreamsRequest)

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
		r.Use(cacheMiddleware)
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
		blogPath := blogPath(blogConfig)

		// Sections
		r.Group(func(r chi.Router) {
			r.Use(privateModeHandler...)
			r.Use(cacheMiddleware)
			for _, section := range blogConfig.Sections {
				if section.Name != "" {
					secPath := blogPath + "/" + section.Name
					handler := serveSection(blog, secPath, section)
					r.Get(secPath, handler)
					r.Get(secPath+feedPath, handler)
					r.Get(secPath+paginationPath, handler)
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
					r.Use(cacheMiddleware)
					r.Get(taxPath, serveTaxonomy(blog, taxonomy))
					for _, tv := range taxValues {
						vPath := taxPath + "/" + urlize(tv)
						handler := serveTaxonomyValue(blog, vPath, taxonomy, tv)
						r.Get(vPath, handler)
						r.Get(vPath+feedPath, handler)
						r.Get(vPath+paginationPath, handler)
					}
				})
			}
		}

		// Photos
		if blogConfig.Photos != nil && blogConfig.Photos.Enabled {
			r.Group(func(r chi.Router) {
				r.Use(privateModeHandler...)
				r.Use(cacheMiddleware)
				photoPath := blogPath + blogConfig.Photos.Path
				handler := servePhotos(blog, photoPath)
				r.Get(photoPath, handler)
				r.Get(photoPath+feedPath, handler)
				r.Get(photoPath+paginationPath, handler)
			})
		}

		// Search
		if blogConfig.Search != nil && blogConfig.Search.Enabled {
			r.Mount(blogPath+blogConfig.Search.Path, searchRouters[blog])
		}

		// Stats
		if blogConfig.BlogStats != nil && blogConfig.BlogStats.Enabled {
			statsPath := blogPath + blogConfig.BlogStats.Path
			r.With(privateModeHandler...).With(cacheMiddleware).Get(statsPath, serveBlogStats(blog, statsPath))
		}

		// Year / month archives
		dates, err := allPublishedDates(blog)
		if err != nil {
			return nil, err
		}
		r.Group(func(r chi.Router) {
			r.Use(privateModeHandler...)
			r.Use(cacheMiddleware)
			already := map[string]bool{}
			for _, d := range dates {
				// Year
				yearPath := blogPath + "/" + fmt.Sprintf("%0004d", d.year)
				if !already[yearPath] {
					yearHandler := serveDate(blog, yearPath, d.year, 0, 0)
					r.Get(yearPath, yearHandler)
					r.Get(yearPath+feedPath, yearHandler)
					r.Get(yearPath+paginationPath, yearHandler)
					already[yearPath] = true
				}
				// Specific month
				monthPath := yearPath + "/" + fmt.Sprintf("%02d", d.month)
				if !already[monthPath] {
					monthHandler := serveDate(blog, monthPath, d.year, d.month, 0)
					r.Get(monthPath, monthHandler)
					r.Get(monthPath+feedPath, monthHandler)
					r.Get(monthPath+paginationPath, monthHandler)
					already[monthPath] = true
				}
				// Specific day
				dayPath := monthPath + "/" + fmt.Sprintf("%02d", d.day)
				if !already[dayPath] {
					dayHandler := serveDate(blog, monthPath, d.year, d.month, d.day)
					r.Get(dayPath, dayHandler)
					r.Get(dayPath+feedPath, dayHandler)
					r.Get(dayPath+paginationPath, dayHandler)
					already[dayPath] = true
				}
				// Generic month
				genericMonthPath := blogPath + "/x/" + fmt.Sprintf("%02d", d.month)
				if !already[genericMonthPath] {
					genericMonthHandler := serveDate(blog, genericMonthPath, 0, d.month, 0)
					r.Get(genericMonthPath, genericMonthHandler)
					r.Get(genericMonthPath+feedPath, genericMonthHandler)
					r.Get(genericMonthPath+paginationPath, genericMonthHandler)
					already[genericMonthPath] = true
				}
				// Specific day
				genericMonthDayPath := genericMonthPath + "/" + fmt.Sprintf("%02d", d.day)
				if !already[genericMonthDayPath] {
					genericMonthDayHandler := serveDate(blog, genericMonthDayPath, 0, d.month, d.day)
					r.Get(genericMonthDayPath, genericMonthDayHandler)
					r.Get(genericMonthDayPath+feedPath, genericMonthDayHandler)
					r.Get(genericMonthDayPath+paginationPath, genericMonthDayHandler)
					already[genericMonthDayPath] = true
				}
			}
		})

		// Blog
		if !blogConfig.PostAsHome {
			r.Group(func(r chi.Router) {
				r.Use(privateModeHandler...)
				r.Use(cacheMiddleware)
				handler := serveHome(blog, blogPath)
				r.Get(blogConfig.Path, handler)
				r.Get(blogConfig.Path+feedPath, handler)
				r.Get(blogPath+paginationPath, handler)
			})
		}

		// Custom pages
		for _, cp := range blogConfig.CustomPages {
			handler := serveCustomPage(blogConfig, cp)
			if cp.Cache {
				r.With(privateModeHandler...).With(cacheMiddleware).Get(cp.Path, handler)
			} else {
				r.With(privateModeHandler...).Get(cp.Path, handler)
			}
		}

		// Random post
		if rp := blogConfig.RandomPost; rp != nil && rp.Enabled {
			randomPath := rp.Path
			if randomPath == "" {
				randomPath = "/random"
			}
			r.With(privateModeHandler...).Get(blogPath+randomPath, redirectToRandomPost(blog))
		}

		// Editor
		r.Mount(blogPath+"/editor", editorRouters[blog])

		// Comments
		if commentsConfig := blogConfig.Comments; commentsConfig != nil && commentsConfig.Enabled {
			r.Mount(blogPath+"/comment", commentRouters[blog])
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

	r.MethodNotAllowed(func(rw http.ResponseWriter, r *http.Request) {
		serveError(rw, r, "", http.StatusMethodNotAllowed)
	})

	log.Println("Building handler took", time.Since(startTime))

	return r, nil
}

func blogPath(cb *configBlog) string {
	blogPath := cb.Path
	if blogPath == "/" {
		return ""
	}
	return blogPath
}

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
