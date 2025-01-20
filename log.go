package main

import (
	"log/slog"
	"os"
)

func (a *goBlog) initLog() {
	a.initLogOnce.Do(func() {
		a.logLevel = new(slog.LevelVar)
		a.logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: a.logLevel,
		}))
	})
}

func (a *goBlog) updateLogLevel() {
	a.initLog()
	if a.cfg.Debug {
		a.logLevel.Set(slog.LevelDebug)
	}
}

func (a *goBlog) debug(msg string, args ...any) {
	a.initLog()
	a.logger.Debug(msg, args...)
}

func (a *goBlog) info(msg string, args ...any) {
	a.initLog()
	a.logger.Info(msg, args...)
}

func (a *goBlog) error(msg string, args ...any) {
	a.initLog()
	a.logger.Error(msg, args...)
}

func (a *goBlog) fatal(msg string, args ...any) {
	a.initLog()
	a.error(msg, args...)
	os.Exit(1)
}
