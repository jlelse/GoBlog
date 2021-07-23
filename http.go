package main

import (
	"compress/flate"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/dchest/captcha"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/justinas/alice"
	servertiming "github.com/mitchellh/go-server-timing"
	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/net/context"
)

const (
	contentType  = "Content-Type"
	userAgent    = "User-Agent"
	appUserAgent = "GoBlog"
)

func (a *goBlog) startServer() (err error) {
	log.Println("Start server(s)...")
	// Load router
	router, err := a.buildRouter()
	if err != nil {
		return err
	}
	a.d = fixHTTPHandler(router)
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
	if a.cfg.Server.PublicHTTPS {
		// Start HTTP server for redirects
		httpServer := &http.Server{
			Addr:         ":http",
			Handler:      http.HandlerFunc(redirectToHttps),
			ReadTimeout:  5 * time.Minute,
			WriteTimeout: 5 * time.Minute,
		}
		a.shutdown.Add(shutdownServer(httpServer, "http server"))
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
		acmeDir := acme.LetsEncryptURL
		// acmeDir := "https://acme-staging-v02.api.letsencrypt.org/directory"
		m := &autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			HostPolicy: autocert.HostWhitelist(hosts...),
			Cache:      &httpsCache{db: a.db},
			Client:     &acme.Client{DirectoryURL: acmeDir},
		}
		if err = s.Serve(m.Listener()); err != nil && err != http.ErrServerClosed {
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
		defer c()
		if err := s.Shutdown(toc); err != nil {
			log.Printf("Error on server shutdown (%v): %v", name, err)
		}
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

const (
	paginationPath = "/page/{page:[0-9-]+}"
	feedPath       = ".{feed:rss|json|atom}"
)

func (a *goBlog) buildRouter() (*chi.Mux, error) {
	r := chi.NewRouter()

	// Private mode
	privateMode := false
	var privateModeHandler []func(http.Handler) http.Handler
	if pm := a.cfg.PrivateMode; pm != nil && pm.Enabled {
		privateMode = true
		privateModeHandler = append(privateModeHandler, a.authMiddleware)
	}

	// Basic middleware
	r.Use(a.redirectShortDomain)
	r.Use(middleware.RedirectSlashes)
	r.Use(middleware.CleanPath)
	r.Use(middleware.GetHead)
	if !a.cfg.Cache.Enable {
		r.Use(middleware.NoCache)
	}

	// No Index Header
	if privateMode {
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
	r.Route(micropubPath, func(r chi.Router) {
		r.Use(a.checkIndieAuth)
		r.Get("/", a.serveMicropubQuery)
		r.Post("/", a.serveMicropubPost)
		r.Post(micropubMediaSubPath, a.serveMicropubMedia)
	})

	// IndieAuth
	r.Route("/indieauth", func(r chi.Router) {
		r.Get("/", a.indieAuthRequest)
		r.With(a.authMiddleware).Post("/accept", a.indieAuthAccept)
		r.Post("/", a.indieAuthVerification)
		r.Get("/token", a.indieAuthToken)
		r.Post("/token", a.indieAuthToken)
	})

	// ActivityPub and stuff
	if ap := a.cfg.ActivityPub; ap != nil && ap.Enabled {
		r.Route("/activitypub", func(r chi.Router) {
			r.Post("/inbox/{blog}", a.apHandleInbox)
			r.Post("/{blog}/inbox", a.apHandleInbox)
		})
		r.Group(func(r chi.Router) {
			r.Use(a.cache.cacheMiddleware)
			r.Get("/.well-known/webfinger", a.apHandleWebfinger)
			r.Get("/.well-known/host-meta", handleWellKnownHostMeta)
			r.Get("/.well-known/nodeinfo", a.serveNodeInfoDiscover)
			r.Get("/nodeinfo", a.serveNodeInfo)
		})
	}

	// Webmentions
	if wm := a.cfg.Webmention; wm != nil && !wm.DisableReceiving {
		r.Route(webmentionPath, func(r chi.Router) {
			r.Post("/", a.handleWebmention)
			r.Group(func(r chi.Router) {
				// Authenticated routes
				r.Use(a.authMiddleware)
				r.Get("/", a.webmentionAdmin)
				r.Get(paginationPath, a.webmentionAdmin)
				r.Post("/delete", a.webmentionAdminDelete)
				r.Post("/approve", a.webmentionAdminApprove)
				r.Post("/reverify", a.webmentionAdminReverify)
			})
		})
	}

	// Notifications
	r.Route(notificationsPath, func(r chi.Router) {
		r.Use(a.authMiddleware)
		r.Get("/", a.notificationsAdmin)
		r.Get(paginationPath, a.notificationsAdmin)
		r.Post("/delete", a.notificationsAdminDelete)
	})

	// Assets
	for _, path := range a.allAssetPaths() {
		r.Get(path, a.serveAsset)
	}

	// Static files
	for _, path := range allStaticPaths() {
		r.With(privateModeHandler...).Get(path, a.serveStaticFile)
	}

	// Media files
	r.With(privateModeHandler...).Get(`/m/{file:[0-9a-fA-F]+(\.[0-9a-zA-Z]+)?}`, a.serveMediaFile)

	// Captcha
	r.Handle("/captcha/*", captcha.Server(500, 250))

	// Short paths
	r.With(privateModeHandler...).With(a.cache.cacheMiddleware).Get("/s/{id:[0-9a-fA-F]+}", a.redirectToLongPath)

	for blog, blogConfig := range a.cfg.Blogs {
		sbm := middleware.WithValue(blogContextKey, blog)

		// Sections
		r.Group(func(r chi.Router) {
			r.Use(privateModeHandler...)
			r.Use(a.cache.cacheMiddleware, sbm)
			for _, section := range blogConfig.Sections {
				if section.Name != "" {
					r.Group(func(r chi.Router) {
						secPath := blogConfig.getRelativePath(section.Name)
						r.Use(middleware.WithValue(indexConfigKey, &indexConfig{
							path:    secPath,
							section: section,
						}))
						r.Get(secPath, a.serveIndex)
						r.Get(secPath+feedPath, a.serveIndex)
						r.Get(secPath+paginationPath, a.serveIndex)
					})
				}
			}
		})

		// Taxonomies
		r.Group(func(r chi.Router) {
			r.Use(privateModeHandler...)
			r.Use(a.cache.cacheMiddleware, sbm)
			for _, taxonomy := range blogConfig.Taxonomies {
				if taxonomy.Name != "" {
					r.Group(func(r chi.Router) {
						r.Use(middleware.WithValue(taxonomyContextKey, taxonomy))
						taxBasePath := blogConfig.getRelativePath(taxonomy.Name)
						r.Get(taxBasePath, a.serveTaxonomy)
						taxValPath := taxBasePath + "/{taxValue}"
						r.Get(taxValPath, a.serveTaxonomyValue)
						r.Get(taxValPath+feedPath, a.serveTaxonomyValue)
						r.Get(taxValPath+paginationPath, a.serveTaxonomyValue)
					})
				}
			}
		})

		// Photos
		if pc := blogConfig.Photos; pc != nil && pc.Enabled {
			r.Group(func(r chi.Router) {
				photoPath := blogConfig.getRelativePath(defaultIfEmpty(pc.Path, defaultPhotosPath))
				r.Use(privateModeHandler...)
				r.Use(a.cache.cacheMiddleware, sbm, middleware.WithValue(indexConfigKey, &indexConfig{
					path:            photoPath,
					parameter:       pc.Parameter,
					title:           pc.Title,
					description:     pc.Description,
					summaryTemplate: templatePhotosSummary,
				}))
				r.Get(photoPath, a.serveIndex)
				r.Get(photoPath+feedPath, a.serveIndex)
				r.Get(photoPath+paginationPath, a.serveIndex)
			})
		}

		// Search
		if bsc := blogConfig.Search; bsc != nil && bsc.Enabled {
			searchPath := blogConfig.getRelativePath(defaultIfEmpty(bsc.Path, defaultSearchPath))
			r.Route(searchPath, func(r chi.Router) {
				r.Use(sbm, middleware.WithValue(
					pathContextKey,
					searchPath,
				))
				r.Group(func(r chi.Router) {
					r.Use(privateModeHandler...)
					r.Use(a.cache.cacheMiddleware)
					r.Get("/", a.serveSearch)
					r.Post("/", a.serveSearch)
					searchResultPath := "/" + searchPlaceholder
					r.Get(searchResultPath, a.serveSearchResult)
					r.Get(searchResultPath+feedPath, a.serveSearchResult)
					r.Get(searchResultPath+paginationPath, a.serveSearchResult)
				})
				r.With(a.cache.cacheMiddleware).Get("/opensearch.xml", a.serveOpenSearch)
			})
		}

		// Stats
		if bsc := blogConfig.BlogStats; bsc != nil && bsc.Enabled {
			statsPath := blogConfig.getRelativePath(defaultIfEmpty(bsc.Path, defaultBlogStatsPath))
			r.Group(func(r chi.Router) {
				r.Use(privateModeHandler...)
				r.Use(a.cache.cacheMiddleware, sbm)
				r.Get(statsPath, a.serveBlogStats)
				r.Get(statsPath+".table.html", a.serveBlogStatsTable)
			})
		}

		// Date archives
		r.Group(func(r chi.Router) {
			r.Use(privateModeHandler...)
			r.Use(a.cache.cacheMiddleware, sbm)

			yearRegex := `/{year:x|\d\d\d\d}`
			monthRegex := `/{month:x|\d\d}`
			dayRegex := `/{day:\d\d}`

			yearPath := blogConfig.getRelativePath(yearRegex)
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
				r.Use(privateModeHandler...)
				r.Use(sbm)
				r.With(a.checkActivityStreamsRequest, a.cache.cacheMiddleware).Get(blogConfig.getRelativePath(""), a.serveHome)
				r.With(a.cache.cacheMiddleware).Get(blogConfig.getRelativePath("")+feedPath, a.serveHome)
				r.With(a.cache.cacheMiddleware).Get(blogConfig.getRelativePath(paginationPath), a.serveHome)
			})
		}

		// Custom pages
		for _, cp := range blogConfig.CustomPages {
			scp := middleware.WithValue(customPageContextKey, cp)
			if cp.Cache {
				r.With(privateModeHandler...).With(a.cache.cacheMiddleware, sbm, scp).Get(cp.Path, a.serveCustomPage)
			} else {
				r.With(privateModeHandler...).With(sbm, scp).Get(cp.Path, a.serveCustomPage)
			}
		}

		// Random post
		if rp := blogConfig.RandomPost; rp != nil && rp.Enabled {
			r.With(privateModeHandler...).With(sbm).Get(blogConfig.getRelativePath(defaultIfEmpty(rp.Path, "/random")), a.redirectToRandomPost)
		}

		// Editor
		r.Route(blogConfig.getRelativePath("/editor"), func(r chi.Router) {
			r.Use(sbm, a.authMiddleware)
			r.Get("/", a.serveEditor)
			r.Post("/", a.serveEditorPost)
			r.Get("/files", a.serveEditorFiles)
			r.Post("/files/view", a.serveEditorFilesView)
			r.Post("/files/delete", a.serveEditorFilesDelete)
			r.Get("/drafts", a.serveDrafts)
			r.Get("/drafts"+feedPath, a.serveDrafts)
			r.Get("/drafts"+paginationPath, a.serveDrafts)
			r.Get("/private", a.servePrivate)
			r.Get("/private"+feedPath, a.servePrivate)
			r.Get("/private"+paginationPath, a.servePrivate)
			r.Get("/unlisted", a.serveUnlisted)
			r.Get("/unlisted"+feedPath, a.serveUnlisted)
			r.Get("/unlisted"+paginationPath, a.serveUnlisted)
		})

		// Comments
		if commentsConfig := blogConfig.Comments; commentsConfig != nil && commentsConfig.Enabled {
			commentsPath := blogConfig.getRelativePath("/comment")
			r.Route(commentsPath, func(r chi.Router) {
				r.Use(sbm, middleware.WithValue(pathContextKey, commentsPath))
				r.Use(privateModeHandler...)
				r.With(a.cache.cacheMiddleware, noIndexHeader).Get("/{id:[0-9]+}", a.serveComment)
				r.With(a.captchaMiddleware).Post("/", a.createComment)
				r.Group(func(r chi.Router) {
					// Admin
					r.Use(a.authMiddleware)
					r.Get("/", a.commentsAdmin)
					r.Get(paginationPath, a.commentsAdmin)
					r.Post("/delete", a.commentsAdminDelete)
				})
			})
		}

		// Blogroll
		if brConfig := blogConfig.Blogroll; brConfig != nil && brConfig.Enabled {
			brPath := blogConfig.getRelativePath(defaultIfEmpty(brConfig.Path, defaultBlogrollPath))
			r.Group(func(r chi.Router) {
				r.Use(privateModeHandler...)
				r.Use(a.cache.cacheMiddleware, sbm)
				r.Get(brPath, a.serveBlogroll)
				r.Get(brPath+".opml", a.serveBlogrollExport)
			})
		}

		// Geo map
		if mc := blogConfig.Map; mc != nil && mc.Enabled {
			mapPath := blogConfig.getRelativePath(defaultIfEmpty(mc.Path, defaultGeoMapPath))
			r.Route(mapPath, func(r chi.Router) {
				r.Use(privateModeHandler...)
				r.Group(func(r chi.Router) {
					r.Use(a.cache.cacheMiddleware, sbm)
					r.Get("/", a.serveGeoMap)
					r.HandleFunc("/leaflet/*", a.serveLeaflet(mapPath+"/"))
				})
				r.Get("/tiles/{z}/{x}/{y}.png", a.proxyTiles(mapPath+"/tiles"))
			})
		}

		// Contact
		if cc := blogConfig.Contact; cc != nil && cc.Enabled {
			contactPath := blogConfig.getRelativePath(defaultIfEmpty(cc.Path, defaultContactPath))
			r.Route(contactPath, func(r chi.Router) {
				r.Use(privateModeHandler...)
				r.Use(a.cache.cacheMiddleware, sbm)
				r.Get("/", a.serveContactForm)
				r.With(a.captchaMiddleware).Post("/", a.sendContactSubmission)
			})
		}

	}

	// Sitemap
	r.With(privateModeHandler...).With(a.cache.cacheMiddleware).Get(sitemapPath, a.serveSitemap)

	// Robots.txt - doesn't need cache, because it's too simple
	if !privateMode {
		r.Get("/robots.txt", a.serveRobotsTXT)
	} else {
		r.Get("/robots.txt", servePrivateRobotsTXT)
	}

	r.NotFound(a.servePostsAliasesRedirects(privateModeHandler...))

	r.MethodNotAllowed(a.serveNotAllowed)

	return r, nil
}

func (a *goBlog) servePostsAliasesRedirects(pmh ...func(http.Handler) http.Handler) http.HandlerFunc {
	// Private mode
	alicePrivate := alice.New()
	for _, h := range pmh {
		alicePrivate = alicePrivate.Append(h)
	}
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
					alicePrivate.Append(a.checkActivityStreamsRequest, a.cache.cacheMiddleware).ThenFunc(a.servePost).ServeHTTP(w, r)
					return
				case statusDraft, statusPrivate:
					alice.New(a.authMiddleware).ThenFunc(a.servePost).ServeHTTP(w, r)
					return
				}
			case "alias":
				// Is alias, redirect
				alicePrivate.Append(a.cache.cacheMiddleware).ThenFunc(func(w http.ResponseWriter, r *http.Request) {
					http.Redirect(w, r, value, http.StatusFound)
				}).ServeHTTP(w, r)
				return
			case "deleted":
				// Is deleted, serve 410
				alicePrivate.Append(a.cache.cacheMiddleware).ThenFunc(func(w http.ResponseWriter, r *http.Request) {
					a.serve410(w, r)
				}).ServeHTTP(w, r)
				return
			}
		}
		// No post, check regex redirects or serve 404 error
		alice.New(a.cache.cacheMiddleware, a.checkRegexRedirects).ThenFunc(a.serve404).ServeHTTP(w, r)
	}
}

const blogContextKey contextKey = "blog"
const pathContextKey contextKey = "httpPath"

func (a *goBlog) refreshCSPDomains() {
	var cspBuilder strings.Builder
	if mp := a.cfg.Micropub.MediaStorage; mp != nil && mp.MediaURL != "" {
		if u, err := url.Parse(mp.MediaURL); err == nil {
			cspBuilder.WriteByte(' ')
			cspBuilder.WriteString(u.Hostname())
		}
	}
	if len(a.cfg.Server.CSPDomains) > 0 {
		cspBuilder.WriteByte(' ')
		cspBuilder.WriteString(strings.Join(a.cfg.Server.CSPDomains, " "))
	}
	a.cspDomains = cspBuilder.String()
}

const cspHeader = "Content-Security-Policy"

func (a *goBlog) securityHeaders(next http.Handler) http.Handler {
	a.refreshCSPDomains()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Strict-Transport-Security", "max-age=31536000;")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("X-Xss-Protection", "1; mode=block")
		w.Header().Set(cspHeader, "default-src 'self'"+a.cspDomains)
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

func fixHTTPHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.URL.RawPath = ""
		next.ServeHTTP(w, r)
	})
}
