package main

import (
	"runtime/debug"
)

func initGC() {
	// Set memory limit to 100 MB
	debug.SetMemoryLimit(100 * 1000 * 1000)
}
