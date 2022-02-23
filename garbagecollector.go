package main

import (
	"fmt"
	"log"
	"runtime"
	"time"
)

func initGC() {
	go func() {
		ticker := time.NewTicker(15 * time.Minute)
		for range ticker.C {
			doGC()
		}
	}()
}

func doGC() {
	var old, new runtime.MemStats
	runtime.ReadMemStats(&old)
	runtime.GC()
	runtime.ReadMemStats(&new)
	log.Println(fmt.Sprintf(
		"\nAlloc: %d MiB -> %d MiB\nSys: %d MiB -> %d MiB\nNumGC: %d",
		old.Alloc/1024/1024, new.Alloc/1024/1024,
		old.Sys/1024/1024, new.Sys/1024/1024,
		new.NumGC,
	))
}
