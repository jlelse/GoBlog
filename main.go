package main

import (
	"cmp"
	"flag"
	"fmt"
	"net"
	"net/http"
	netpprof "net/http/pprof"
	"os"
	"runtime"
	"runtime/pprof"
	"time"

	"github.com/pquerna/otp/totp"
)

func main() {
	var err error

	// Command line flags
	cpuprofile := flag.String("cpuprofile", "", "write cpu profile to `file`")
	memprofile := flag.String("memprofile", "", "write memory profile to `file`")
	configfile := flag.String("config", "", "use a specific config file")

	// Init app and logger
	app := &goBlog{
		httpClient: newHttpClient(),
	}
	app.initLog()

	// Init CPU and memory profiling
	flag.Parse()
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			app.fatal("could not create CPU profile", "err", err)
			return
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			app.fatal("could not start CPU profile", "err", err)
			return
		}
		defer pprof.StopCPUProfile()
	}
	if *memprofile != "" {
		defer func() {
			f, err := os.Create(*memprofile)
			if err != nil {
				app.fatal("could not create memory profile", "err", err)
				return
			}
			defer f.Close()
			runtime.GC()
			if err := pprof.WriteHeapProfile(f); err != nil {
				app.fatal("could not write memory profile", "err", err)
				return
			}
		}()
	}

	// Initialize config
	if err = app.loadConfigFile(*configfile); err != nil {
		app.logErrAndQuit("Failed to load config file", "err", err)
		return
	}
	if err = app.initConfig(false); err != nil {
		app.logErrAndQuit("Failed to init config", "err", err)
		return
	}

	// Healthcheck tool
	if index := findIndex(os.Args, "healthcheck"); len(os.Args) >= index && index != -1 {
		// Connect to public address + "/ping" and exit with 0 when successful
		health := app.healthcheckExitCode()
		app.shutdown.ShutdownAndWait()
		os.Exit(health)
		return
	}

	// Tool to generate TOTP secret
	if index := findIndex(os.Args, "totp-secret"); len(os.Args) >= index && index != -1 {
		key, err := totp.Generate(totp.GenerateOpts{
			Issuer:      app.cfg.Server.PublicAddress,
			AccountName: app.cfg.User.Nick,
		})
		if err != nil {
			app.logErrAndQuit("Failed to generate TOTP secret", "err", err)
			return
		}
		fmt.Println("TOTP-Secret:", key.Secret())
		app.shutdown.ShutdownAndWait()
		return
	}

	// Initialize plugins
	if err = app.initPlugins(); err != nil {
		app.logErrAndQuit("Failed to init plugins", "err", err)
		return
	}

	// Start pprof server
	if pprofCfg := app.cfg.Pprof; pprofCfg != nil && pprofCfg.Enabled {
		go func() {
			// Build handler
			pprofHandler := http.NewServeMux()
			pprofHandler.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
				http.Redirect(rw, r, "/debug/pprof/", http.StatusFound)
			})
			pprofHandler.HandleFunc("/debug/pprof/", netpprof.Index)
			pprofHandler.HandleFunc("/debug/pprof/{action}", netpprof.Index)
			pprofHandler.HandleFunc("/debug/pprof/cmdline", netpprof.Cmdline)
			pprofHandler.HandleFunc("/debug/pprof/profile", netpprof.Profile)
			pprofHandler.HandleFunc("/debug/pprof/symbol", netpprof.Symbol)
			pprofHandler.HandleFunc("/debug/pprof/trace", netpprof.Trace)
			// Build server and listener
			pprofServer := &http.Server{
				Addr:              cmp.Or(pprofCfg.Address, "localhost:0"),
				Handler:           pprofHandler,
				ReadHeaderTimeout: 1 * time.Minute,
			}
			listener, err := net.Listen("tcp", pprofServer.Addr)
			if err != nil {
				app.fatal("Failed to start pprof server", "err", err)
				return
			}
			app.info("Pprof server listening", "addr", listener.Addr().String())
			// Start server
			if err := pprofServer.Serve(listener); err != nil {
				app.fatal("Failed to start pprof server", "err", err)
				return
			}
		}()
	}

	// Execute pre-start hooks
	app.preStartHooks()

	// Link check tool after init of markdown
	if index := findIndex(os.Args, "check"); len(os.Args) >= index && index != -1 {
		app.initMarkdown()
		err = app.initTemplateStrings()
		if err != nil {
			app.logErrAndQuit("Failed to start check", "err", err)
		}
		err = app.checkAllExternalLinks()
		if err != nil {
			app.logErrAndQuit("Failed to start check", "err", err)
		}
		app.shutdown.ShutdownAndWait()
		return
	}

	// Markdown export
	if index := findIndex(os.Args, "export"); len(os.Args) >= index+1 && index != -1 {
		var dir string
		if len(os.Args) >= 3 {
			dir = os.Args[index+1]
		}
		err = app.exportMarkdownFiles(dir)
		if err != nil {
			app.logErrAndQuit("Failed to export markdown files", "err", err)
			return
		}
		app.shutdown.ShutdownAndWait()
		return
	}

	// Markdown IMPORT
	if importIndex := findIndex(os.Args, "import"); len(os.Args) > importIndex+1 && importIndex != -1 {
		var dir string
		if len(os.Args) >= 3 {
			dir = os.Args[importIndex+1]
		}
		err = app.importMarkdownFiles(dir)
		if err != nil {
			app.logErrAndQuit("Failed to import markdown files", "err", err)
			return
		}
		app.shutdown.ShutdownAndWait()
		return
	}

	// ActivityPub refetch followers
	if index := findIndex(os.Args, "activitypub"); len(os.Args) >= index && index != -1 {
		if !app.apEnabled() {
			app.logErrAndQuit("ActivityPub not enabled")
			return
		}
		if err = app.initActivityPubBase(); err != nil {
			app.logErrAndQuit("Failed to init ActivityPub base", "err", err)
			return
		}
		if i2 := findIndex(os.Args, "refetch-followers"); len(os.Args) >= i2+1 && i2 != -1 {
			blog := os.Args[i2+1]
			if err = app.apRefetchFollowers(blog); err != nil {
				app.logErrAndQuit("Failed to refetch ActivityPub followers", "blog", blog, "err", err)
				return
			}
			app.shutdown.ShutdownAndWait()
			return
		}
	}

	// Initialize components
	app.initComponents()

	// Start cron hooks
	app.startHourlyHooks()

	// Start the server
	err = app.startServer()
	if err != nil {
		app.logErrAndQuit("Failed to start server(s)", "err", err)
		return
	}

	// Wait till everything is shutdown
	app.shutdown.Wait()
}

