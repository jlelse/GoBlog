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
	"sync"
	"syscall"
	"time"
)

func startServer() error {
	d := newDynamicHandler()
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

func buildHandler() (http.Handler, error) {
	r := chi.NewRouter()

	if appConfig.server.logging {
		r.Use(middleware.RealIP)
		r.Use(middleware.Logger)
	}
	r.Use(middleware.Recoverer)
	r.Use(middleware.StripSlashes)

	r.Get("/", hello)

	allPostPaths, err := allPostPaths()
	if err != nil {
		return nil, err
	} else {
		for _, path := range allPostPaths {
			if path != "" {
				r.Get("/"+path, servePost)
			}
		}
	}

	return r, nil
}

func hello(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte("Hello World!"))
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
