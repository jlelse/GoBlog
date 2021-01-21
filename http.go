package main

import (
	"compress/flate"
	"fmt"
	"net/http"
	"strconv"
	"sync/atomic"

	"github.com/caddyserver/certmagic"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
)

const (
	contentType = "Content-Type"

	charsetUtf8Suffix = "; charset=utf-8"

	contentTypeHTML          = "text/html"
	contentTypeJSON          = "application/json"
	contentTypeWWWForm       = "application/x-www-form-urlencoded"
	contentTypeMultipartForm = "multipart/form-data"
	contentTypeAS            = "application/activity+json"

	contentTypeHTMLUTF8 = contentTypeHTML + charsetUtf8Suffix
	contentTypeJSONUTF8 = contentTypeJSON + charsetUtf8Suffix
	contentTypeASUTF8   = contentTypeAS + charsetUtf8Suffix

	userAgent    = "User-Agent"
	appUserAgent = "GoBlog"
)

var (
	d *dynamicHandler
)

func startServer() (err error) {
	// Start
	d = &dynamicHandler{}
	err = reloadRouter()
	if err != nil {
		return
	}
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
		err = certmagic.HTTPS(hosts, securityHeaders(d))
	} else {
		err = http.ListenAndServe(localAddress, d)
	}
	return
}

func reloadRouter() error {
	h, err := buildHandler()
	if err != nil {
		return err
	}
	purgeCache()
	d.swapHandler(h)
	return nil
}