func (app *goBlog) initComponents() {
	var err error

	app.info("Initialize components...")

	app.initMarkdown()
	if err = app.initTemplateAssets(); err != nil { // Needs minify
		app.logErrAndQuit("Failed to init template assets", "err", err)
		return
	}
	if err = app.initTemplateStrings(); err != nil {
		app.logErrAndQuit("Failed to init template translations", "err", err)
		return
	}
	if err = app.initCache(); err != nil {
		app.logErrAndQuit("Failed to init HTTP cache", "err", err)
		return
	}
	if err = app.initRegexRedirects(); err != nil {
		app.logErrAndQuit("Failed to init redirects", "err", err)
		return
	}
	if err = app.initHTTPLog(); err != nil {
		app.logErrAndQuit("Failed to init HTTP logging", "err", err)
		return
	}
	if err = app.initActivityPub(); err != nil {
		app.logErrAndQuit("Failed to init ActivityPub", "err", err)
		return
	}
	if err = app.initWebAuthn(); err != nil {
		app.logErrAndQuit("Failed to init WebAuthn", "err", err)
		return
	}
	app.initWebmention()
	app.initTelegram()
	app.initAtproto()
	app.initBlogStats()
	app.initTTS()
	app.initSessions()
	app.initIndieAuth()
	app.startPostsScheduler()
	app.initPostsDeleter()
	app.initIndexNow()

	app.info("Initialized components")
}

func (a *goBlog) logErrAndQuit(msg string, args ...any) {
	a.error(msg, args...)
	a.shutdown.ShutdownAndWait()
	os.Exit(1)
}

func findIndex(slice []string, val string) int {
	for i, v := range slice {
		if v == val {
			return i
		}
	}
	return -1 // Value not found
}
