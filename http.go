package main

import (
	"compress/flate"
	"fmt"
	"net/http"
	"strconv"
	"sync/atomic"

	"github.com/caddyserver/certmagic"
	"github.com/dchest/captcha"
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
	contentTypeRSS           = "application/rss+xml"
	contentTypeATOM          = "application/atom+xml"
	contentTypeJSONFeed      = "application/feed+json"

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
	} else if appConfig.Server.SecurityHeaders {
		err = http.ListenAndServe(localAddress, securityHeaders(d))
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

	paginationPath := "/page/{page:[0-9-]+}"
	feedPath := ".{feed:rss|json|atom}"

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
	r.Use(checkIsCaptcha)

	// Profiler
	if appConfig.Server.Debug {
		r.Mount("/debug", middleware.Profiler())
	}

	// Micropub
	r.Route(micropubPath, func(mpRouter chi.Router) {
		mpRouter.Use(checkIndieAuth)
		mpRouter.Get("/", serveMicropubQuery)
		mpRouter.Post("/", serveMicropubPost)
		mpRouter.Post(micropubMediaSubPath, serveMicropubMedia)
	})

	// IndieAuth
	r.Route("/indieauth", func(indieauthRouter chi.Router) {
		indieauthRouter.Get("/", indieAuthRequest)
		indieauthRouter.With(authMiddleware).Post("/accept", indieAuthAccept)
		indieauthRouter.Post("/", indieAuthVerification)
		indieauthRouter.Get("/token", indieAuthToken)
		indieauthRouter.Post("/token", indieAuthToken)
	})

	// ActivityPub and stuff
	if ap := appConfig.ActivityPub; ap != nil && ap.Enabled {
		r.Post("/activitypub/inbox/{blog}", apHandleInbox)
		r.Post("/activitypub/{blog}/inbox", apHandleInbox)
		r.With(cacheMiddleware).Get("/.well-known/webfinger", apHandleWebfinger)
		r.With(cacheMiddleware).Get("/.well-known/host-meta", handleWellKnownHostMeta)
		r.With(cacheMiddleware).Get("/.well-known/nodeinfo", serveNodeInfoDiscover)
		r.With(cacheMiddleware).Get("/nodeinfo", serveNodeInfo)
	}

	// Webmentions
	r.Route(webmentionPath, func(webmentionRouter chi.Router) {
		webmentionRouter.Post("/", handleWebmention)
		webmentionRouter.Group(func(r chi.Router) {
			// Authenticated routes
			r.Use(authMiddleware)
			r.Get("/", webmentionAdmin)
			r.Get(paginationPath, webmentionAdmin)
			r.Post("/delete", webmentionAdminDelete)
			r.Post("/approve", webmentionAdminApprove)
		})
	})

	// Posts
	pp, err := allPostPaths(statusPublished)
	if err != nil {
		return nil, err
	}
	r.Group(func(r chi.Router) {
		r.Use(manipulateAsPath, cacheMiddleware)
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
		r.Use(authMiddleware, cacheMiddleware)
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
	r.Get(`/m/{file:[0-9a-fA-F]+(\.[0-9a-zA-Z]+)?}`, serveMediaFile)

	// Captcha
	r.Handle("/captcha/*", captcha.Server(500, 250))

	// Short paths
	r.With(cacheMiddleware).Get("/s/{id:[0-9a-fA-F]+}", redirectToLongPath)

	for blog, blogConfig := range appConfig.Blogs {

		fullBlogPath := blogConfig.Path
		blogPath := fullBlogPath
		if blogPath == "/" {
			blogPath = ""
		}

		r.Group(func(r chi.Router) {

		})

		// Sections
		r.Group(func(r chi.Router) {
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
			r.Group(func(r chi.Router) {
				r.Use(cacheMiddleware)
				searchPath := blogPath + blogConfig.Search.Path
				handler := serveSearch(blog, searchPath)
				r.Get(searchPath, handler)
				r.Post(searchPath, handler)
				searchResultPath := searchPath + "/" + searchPlaceholder
				resultHandler := serveSearchResults(blog, searchResultPath)
				r.Get(searchResultPath, resultHandler)
				r.Get(searchResultPath+feedPath, resultHandler)
				r.Get(searchResultPath+paginationPath, resultHandler)
			})
		}

		// Stats
		if blogConfig.BlogStats != nil && blogConfig.BlogStats.Enabled {
			statsPath := blogPath + blogConfig.BlogStats.Path
			r.With(cacheMiddleware).Get(statsPath, serveBlogStats(blog, statsPath))
		}

		// Year / month archives
		dates, err := allPublishedDates(blog)
		if err != nil {
			return nil, err
		}
		r.Group(func(r chi.Router) {
			r.Use(cacheMiddleware)
			for _, d := range dates {
				// Year
				yearPath := blogPath + "/" + fmt.Sprintf("%0004d", d.year)
				yearHandler := serveDate(blog, yearPath, d.year, 0, 0)
				r.Get(yearPath, yearHandler)
				r.Get(yearPath+feedPath, yearHandler)
				r.Get(yearPath+paginationPath, yearHandler)
				// Specific month
				monthPath := yearPath + "/" + fmt.Sprintf("%02d", d.month)
				monthHandler := serveDate(blog, monthPath, d.year, d.month, 0)
				r.Get(monthPath, monthHandler)
				r.Get(monthPath+feedPath, monthHandler)
				r.Get(monthPath+paginationPath, monthHandler)
				// Specific day
				dayPath := monthPath + "/" + fmt.Sprintf("%02d", d.day)
				dayHandler := serveDate(blog, monthPath, d.year, d.month, d.day)
				r.Get(dayPath, dayHandler)
				r.Get(dayPath+feedPath, dayHandler)
				r.Get(dayPath+paginationPath, dayHandler)
				// Generic month
				genericMonthPath := blogPath + "/x/" + fmt.Sprintf("%02d", d.month)
				genericMonthHandler := serveDate(blog, genericMonthPath, 0, d.month, 0)
				r.Get(genericMonthPath, genericMonthHandler)
				r.Get(genericMonthPath+feedPath, genericMonthHandler)
				r.Get(genericMonthPath+paginationPath, genericMonthHandler)
				// Specific day
				genericMonthDayPath := genericMonthPath + "/" + fmt.Sprintf("%02d", d.day)
				genericMonthDayHandler := serveDate(blog, genericMonthDayPath, 0, d.month, d.day)
				r.Get(genericMonthDayPath, genericMonthDayHandler)
				r.Get(genericMonthDayPath+feedPath, genericMonthDayHandler)
				r.Get(genericMonthDayPath+paginationPath, genericMonthDayHandler)
			}
		})

		// Blog
		if !blogConfig.PostAsHome {
			handler := serveHome(blog, blogPath)
			r.With(manipulateAsPath, cacheMiddleware).Get(fullBlogPath, handler)
			r.With(cacheMiddleware).Get(fullBlogPath+feedPath, handler)
			r.With(cacheMiddleware).Get(blogPath+paginationPath, handler)
		}

		// Custom pages
		for _, cp := range blogConfig.CustomPages {
			handler := serveCustomPage(blogConfig, cp)
			if cp.Cache {
				r.With(cacheMiddleware).Get(cp.Path, handler)
			} else {
				r.Get(cp.Path, handler)
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
			mpRouter.Use(authMiddleware)
			mpRouter.Get("/", serveEditor(blog))
			mpRouter.Post("/", serveEditorPost(blog))
		})

		// Comments
		if commentsConfig := blogConfig.Comments; commentsConfig != nil && commentsConfig.Enabled {
			commentsPath := blogPath + "/comment"
			r.Route(commentsPath, func(cr chi.Router) {
				cr.With(cacheMiddleware).Get("/{id:[0-9]+}", serveComment(blog))
				cr.With(captchaMiddleware).Post("/", createComment(blog, commentsPath))
				// Admin
				cr.Group(func(r chi.Router) {
					r.Use(authMiddleware)
					r.Get("/", commentsAdmin)
					r.Post("/delete", commentsAdminDelete)
				})
			})
		}
	}

	// Sitemap
	r.With(cacheMiddleware).Get(sitemapPath, serveSitemap)

	// Robots.txt - doesn't need cache, because it's too simple
	r.Get("/robots.txt", serveRobotsTXT)

	// Check redirects, then serve 404
	r.With(cacheMiddleware, checkRegexRedirects).NotFound(serve404)

	r.MethodNotAllowed(func(rw http.ResponseWriter, r *http.Request) {
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
