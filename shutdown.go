package main

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
)

var shutdownWg sync.WaitGroup

func onShutdown(f func()) {
	defer shutdownWg.Done()
	shutdownWg.Add(1)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	f()
}

func waitForShutdown() {
	shutdownWg.Wait()
}
