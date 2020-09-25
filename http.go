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

const contentTypeHTML = "text/html; charset=utf-8"
const contentTypeJSON = "application/json; charset=utf-8"

var d *dynamicHandler

func startServer() (err error) {
	d = newDynamicHandler()
	h, err := buildHandler()
	if err != nil {
		return
	}
	d.swapHandler(h)
	localAddress := ":" + strconv.Itoa(appConfig.Server.Port)
	if appConfig.Server.PublicHttps {
		initPublicHTTPS()
		err = certmagic.HTTPS([]string{appConfig.Server.Domain}, d)
	} else if appConfig.Server.LocalHttps {
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

	// Indexes, Feeds
	for _, section := range appConfig.Blog.Sections {
		if section.Name != "" {
			path := "/" + section.Name
			r.With(cacheMiddleware, minifier.Middleware).Get(path, serveSection(path, section, NONE))
			r.With(cacheMiddleware, minifier.Middleware).Get(path+rssPath, serveSection(path, section, RSS))
			r.With(cacheMiddleware, minifier.Middleware).Get(path+jsonPath, serveSection(path, section, JSON))
			r.With(cacheMiddleware, minifier.Middleware).Get(path+atomPath, serveSection(path, section, ATOM))
			r.With(cacheMiddleware, minifier.Middleware).Get(path+paginationPath, serveSection(path, section, NONE))
		}
	}

	for _, taxonomy := range appConfig.Blog.Taxonomies {
		if taxonomy.Name != "" {
			r.With(cacheMiddleware, minifier.Middleware).Get("/"+taxonomy.Name, serveTaxonomy(taxonomy))
			values, err := allTaxonomyValues(taxonomy.Name)
			if err != nil {
				return nil, err
			}
			for _, tv := range values {
				path := "/" + taxonomy.Name + "/" + urlize(tv)
				r.With(cacheMiddleware, minifier.Middleware).Get(path, serveTaxonomyValue(path, taxonomy, tv, NONE))
				r.With(cacheMiddleware, minifier.Middleware).Get(path+rssPath, serveTaxonomyValue(path, taxonomy, tv, RSS))
				r.With(cacheMiddleware, minifier.Middleware).Get(path+jsonPath, serveTaxonomyValue(path, taxonomy, tv, JSON))
				r.With(cacheMiddleware, minifier.Middleware).Get(path+atomPath, serveTaxonomyValue(path, taxonomy, tv, ATOM))
				r.With(cacheMiddleware, minifier.Middleware).Get(path+paginationPath, serveTaxonomyValue(path, taxonomy, tv, NONE))
			}
		}
	}

	if appConfig.Blog.Photos.Enabled {
		r.With(cacheMiddleware, minifier.Middleware).Get(appConfig.Blog.Photos.Path, servePhotos(appConfig.Blog.Photos.Path))
		r.With(cacheMiddleware, minifier.Middleware).Get(appConfig.Blog.Photos.Path+paginationPath, servePhotos(appConfig.Blog.Photos.Path))
	}

	// Blog
	rootPath := "/"
	blogPath := "/blog"
	if !r.Match(chi.NewRouteContext(), http.MethodGet, rootPath) {
		r.With(cacheMiddleware, minifier.Middleware).Get(rootPath, serveHome("", NONE))
		r.With(cacheMiddleware, minifier.Middleware).Get(rootPath+rssPath, serveHome("", RSS))
		r.With(cacheMiddleware, minifier.Middleware).Get(rootPath+jsonPath, serveHome("", JSON))
		r.With(cacheMiddleware, minifier.Middleware).Get(rootPath+atomPath, serveHome("", ATOM))
		r.With(cacheMiddleware, minifier.Middleware).Get(paginationPath, serveHome("", NONE))
	} else if !r.Match(chi.NewRouteContext(), http.MethodGet, blogPath) {
		r.With(cacheMiddleware, minifier.Middleware).Get(blogPath, serveHome(blogPath, NONE))
		r.With(cacheMiddleware, minifier.Middleware).Get(blogPath+rssPath, serveHome(blogPath, RSS))
		r.With(cacheMiddleware, minifier.Middleware).Get(blogPath+jsonPath, serveHome(blogPath, JSON))
		r.With(cacheMiddleware, minifier.Middleware).Get(blogPath+atomPath, serveHome(blogPath, ATOM))
		r.With(cacheMiddleware, minifier.Middleware).Get(blogPath+paginationPath, serveHome(blogPath, NONE))
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
