package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"runtime/pprof"
	"syscall"

	"github.com/pquerna/otp/totp"
)

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to `file`")
var memprofile = flag.String("memprofile", "", "write memory profile to `file`")

func main() {
	// Init CPU profiling
	flag.Parse()
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal("could not create CPU profile: ", err)
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal("could not start CPU profile: ", err)
		}
		defer pprof.StopCPUProfile()
	}

	// Initialize config
	log.Println("Initialize configuration...")
	err := initConfig()
	if err != nil {
		log.Fatal(err)
	}

	// Small tools
	if len(os.Args) >= 2 {
		if os.Args[1] == "totp-secret" {
			key, err := totp.Generate(totp.GenerateOpts{
				Issuer:      appConfig.Server.PublicAddress,
				AccountName: appConfig.User.Nick,
			})
			if err != nil {
				log.Fatal(err.Error())
				return
			}
			log.Println("TOTP-Secret:", key.Secret())
			return
		}
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
	err = initCache()
	if err != nil {
		log.Fatal(err)
		return
	}
	err = initRegexRedirects()
	if err != nil {
		log.Fatal(err)
		return
	}
	err = initHTTPLog()
	if err != nil {
		log.Fatal(err)
		return
	}
	err = initActivityPub()
	if err != nil {
		log.Fatal(err)
		return
	}
	err = initWebmention()
	if err != nil {
		log.Fatal(err)
		return
	}
	initTelegram()

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
	signal.Notify(quit, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Stopping...")

	// Write memory profile
	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			log.Fatal("could not create memory profile: ", err)
		}
		defer f.Close()
		// runtime.GC() // get up-to-date statistics
		if err := pprof.WriteHeapProfile(f); err != nil {
			log.Fatal("could not write memory profile: ", err)
		}
	}

	// Close DB
	err = closeDb()
	if err != nil {
		log.Fatal(err)
		return
	}
}