func buildHandler() (http.Handler, error) {
	r := chi.NewRouter()

	if appConfig.Server.Logging {
		r.Use(logMiddleware)
	}
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(flate.DefaultCompression))
	r.Use(redirectShortDomain)
	r.Use(middleware.RedirectSlashes)
	r.Use(middleware.CleanPath)
	r.Use(middleware.GetHead)
	if !appConfig.Cache.Enable {
		r.Use(middleware.NoCache)
	}
	r.Use(checkIsLogin)

	// Profiler
	if appConfig.Server.Debug {
		r.Mount("/debug", middleware.Profiler())
	}

	// API
	r.Route("/api", func(apiRouter chi.Router) {
		apiRouter.Use(middleware.NoCache, authMiddleware)
		apiRouter.Post("/hugo", apiPostCreateHugo)
	})

	// Micropub
	r.Route(micropubPath, func(mpRouter chi.Router) {
		mpRouter.Use(checkIndieAuth, middleware.NoCache, minifier.Middleware)
		mpRouter.Get("/", serveMicropubQuery)
		mpRouter.Post("/", serveMicropubPost)
		mpRouter.Post(micropubMediaSubPath, serveMicropubMedia)
	})

	// IndieAuth
	r.Route("/indieauth", func(indieauthRouter chi.Router) {
		indieauthRouter.Use(middleware.NoCache, minifier.Middleware)
		indieauthRouter.Get("/", indieAuthRequest)
		indieauthRouter.With(authMiddleware).Post("/accept", indieAuthAccept)
		indieauthRouter.Post("/", indieAuthVerification)
		indieauthRouter.Get("/token", indieAuthToken)
		indieauthRouter.Post("/token", indieAuthToken)
	})

	// ActivityPub and stuff
	if appConfig.ActivityPub.Enabled {
		r.Post("/activitypub/inbox/{blog}", apHandleInbox)
		r.Post("/activitypub/{blog}/inbox", apHandleInbox)
		r.Get("/.well-known/webfinger", apHandleWebfinger)
		r.With(cacheMiddleware).Get("/.well-known/host-meta", handleWellKnownHostMeta)
		r.With(cacheMiddleware, minifier.Middleware).Get("/.well-known/nodeinfo", serveNodeInfoDiscover)
		r.With(cacheMiddleware, minifier.Middleware).Get("/nodeinfo", serveNodeInfo)
	}

	// Webmentions
	r.Route("/webmention", func(webmentionRouter chi.Router) {
		webmentionRouter.Use(middleware.NoCache)
		webmentionRouter.Post("/", handleWebmention)
		webmentionRouter.With(minifier.Middleware, authMiddleware).Get("/", webmentionAdmin)
		webmentionRouter.With(authMiddleware).Post("/delete", webmentionAdminDelete)
		webmentionRouter.With(authMiddleware).Post("/approve", webmentionAdminApprove)
	})

	// Posts
	pp, err := allPostPaths(statusPublished)
	if err != nil {
		return nil, err
	}
	var postMW []func(http.Handler) http.Handler
	if appConfig.ActivityPub.Enabled {
		postMW = []func(http.Handler) http.Handler{manipulateAsPath, cacheMiddleware, minifier.Middleware}
	} else {
		postMW = []func(http.Handler) http.Handler{cacheMiddleware, minifier.Middleware}
	}
	for _, path := range pp {
		if path != "" {
			r.With(postMW...).Get(path, servePost)
		}
	}

	// Drafts
	dp, err := allPostPaths(statusDraft)
	if err != nil {
		return nil, err
	}
	for _, path := range dp {
		if path != "" {
			r.With(middleware.NoCache, minifier.Middleware, authMiddleware).Get(path, servePost)
		}
	}

	// Post aliases
	allPostAliases, err := allPostAliases()
	if err != nil {
		return nil, err
	}
	for _, path := range allPostAliases {
		if path != "" {
			r.With(cacheMiddleware).Get(path, servePostAlias)
		}
	}

	// Assets
	for _, path := range allAssetPaths() {
		r.Get(path, serveAsset)
	}

	// Static files
	for _, path := range allStaticPaths() {
		r.Get(path, serveStaticFile)
	}

	// Media files
	r.Get(`/m/{file:[0-9a-fA-F]+(\.[0-9a-zA-Z]+)?}`, serveMediaFile)

	// Short paths
	r.With(cacheMiddleware).Get("/s/{id:[0-9a-fA-F]+}", redirectToLongPath)

	paginationPath := "/page/{page:[0-9-]+}"
	feedPath := ".{feed:rss|json|atom}"

	for blog, blogConfig := range appConfig.Blogs {

		fullBlogPath := blogConfig.Path
		blogPath := fullBlogPath
		if blogPath == "/" {
			blogPath = ""
		}

		// Indexes, Feeds
		for _, section := range blogConfig.Sections {
			if section.Name != "" {
				path := blogPath + "/" + section.Name
				handler := serveSection(blog, path, section)
				r.With(cacheMiddleware, minifier.Middleware).Get(path, handler)
				r.With(cacheMiddleware, minifier.Middleware).Get(path+feedPath, handler)
				r.With(cacheMiddleware, minifier.Middleware).Get(path+paginationPath, handler)
			}
		}

		for _, taxonomy := range blogConfig.Taxonomies {
			if taxonomy.Name != "" {
				path := blogPath + "/" + taxonomy.Name
				r.With(cacheMiddleware, minifier.Middleware).Get(path, serveTaxonomy(blog, taxonomy))
				values, err := allTaxonomyValues(blog, taxonomy.Name)
				if err != nil {
					return nil, err
				}
				for _, tv := range values {
					vPath := path + "/" + urlize(tv)
					handler := serveTaxonomyValue(blog, vPath, taxonomy, tv)
					r.With(cacheMiddleware, minifier.Middleware).Get(vPath, handler)
					r.With(cacheMiddleware, minifier.Middleware).Get(vPath+feedPath, handler)
					r.With(cacheMiddleware, minifier.Middleware).Get(vPath+paginationPath, handler)
				}
			}
		}

		// Photos
		if blogConfig.Photos != nil && blogConfig.Photos.Enabled {
			photoPath := blogPath + blogConfig.Photos.Path
			handler := servePhotos(blog, photoPath)
			r.With(cacheMiddleware, minifier.Middleware).Get(photoPath, handler)
			r.With(cacheMiddleware, minifier.Middleware).Get(photoPath+paginationPath, handler)
		}

		// Search
		if blogConfig.Search != nil && blogConfig.Search.Enabled {
			searchPath := blogPath + blogConfig.Search.Path
			handler := serveSearch(blog, searchPath)
			r.With(cacheMiddleware, minifier.Middleware).Get(searchPath, handler)
			r.With(cacheMiddleware, minifier.Middleware).Post(searchPath, handler)
			searchResultPath := searchPath + "/" + searchPlaceholder
			resultHandler := serveSearchResults(blog, searchResultPath)
			r.With(cacheMiddleware, minifier.Middleware).Get(searchResultPath, resultHandler)
			r.With(cacheMiddleware, minifier.Middleware).Get(searchResultPath+feedPath, resultHandler)
			r.With(cacheMiddleware, minifier.Middleware).Get(searchResultPath+paginationPath, resultHandler)
		}

		// Stats
		if blogConfig.BlogStats != nil && blogConfig.BlogStats.Enabled {
			statsPath := blogPath + blogConfig.BlogStats.Path
			r.With(cacheMiddleware, minifier.Middleware).Get(statsPath, serveBlogStats(blog, statsPath))
		}

		// Year / month archives
		dates, err := allPublishedDates(blog)
		if err != nil {
			return nil, err
		}
		for _, d := range dates {
			// Year
			yearPath := blogPath + "/" + fmt.Sprintf("%0004d", d.year)
			yearHandler := serveDate(blog, yearPath, d.year, 0, 0)
			r.With(cacheMiddleware, minifier.Middleware).Get(yearPath, yearHandler)
			r.With(cacheMiddleware, minifier.Middleware).Get(yearPath+feedPath, yearHandler)
			r.With(cacheMiddleware, minifier.Middleware).Get(yearPath+paginationPath, yearHandler)
			// Specific month
			monthPath := yearPath + "/" + fmt.Sprintf("%02d", d.month)
			monthHandler := serveDate(blog, monthPath, d.year, d.month, 0)
			r.With(cacheMiddleware, minifier.Middleware).Get(monthPath, monthHandler)
			r.With(cacheMiddleware, minifier.Middleware).Get(monthPath+feedPath, monthHandler)
			r.With(cacheMiddleware, minifier.Middleware).Get(monthPath+paginationPath, monthHandler)
			// Specific day
			dayPath := monthPath + "/" + fmt.Sprintf("%02d", d.day)
			dayHandler := serveDate(blog, monthPath, d.year, d.month, d.day)
			r.With(cacheMiddleware, minifier.Middleware).Get(dayPath, dayHandler)
			r.With(cacheMiddleware, minifier.Middleware).Get(dayPath+feedPath, dayHandler)
			r.With(cacheMiddleware, minifier.Middleware).Get(dayPath+paginationPath, dayHandler)
			// Generic month
			genericMonthPath := blogPath + "/x/" + fmt.Sprintf("%02d", d.month)
			genericMonthHandler := serveDate(blog, genericMonthPath, 0, d.month, 0)
			r.With(cacheMiddleware, minifier.Middleware).Get(genericMonthPath, genericMonthHandler)
			r.With(cacheMiddleware, minifier.Middleware).Get(genericMonthPath+feedPath, genericMonthHandler)
			r.With(cacheMiddleware, minifier.Middleware).Get(genericMonthPath+paginationPath, genericMonthHandler)
			// Specific day
			genericMonthDayPath := genericMonthPath + "/" + fmt.Sprintf("%02d", d.day)
			genericMonthDayHandler := serveDate(blog, genericMonthDayPath, 0, d.month, d.day)
			r.With(cacheMiddleware, minifier.Middleware).Get(genericMonthDayPath, genericMonthDayHandler)
			r.With(cacheMiddleware, minifier.Middleware).Get(genericMonthDayPath+feedPath, genericMonthDayHandler)
			r.With(cacheMiddleware, minifier.Middleware).Get(genericMonthDayPath+paginationPath, genericMonthDayHandler)
		}

		// Blog
		if !blogConfig.PostAsHome {
			var mw []func(http.Handler) http.Handler
			if appConfig.ActivityPub.Enabled {
				mw = []func(http.Handler) http.Handler{manipulateAsPath, cacheMiddleware, minifier.Middleware}
			} else {
				mw = []func(http.Handler) http.Handler{cacheMiddleware, minifier.Middleware}
			}
			handler := serveHome(blog, blogPath)
			r.With(mw...).Get(fullBlogPath, handler)
			r.With(cacheMiddleware, minifier.Middleware).Get(fullBlogPath+feedPath, handler)
			r.With(cacheMiddleware, minifier.Middleware).Get(blogPath+paginationPath, handler)
		}

		// Custom pages
		for _, cp := range blogConfig.CustomPages {
			handler := serveCustomPage(blogConfig, cp)
			if cp.Cache {
				r.With(cacheMiddleware, minifier.Middleware).Get(cp.Path, handler)
			} else {
				r.With(minifier.Middleware).Get(cp.Path, handler)
			}
		}

		// Random post
		if rp := blogConfig.RandomPost; rp != nil && rp.Enabled {
			randomPath := rp.Path
			if randomPath == "" {
				randomPath = "/random"
			}
			r.Get(blogPath+randomPath, redirectToRandomPost(blog))
		}

		// Editor
		r.Route(blogPath+"/editor", func(mpRouter chi.Router) {
			mpRouter.Use(middleware.NoCache, minifier.Middleware, authMiddleware)
			mpRouter.Get("/", serveEditor(blog))
			mpRouter.Post("/", serveEditorPost(blog))
		})
	}

	// Sitemap
	r.With(cacheMiddleware, minifier.Middleware).Get(sitemapPath, serveSitemap)

	// Robots.txt - doesn't need cache, because it's too simple
	r.Get("/robots.txt", serveRobotsTXT)

	// Check redirects, then serve 404
	r.With(cacheMiddleware, checkRegexRedirects, minifier.Middleware).NotFound(serve404)

	r.With(minifier.Middleware).MethodNotAllowed(func(rw http.ResponseWriter, r *http.Request) {
		serveError(rw, r, "", http.StatusMethodNotAllowed)
	})

	return r, nil
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Strict-Transport-Security", "max-age=31536000;")
		w.Header().Add("Referrer-Policy", "no-referrer")
		w.Header().Add("X-Content-Type-Options", "nosniff")
		w.Header().Add("X-Frame-Options", "SAMEORIGIN")
		w.Header().Add("X-Xss-Protection", "1; mode=block")
		// TODO: Add CSP
		next.ServeHTTP(w, r)
	})
}

type dynamicHandler struct {
	realHandler atomic.Value
}

func (d *dynamicHandler) swapHandler(h http.Handler) {
	d.realHandler.Store(h)
}

func (d *dynamicHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Fix to use Path routing instead of RawPath routing in Chi
	r.URL.RawPath = ""
	// Serve request
	d.realHandler.Load().(http.Handler).ServeHTTP(w, r)
}
