package main

import (
	"flag"
	"log"
	"os"
	"runtime"
	"runtime/pprof"

	"github.com/pquerna/otp/totp"
)

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to `file`")
var memprofile = flag.String("memprofile", "", "write memory profile to `file`")

func main() {
	var err error

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
	if err = initConfig(); err != nil {
		log.Fatalln("Failed to init config:", err.Error())
	}

	// Small tools before init
	if len(os.Args) >= 2 && os.Args[1] == "totp-secret" {
		key, err := totp.Generate(totp.GenerateOpts{
			Issuer:      appConfig.Server.PublicAddress,
			AccountName: appConfig.User.Nick,
		})
		if err != nil {
			log.Fatalln(err.Error())
			return
		}
		log.Println("TOTP-Secret:", key.Secret())
		return
	}

	// Init regular garbage collection
	initGC()

	// Execute pre-start hooks
	preStartHooks()

	// Initialize database and markdown
	log.Println("Initialize database...")
	if err = initDatabase(); err != nil {
		log.Fatalln("Failed to init database:", err.Error())
		return
	}
	log.Println("Initialize server components...")
	initMarkdown()

	// Link check tool after init of markdown
	if len(os.Args) >= 2 && os.Args[1] == "check" {
		checkAllExternalLinks()
		return
	}

	// More initializations
	initMinify()
	if err = initTemplateAssets(); err != nil { // Needs minify
		log.Fatalln("Failed to init template assets:", err.Error())
		return
	}
	if err = initTemplateStrings(); err != nil {
		log.Fatalln("Failed to init template translations:", err.Error())
		return
	}
	if err = initRendering(); err != nil { // Needs assets and minify
		log.Fatalln("Failed to init HTML rendering:", err.Error())
		return
	}
	if err = initCache(); err != nil {
		log.Fatalln("Failed to init HTTP cache:", err.Error())
		return
	}
	if err = initRegexRedirects(); err != nil {
		log.Fatalln("Failed to init redirects:", err.Error())
		return
	}
	if err = initHTTPLog(); err != nil {
		log.Fatal("Failed to init HTTP logging:", err.Error())
		return
	}
	if err = initActivityPub(); err != nil {
		log.Fatalln("Failed to init ActivityPub:", err.Error())
		return
	}
	if err = initWebmention(); err != nil {
		log.Fatalln("Failed to init webmention support:", err.Error())
		return
	}
	initTelegram()

	// Start cron hooks
	startHourlyHooks()

	// Start the server
	log.Println("Starting server...")
	err = startServer()
	if err != nil {
		log.Fatalln("Failed to start server:", err.Error())
		return
	}
	log.Println("Stopped server(s)")

	// Wait till everything is shutdown
	waitForShutdown()

	// Close DB
	if err = closeDb(); err != nil {
		log.Fatalln("Failed to close DB:", err.Error())
		return
	}
	log.Println("Closed Database")

	// Write memory profile
	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			log.Fatalln("could not create memory profile: ", err.Error())
			return
		}
		defer f.Close()
		runtime.GC()
		if err := pprof.WriteHeapProfile(f); err != nil {
			log.Fatalln("could not write memory profile: ", err.Error())
			return
		}
	}
}
