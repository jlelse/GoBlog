package main

import (
	"log"
	"os"
	"os/signal"

	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

func main() {
	// Initialize config
	log.Println("Initialize configuration...")
	err := initConfig()
	if err != nil {
		log.Fatal(err)
	}
	// Execute pre-start hooks
	preStartHooks()
	// Initialize everything else
	log.Println("Initialize database...")
	err = initDatabase()
	if err != nil {
		log.Fatal(err)
		return
	}
	log.Println("Initialize server components...")
	initMinify()
	initMarkdown()
	err = initTemplateAssets() // Needs minify
	if err != nil {
		log.Fatal(err)
		return
	}
	err = initTemplateStrings()
	if err != nil {
		log.Fatal(err)
		return
	}
	err = initRendering() // Needs assets
	if err != nil {
		log.Fatal(err)
		return
	}
	err = initRegexRedirects()
	if err != nil {
		log.Fatal(err)
		return
	}
	initCache()

	// Start cron hooks
	startHourlyHooks()

	// Prepare graceful shutdown
	quit := make(chan os.Signal, 1)

	// Start the server
	go func() {
		log.Println("Starting server...")
		err = startServer()
		if err != nil {
			log.Println("Failed to start server:")
			log.Println(err)
		}
		quit <- os.Interrupt
	}()

	// Graceful shutdown
	signal.Notify(quit, os.Interrupt)
	<-quit
	log.Println("Stopping...")

	// Close DB
	err = closeDb()
	if err != nil {
		log.Fatal(err)
		return
	}
}
