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

const contentTypeHTML = "text/html"

var d *dynamicHandler

func startServer() (err error) {
	d = newDynamicHandler()
	h, err := buildHandler()
	if err != nil {
		return
	}
	d.swapHandler(h)
	localAddress := ":" + strconv.Itoa(appConfig.server.port)
	if appConfig.server.publicHttps {
		initPublicHTTPS()
		err = certmagic.HTTPS([]string{appConfig.server.domain}, d)
	} else if appConfig.server.localHttps {
		err = http.ListenAndServeTLS(localAddress, "https/server.crt", "https/server.key", d)
	} else {
		err = http.ListenAndServe(localAddress, d)
	}
	return
}

func initPublicHTTPS() {
	certmagic.Default.Storage = &certmagic.FileStorage{Path: "certmagic"}
	certmagic.DefaultACME.Agreed = true
	certmagic.DefaultACME.Email = appConfig.server.letsEncryptMail
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

	if appConfig.server.logging {
		r.Use(middleware.RealIP)
		r.Use(middleware.Logger)
	}
	r.Use(middleware.Recoverer)
	r.Use(middleware.StripSlashes)
	r.Use(middleware.GetHead)

	r.Route("/api", func(apiRouter chi.Router) {
		apiRouter.Use(middleware.BasicAuth("API", map[string]string{
			appConfig.user.nick: appConfig.user.password,
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
