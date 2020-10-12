package main

import (
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/caddyserver/certmagic"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
)

const contentType = "Content-Type"
const contentTypeHTMLUTF8 = "text/html; charset=utf-8"
const contentTypeJSONUTF8 = "application/json; charset=utf-8"
const contentTypeJSON = "application/json"
const contentTypeWWWForm = "application/x-www-form-urlencoded"
const contentTypeMultipartForm = "multipart/form-data"

var d *dynamicHandler

func startServer() (err error) {
	d = newDynamicHandler()
	h, err := buildHandler()
	if err != nil {
		return
	}
	d.swapHandler(h)
	localAddress := ":" + strconv.Itoa(appConfig.Server.Port)
	if appConfig.Server.PublicHTTPS {
		initPublicHTTPS()
		err = certmagic.HTTPS([]string{appConfig.Server.Domain}, d)
	} else if appConfig.Server.LocalHTTPS {
		err = http.ListenAndServeTLS(localAddress, "https/server.crt", "https/server.key", d)
	} else {
		err = http.ListenAndServe(localAddress, d)
	}
	return
}

func initPublicHTTPS() {
	certmagic.Default.Storage = &certmagic.FileStorage{Path: "certmagic"}
	certmagic.DefaultACME.Agreed = true
	certmagic.DefaultACME.Email = appConfig.Server.LetsEncryptMail
	certmagic.DefaultACME.CA = certmagic.LetsEncryptProductionCA
}

func reloadRouter() error {
	h, err := buildHandler()
	if err != nil {
		return err
	}
	d.swapHandler(h)
	return nil
}

func buildHandler() (http.Handler, error) {
	r := chi.NewRouter()

	if appConfig.Server.Logging {
		r.Use(middleware.RealIP, middleware.Logger)
	}
	r.Use(middleware.Recoverer, middleware.StripSlashes, middleware.GetHead)

	// Profiler
	if appConfig.Server.Debug {
		r.Mount("/debug", middleware.Profiler())
	}

	// API
	r.Route("/api", func(apiRouter chi.Router) {
		apiRouter.Use(middleware.BasicAuth("API", map[string]string{
			appConfig.User.Nick: appConfig.User.Password,
		}))
		apiRouter.Post("/post", apiPostCreate)
		apiRouter.Get("/post", apiPostRead)
		apiRouter.Delete("/post", apiPostDelete)
		apiRouter.Post("/hugo", apiPostCreateHugo)
	})

	// Micropub
	if appConfig.Micropub.Enabled {
		r.Get(appConfig.Micropub.Path, serveMicropubQuery)
		r.With(checkIndieAuth).Post(appConfig.Micropub.Path, serveMicropubPost)
		r.With(checkIndieAuth).Post(appConfig.Micropub.Path+micropubMediaSubPath, serveMicropubMedia)
	}

	// Posts
	allPostPaths, err := allPostPaths()
	if err != nil {
		return nil, err
	}
	for _, path := range allPostPaths {
		if path != "" {
			r.With(manipulateAsPath, cacheMiddleware, minifier.Middleware).Get(path, servePost)
		}
	}

	// Redirects
	allRedirectPaths, err := allRedirectPaths()
	if err != nil {
		return nil, err
	}
	for _, path := range allRedirectPaths {
		if path != "" {
			r.With(minifier.Middleware).Get(path, serveRedirect)
		}
	}

	// Assets
	for _, path := range allAssetPaths() {
		if path != "" {
			r.Get(path, serveAsset(path))
		}
	}

	paginationPath := "/page/{page}"
	rssPath := ".rss"
	jsonPath := ".json"
	atomPath := ".atom"

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
				r.With(cacheMiddleware, minifier.Middleware).Get(path, serveSection(blog, path, section, noFeed))
				r.With(cacheMiddleware, minifier.Middleware).Get(path+rssPath, serveSection(blog, path, section, rssFeed))
				r.With(cacheMiddleware, minifier.Middleware).Get(path+jsonPath, serveSection(blog, path, section, jsonFeed))
				r.With(cacheMiddleware, minifier.Middleware).Get(path+atomPath, serveSection(blog, path, section, atomFeed))
				r.With(cacheMiddleware, minifier.Middleware).Get(path+paginationPath, serveSection(blog, path, section, noFeed))
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
					r.With(cacheMiddleware, minifier.Middleware).Get(vPath, serveTaxonomyValue(blog, vPath, taxonomy, tv, noFeed))
					r.With(cacheMiddleware, minifier.Middleware).Get(vPath+rssPath, serveTaxonomyValue(blog, vPath, taxonomy, tv, rssFeed))
					r.With(cacheMiddleware, minifier.Middleware).Get(vPath+jsonPath, serveTaxonomyValue(blog, vPath, taxonomy, tv, jsonFeed))
					r.With(cacheMiddleware, minifier.Middleware).Get(vPath+atomPath, serveTaxonomyValue(blog, vPath, taxonomy, tv, atomFeed))
					r.With(cacheMiddleware, minifier.Middleware).Get(vPath+paginationPath, serveTaxonomyValue(blog, vPath, taxonomy, tv, noFeed))
				}
			}
		}

		// Photos
		if blogConfig.Photos.Enabled {
			r.With(cacheMiddleware, minifier.Middleware).Get(blogPath+blogConfig.Photos.Path, servePhotos(blog))
			r.With(cacheMiddleware, minifier.Middleware).Get(blogPath+blogConfig.Photos.Path+paginationPath, servePhotos(blog))
		}

		// Blog
		r.With(cacheMiddleware, minifier.Middleware).Get(fullBlogPath, serveHome(blog, blogPath, noFeed))
		r.With(cacheMiddleware, minifier.Middleware).Get(fullBlogPath+rssPath, serveHome(blog, blogPath, rssFeed))
		r.With(cacheMiddleware, minifier.Middleware).Get(fullBlogPath+jsonPath, serveHome(blog, blogPath, jsonFeed))
		r.With(cacheMiddleware, minifier.Middleware).Get(fullBlogPath+atomPath, serveHome(blog, blogPath, atomFeed))
		r.With(cacheMiddleware, minifier.Middleware).Get(blogPath+paginationPath, serveHome(blog, blogPath, noFeed))
	}

	// Sitemap
	r.With(cacheMiddleware).Get("/sitemap.xml", serveSitemap())

	r.With(minifier.Middleware).NotFound(serve404)

	return r, nil
}

type dynamicHandler struct {
	realHandler http.Handler
	changeMutex *sync.Mutex
}

func newDynamicHandler() *dynamicHandler {
	return &dynamicHandler{
		changeMutex: &sync.Mutex{},
	}
}

func (d *dynamicHandler) swapHandler(h http.Handler) {
	d.changeMutex.Lock()
	d.realHandler = h
	d.changeMutex.Unlock()
}

func (d *dynamicHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	d.realHandler.ServeHTTP(w, r)
}

func slashTrimmedPath(r *http.Request) string {
	return trimSlash(r.URL.Path)
}

func trimSlash(s string) string {
	if len(s) > 1 {
		s = strings.TrimSuffix(s, "/")
	}
	return s
}
