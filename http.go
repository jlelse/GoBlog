package main

import (
	"context"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"time"
)

func startServer() {
	e := echo.New()

	if appConfig.server.logging {
		e.Use(middleware.Logger())
	}
	e.Use(middleware.Recover(), middleware.Gzip())

	e.GET("/", hello)
	e.GET("/*", servePost)

	address := ":" + strconv.Itoa(appConfig.server.port)
	go func() {
		if err := e.Start(address); err != nil {
			log.Println("Shutting down the server")
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server with a timeout of 10 seconds.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		e.Logger.Fatal(err)
	}
}

func hello(c echo.Context) error {
	return c.String(http.StatusOK, "Hello, World!")
}
