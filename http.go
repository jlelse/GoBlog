package main

import (
	"context"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const contentTypeHTML = "text/html"

var d *dynamicHandler

func startServer() error {
	d = newDynamicHandler()
	h, err := buildHandler()
	if err != nil {
		return err
	}
	d.swapHandler(h)

	address := ":" + strconv.Itoa(appConfig.server.port)
	srv := &http.Server{
		Addr:    address,
		Handler: d,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Println("Shutting down the server")
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server with a timeout of 10 seconds.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		return err
	}
	return nil
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
