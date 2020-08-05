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

	routePatterns := routesToStringSlice(r.Routes())
	if !routePatterns.has("/") {
		r.With(cacheMiddleware, minifier.Middleware).Get("/", serveIndex)
	} else if !routePatterns.has("/blog") {
		r.With(cacheMiddleware, minifier.Middleware).Get("/blog", serveIndex)
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
