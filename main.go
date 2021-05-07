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

	// Init CPU and memory profiling
	flag.Parse()
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatalln("could not create CPU profile: ", err)
			return
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatalln("could not start CPU profile: ", err)
			return
		}
		defer pprof.StopCPUProfile()
	}
	if *memprofile != "" {
		defer func() {
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
		}()
	}

	// Initialize config
	log.Println("Initialize configuration...")
	if err = initConfig(); err != nil {
		logErrAndQuit("Failed to init config:", err.Error())
		return
	}

	// Healthcheck tool
	if len(os.Args) >= 2 && os.Args[1] == "healthcheck" {
		// Connect to public address + "/ping" and exit with 0 when successful
		health := healthcheckExitCode()
		shutdown()
		os.Exit(health)
		return
	}

	// Tool to generate TOTP secret
	if len(os.Args) >= 2 && os.Args[1] == "totp-secret" {
		key, err := totp.Generate(totp.GenerateOpts{
			Issuer:      appConfig.Server.PublicAddress,
			AccountName: appConfig.User.Nick,
		})
		if err != nil {
			logErrAndQuit(err.Error())
			return
		}
		log.Println("TOTP-Secret:", key.Secret())
		shutdown()
		return
	}

	// Init regular garbage collection
	initGC()

	// Execute pre-start hooks
	preStartHooks()

	// Initialize database and markdown
	log.Println("Initialize database...")
	if err = initDatabase(); err != nil {
		logErrAndQuit("Failed to init database:", err.Error())
		return
	}
	log.Println("Initialize server components...")
	initMarkdown()

	// Link check tool after init of markdown
	if len(os.Args) >= 2 && os.Args[1] == "check" {
		checkAllExternalLinks()
		shutdown()
		return
	}

	// More initializations
	initMinify()
	if err = initTemplateAssets(); err != nil { // Needs minify
		logErrAndQuit("Failed to init template assets:", err.Error())
		return
	}
	if err = initTemplateStrings(); err != nil {
		logErrAndQuit("Failed to init template translations:", err.Error())
		return
	}
	if err = initRendering(); err != nil { // Needs assets and minify
		logErrAndQuit("Failed to init HTML rendering:", err.Error())
		return
	}
	if err = initCache(); err != nil {
		logErrAndQuit("Failed to init HTTP cache:", err.Error())
		return
	}
	if err = initRegexRedirects(); err != nil {
		logErrAndQuit("Failed to init redirects:", err.Error())
		return
	}
	if err = initHTTPLog(); err != nil {
		logErrAndQuit("Failed to init HTTP logging:", err.Error())
		return
	}
	if err = initActivityPub(); err != nil {
		logErrAndQuit("Failed to init ActivityPub:", err.Error())
		return
	}
	if err = initWebmention(); err != nil {
		logErrAndQuit("Failed to init webmention support:", err.Error())
		return
	}
	initTelegram()

	// Start cron hooks
	startHourlyHooks()

	// Start the server
	log.Println("Starting server(s)...")
	err = startServer()
	if err != nil {
		logErrAndQuit("Failed to start server(s):", err.Error())
		return
	}

	// Wait till everything is shutdown
	waitForShutdown()

}

func logErrAndQuit(v ...interface{}) {
	log.Println(v...)
	shutdown()
	os.Exit(1)
}
