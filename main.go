package main

import (
	"log"
	"os"
	"os/signal"
)

func main() {
	// Initialize all things
	log.Println("Initializing...")
	err := initConfig()
	if err != nil {
		log.Fatal(err)
	}
	err = initDatabase()
	if err != nil {
		log.Fatal(err)
		return
	}
	initMinify()
	err = initTemplateAssets() // Needs minify
	if err != nil {
		log.Fatal(err)
		return
	}
	initMarkdown()
	initRendering() // Needs assets

	// Prepare graceful shutdown
	quit := make(chan os.Signal, 1)

	// Start the server
	go func() {
		log.Println("Starting...")
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
