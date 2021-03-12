package main

import (
	"compress/flate"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
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

var (
	d *dynamicHandler
)

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
	// Load router
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
		err = certmagic.HTTPS(hosts, finalHandler)
	} else if appConfig.Server.SecurityHeaders {
		err = http.ListenAndServe(localAddress, finalHandler)
	} else {
		err = http.ListenAndServe(localAddress, finalHandler)
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
	// Do manual GC
	go func() {
		time.Sleep(10 * time.Second)
		runtime.GC()
	}()
	return nil
}

const paginationPath = "/page/{page:[0-9-]+}"
const feedPath = ".{feed:rss|json|atom}"

func buildHandler() (http.Handler, error) {
	startTime := time.Now()

	r := chi.NewRouter()

	// Private mode
	privateMode := false
	privateModeHandler := []func(http.Handler) http.Handler{}
	if pm := appConfig.PrivateMode; pm != nil && pm.Enabled {
		privateMode = true
		privateModeHandler = append(privateModeHandler, authMiddleware)
	}

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
	r.Route(micropubPath, func(r chi.Router) {
		r.Use(checkIndieAuth)
		r.Get("/", serveMicropubQuery)
		r.Post("/", serveMicropubPost)
		r.Post(micropubMediaSubPath, serveMicropubMedia)
	})

	// IndieAuth
	r.Route("/indieauth", func(r chi.Router) {
		r.Get("/", indieAuthRequest)
		r.With(authMiddleware).Post("/accept", indieAuthAccept)
		r.Post("/", indieAuthVerification)
		r.Get("/token", indieAuthToken)
		r.Post("/token", indieAuthToken)
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
	r.Route(webmentionPath, func(r chi.Router) {
		r.Post("/", handleWebmention)
		r.Group(func(r chi.Router) {
			// Authenticated routes
			r.Use(authMiddleware)
			r.Get("/", webmentionAdmin)
			r.Get(paginationPath, webmentionAdmin)
			r.Post("/delete", webmentionAdminDelete)
			r.Post("/approve", webmentionAdminApprove)
		})
	})

	// Notifications
	notificationsPath := "/notifications"
	r.Route(notificationsPath, func(r chi.Router) {
		r.Use(authMiddleware)
		handler := notificationsAdmin(notificationsPath)
		r.Get("/", handler)
		r.Get(paginationPath, handler)
	})

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
	r.Handle("/captcha/*", captcha.Server(500, 250))

	// Short paths
	r.With(privateModeHandler...).With(cacheMiddleware).Get("/s/{id:[0-9a-fA-F]+}", redirectToLongPath)

	for blog, blogConfig := range appConfig.Blogs {

		fullBlogPath := blogConfig.Path
		blogPath := fullBlogPath
		if blogPath == "/" {
			blogPath = ""
		}

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
			r.Group(func(r chi.Router) {
				r.Use(privateModeHandler...)
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
				r.Get(fullBlogPath, handler)
				r.Get(fullBlogPath+feedPath, handler)
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
		r.Route(blogPath+"/editor", func(r chi.Router) {
			r.Use(authMiddleware)
			r.Get("/", serveEditor(blog))
			r.Post("/", serveEditorPost(blog))
		})

		// Comments
		if commentsConfig := blogConfig.Comments; commentsConfig != nil && commentsConfig.Enabled {
			commentsPath := blogPath + "/comment"
			r.Route(commentsPath, func(cr chi.Router) {
				cr.Use(privateModeHandler...)
				cr.With(cacheMiddleware).Get("/{id:[0-9]+}", serveComment(blog))
				cr.With(captchaMiddleware).Post("/", createComment(blog, commentsPath))
				// Admin
				cr.Group(func(r chi.Router) {
					r.Use(authMiddleware)
					handler := commentsAdmin(blog, commentsPath)
					r.Get("/", handler)
					r.Get(paginationPath, handler)
					r.Post("/delete", commentsAdminDelete)
				})
			})
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

func securityHeaders(next http.Handler) http.Handler {
	extraCSPDomains := ""
	if mp := appConfig.Micropub.MediaStorage; mp != nil && mp.MediaURL != "" {
		if u, err := url.Parse(mp.MediaURL); err == nil {
			extraCSPDomains += " " + u.Hostname()
		}
	}
	if len(appConfig.Server.CSPDomains) > 0 {
		extraCSPDomains += " " + strings.Join(appConfig.Server.CSPDomains, " ")
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Strict-Transport-Security", "max-age=31536000;")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("X-Xss-Protection", "1; mode=block")
		w.Header().Set("Content-Security-Policy", "default-src 'self'"+extraCSPDomains)
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
