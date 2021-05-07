package main

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
)

var (
	quit                 = make(chan os.Signal, 1)
	shutdownFuncs        = []func(){}
	shutdownWg           sync.WaitGroup
	shutdownFuncMapMutex sync.Mutex
)

func init() {
	signal.Notify(quit,
		os.Interrupt,
		syscall.SIGINT,
		syscall.SIGTERM, // e.g. Docker stop
	)
	go func() {
		<-quit
		shutdown()
	}()
}

func addShutdownFunc(f func()) {
	shutdownWg.Add(1)
	shutdownFuncMapMutex.Lock()
	shutdownFuncs = append(shutdownFuncs, f)
	shutdownFuncMapMutex.Unlock()
}

func shutdown() {
	for _, f := range shutdownFuncs {
		go func(f func()) {
			defer shutdownWg.Done()
			f()
		}(f)
	}
	shutdownWg.Wait()
}

func waitForShutdown() {
	shutdownWg.Wait()
}
