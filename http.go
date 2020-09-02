package main

import (
	"github.com/caddyserver/certmagic"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

const contentTypeHTML = "text/html; charset=utf-8"

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

	r.Route("/api", func(apiRouter chi.Router) {
		apiRouter.Use(middleware.BasicAuth("API", map[string]string{
			appConfig.User.Nick: appConfig.User.Password,
		}))
		apiRouter.Post("/post", apiPostCreate)
		apiRouter.Delete("/post", apiPostDelete)
	})

	allPostPaths, err := allPostPaths()
	if err != nil {
		return nil, err
	}
	for _, path := range allPostPaths {
		if path != "" {
			r.With(cacheMiddleware, minifier.Middleware).Get(path, servePost)
		}
	}

	allRedirectPaths, err := allRedirectPaths()
	if err != nil {
		return nil, err
	}
	for _, path := range allRedirectPaths {
		if path != "" {
			r.With(minifier.Middleware).Get(path, serveRedirect)
		}
	}

	paginationPath := "/page/{page}"

	for _, section := range appConfig.Blog.Sections {
		if section.Name != "" {
			path := "/" + section.Name
			r.With(cacheMiddleware, minifier.Middleware).Get(path, serveSection(path, section))
			r.With(cacheMiddleware, minifier.Middleware).Get(path+paginationPath, serveSection(path, section))
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
				r.With(cacheMiddleware, minifier.Middleware).Get(path, serveTaxonomyValue(path, taxonomy, tv))
				r.With(cacheMiddleware, minifier.Middleware).Get(path+paginationPath, serveTaxonomyValue(path, taxonomy, tv))
			}
		}
	}

	rootPath := "/"
	blogPath := "/blog"
	if routePatterns := routesToStringSlice(r.Routes()); !routePatterns.has(rootPath) {
		r.With(cacheMiddleware, minifier.Middleware).Get(rootPath, serveHome(rootPath))
		r.With(cacheMiddleware, minifier.Middleware).Get(paginationPath, serveHome(rootPath))
	} else if !routePatterns.has(blogPath) {
		r.With(cacheMiddleware, minifier.Middleware).Get(blogPath, serveHome(blogPath))
		r.With(cacheMiddleware, minifier.Middleware).Get(blogPath+paginationPath, serveHome(blogPath))
	}

	r.With(minifier.Middleware).NotFound(serve404)

	return r, nil
}

func routesToStringSlice(routes []chi.Route) (ss stringSlice) {
	for _, r := range routes {
		ss = append(ss, r.Pattern)
	}
	return
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
