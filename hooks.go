package main

import (
	"bytes"
	"fmt"
	"log"
	"os/exec"
)

func preStartHooks() {
	for _, cmd := range appConfig.Hooks.PreStart {
		log.Println("Executing pre-start hook:", cmd)
		executeCommand(cmd)
	}
}

func executeCommand(cmd string) {
	var stdout, stderr bytes.Buffer
	parsed := exec.Command(appConfig.Hooks.Shell, "-c", cmd)
	parsed.Stdout = &stdout
	parsed.Stderr = &stderr
	cmdErr := parsed.Run()
	if cmdErr != nil {
		fmt.Println("Executing command failed:")
		fmt.Println(cmdErr.Error())
	}
	if stdout.Len() > 0 {
		log.Println("Output:")
		log.Print(stdout.String())
	}
	if stderr.Len() > 0 {
		log.Println("Error:")
		log.Print(stderr.String())
	}
}
