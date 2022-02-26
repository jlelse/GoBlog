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
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	runtime.GC()
	runtime.ReadMemStats(&after)
	log.Println(fmt.Sprintf(
		"\nAlloc: %d MiB -> %d MiB\nSys: %d MiB -> %d MiB\nNumGC: %d",
		before.Alloc/1024/1024, after.Alloc/1024/1024,
		before.Sys/1024/1024, after.Sys/1024/1024,
		after.NumGC,
	))
}
