package main

import (
	"log"
	"os/exec"
	"time"
)

func preStartHooks() {
	for _, cmd := range appConfig.Hooks.PreStart {
		log.Println("Executing pre-start hook:", cmd)
		executeCommand(cmd)
	}
}

func startHourlyHooks() {
	for _, cmd := range appConfig.Hooks.Hourly {
		go func(cmd string) {
			run := func() {
				log.Println("Executing hourly hook:", cmd)
				executeCommand(cmd)
			}
			// Execute once
			run()
			// Start ticker and execute regularly
			ticker := time.NewTicker(1 * time.Hour)
			for range ticker.C {
				run()
			}
		}(cmd)
	}
}

func executeCommand(cmd string) {
	out, err := exec.Command(appConfig.Hooks.Shell, "-c", cmd).CombinedOutput()
	if err != nil {
		log.Println("Failed to execute command:", err.Error())
	}
	log.Println("Output:")
	log.Print(string(out))
}
