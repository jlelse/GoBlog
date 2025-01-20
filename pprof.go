package main

import (
	"cmp"
	"net"
	"net/http"
	"net/http/pprof"
	"time"
)

func (app *goBlog) startPprofServer() {
	if pprofCfg := app.cfg.Pprof; pprofCfg != nil && pprofCfg.Enabled {
		go func() {
			pprofHandler := http.NewServeMux()
			pprofHandler.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
				http.Redirect(rw, r, "/debug/pprof/", http.StatusFound)
			})
			pprofHandler.HandleFunc("/debug/pprof/", pprof.Index)
			pprofHandler.HandleFunc("/debug/pprof/profile", pprof.Profile)

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
			if err := pprofServer.Serve(listener); err != nil {
				app.fatal("Failed to start pprof server", "err", err)
				return
			}
		}()
	}
}
