package main

import (
	"flag"
	"log"
	"os"
	"runtime"
	"runtime/pprof"

	"github.com/pquerna/otp/totp"
)

func main() {
	var err error

	// Command line flags
	cpuprofile := flag.String("cpuprofile", "", "write cpu profile to `file`")
	memprofile := flag.String("memprofile", "", "write memory profile to `file`")
	configfile := flag.String("config", "", "use a specific config file")

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

	// Init regular garbage collection
	initGC()

	app := &goBlog{
		httpClient: &appHttpClient{},
	}

	// Initialize config
	if err = app.initConfig(*configfile); err != nil {
		app.logErrAndQuit("Failed to init config:", err.Error())
		return
	}

	// Healthcheck tool
	if len(os.Args) >= 2 && os.Args[1] == "healthcheck" {
		// Connect to public address + "/ping" and exit with 0 when successful
		health := app.healthcheckExitCode()
		app.shutdown.ShutdownAndWait()
		os.Exit(health)
		return
	}

	// Tool to generate TOTP secret
	if len(os.Args) >= 2 && os.Args[1] == "totp-secret" {
		key, err := totp.Generate(totp.GenerateOpts{
			Issuer:      app.cfg.Server.PublicAddress,
			AccountName: app.cfg.User.Nick,
		})
		if err != nil {
			app.logErrAndQuit(err.Error())
			return
		}
		log.Println("TOTP-Secret:", key.Secret())
		app.shutdown.ShutdownAndWait()
		return
	}

	// Execute pre-start hooks
	app.preStartHooks()

	// Initialize database
	if err = app.initDatabase(true); err != nil {
		app.logErrAndQuit("Failed to init database:", err.Error())
		return
	}

	// Link check tool after init of markdown
	if len(os.Args) >= 2 && os.Args[1] == "check" {
		app.initMarkdown()
		app.checkAllExternalLinks()
		app.shutdown.ShutdownAndWait()
		return
	}

	// Markdown export
	if len(os.Args) >= 2 && os.Args[1] == "export" {
		var dir string
		if len(os.Args) >= 3 {
			dir = os.Args[2]
		}
		err = app.exportMarkdownFiles(dir)
		if err != nil {
			app.logErrAndQuit("Failed to export markdown files:", err.Error())
			return
		}
		app.shutdown.ShutdownAndWait()
		return
	}

	// Initialize components
	app.initComponents(true)

	// Start cron hooks
	app.startHourlyHooks()

	// Start the server
	err = app.startServer()
	if err != nil {
		app.logErrAndQuit("Failed to start server(s):", err.Error())
		return
	}

	// Wait till everything is shutdown
	app.shutdown.Wait()
}

func (app *goBlog) initComponents(logging bool) {
	var err error
	// Log start
	if logging {
		log.Println("Initialize components...")
	}
	app.initMarkdown()
	if err = app.initTemplateAssets(); err != nil { // Needs minify
		app.logErrAndQuit("Failed to init template assets:", err.Error())
		return
	}
	if err = app.initTemplateStrings(); err != nil {
		app.logErrAndQuit("Failed to init template translations:", err.Error())
		return
	}
	if err = app.initRendering(); err != nil { // Needs assets and minify
		app.logErrAndQuit("Failed to init HTML rendering:", err.Error())
		return
	}
	if err = app.initCache(); err != nil {
		app.logErrAndQuit("Failed to init HTTP cache:", err.Error())
		return
	}
	if err = app.initRegexRedirects(); err != nil {
		app.logErrAndQuit("Failed to init redirects:", err.Error())
		return
	}
	if err = app.initHTTPLog(); err != nil {
		app.logErrAndQuit("Failed to init HTTP logging:", err.Error())
		return
	}
	if err = app.initActivityPub(); err != nil {
		app.logErrAndQuit("Failed to init ActivityPub:", err.Error())
		return
	}
	app.initWebmention()
	app.initTelegram()
	app.initBlogStats()
	app.initSessions()
	app.initIndieAuth()
	// Log finish
	if logging {
		log.Println("Initialized components")
	}
}

func (a *goBlog) logErrAndQuit(v ...interface{}) {
	log.Println(v...)
	a.shutdown.ShutdownAndWait()
	os.Exit(1)
}
