package main

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"net/http"
	"strconv"
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
	e.Logger.Fatal(e.Start(address))
}

func hello(c echo.Context) error {
	return c.String(http.StatusOK, "Hello, World!")
}
