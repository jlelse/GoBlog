package main

import (
	"bytes"
	"html/template"
	"log"
	"os/exec"
	"time"
)

func preStartHooks() {
	for _, cmd := range appConfig.Hooks.PreStart {
		go func(cmd string) {
			log.Println("Executing pre-start hook:", cmd)
			executeCommand(cmd)
		}(cmd)
	}
}

func postPostHooks(path string) {
	for _, cmdTmplString := range appConfig.Hooks.PostPost {
		go func(path, cmdTmplString string) {
			executeTemplateCommand("post-post", cmdTmplString, &hookTemplateData{
				URL: appConfig.Server.PublicAddress + path,
			})
		}(path, cmdTmplString)
	}
}

func postDeleteHooks(path string) {
	for _, cmdTmplString := range appConfig.Hooks.PostDelete {
		go func(path, cmdTmplString string) {
			executeTemplateCommand("post-delete", cmdTmplString, &hookTemplateData{
				URL: appConfig.Server.PublicAddress + path,
			})
		}(path, cmdTmplString)
	}
}

type hookTemplateData struct {
	URL string
}

func executeTemplateCommand(hookType string, tmpl string, data *hookTemplateData) {
	cmdTmpl, err := template.New("cmd").Parse(tmpl)
	if err != nil {
		log.Println("Failed to parse cmd template:", err.Error())
		return
	}
	var cmdBuf bytes.Buffer
	cmdTmpl.Execute(&cmdBuf, data)
	cmd := cmdBuf.String()
	log.Println("Executing "+hookType+" hook:", cmd)
	executeCommand(cmd)
}

func startHourlyHooks() {
	for _, cmd := range appConfig.Hooks.Hourly {
		go func(cmd string) {
			run := func() {
				log.Println("Executing hourly hook:", cmd)
				executeCommand(cmd)
			}
			// Execute once
			go run()
			// Start ticker and execute regularly
			ticker := time.NewTicker(1 * time.Hour)
			for range ticker.C {
				go run()
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
