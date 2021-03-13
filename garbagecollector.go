package main

import (
	"log"
	"runtime"
	"time"
)

func initGC() {
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		for range ticker.C {
			go doGC()
		}
	}()
}

func doGC() {
	var old, new runtime.MemStats
	runtime.ReadMemStats(&old)
	runtime.GC()
	runtime.ReadMemStats(&new)
	log.Printf("Alloc: %v MiB → %v MiB", old.Alloc/1024/1024, new.Alloc/1024/1024)
}
