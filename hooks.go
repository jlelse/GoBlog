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
		func(cmd string) {
			log.Println("Executing pre-start hook:", cmd)
			executeCommand(cmd)
		}(cmd)
	}
}

type postHookType string

const (
	postPostHook   postHookType = "post"
	postUpdateHook postHookType = "update"
	postDeleteHook postHookType = "delete"
)

var postHooks = map[postHookType][]func(*post){}

func (p *post) postPostHooks() {
	// Hooks after post published
	for _, cmdTmplString := range appConfig.Hooks.PostPost {
		go func(p *post, cmdTmplString string) {
			executeTemplateCommand("post-post", cmdTmplString, map[string]interface{}{
				"URL":  p.fullURL(),
				"Post": p,
			})
		}(p, cmdTmplString)
	}
	for _, f := range postHooks[postPostHook] {
		go f(p)
	}
}

func (p *post) postUpdateHooks() {
	// Hooks after post updated
	for _, cmdTmplString := range appConfig.Hooks.PostUpdate {
		go func(p *post, cmdTmplString string) {
			executeTemplateCommand("post-update", cmdTmplString, map[string]interface{}{
				"URL":  p.fullURL(),
				"Post": p,
			})
		}(p, cmdTmplString)
	}
	for _, f := range postHooks[postUpdateHook] {
		go f(p)
	}
}

func (p *post) postDeleteHooks() {
	for _, cmdTmplString := range appConfig.Hooks.PostDelete {
		go func(p *post, cmdTmplString string) {
			executeTemplateCommand("post-delete", cmdTmplString, map[string]interface{}{
				"URL":  p.fullURL(),
				"Post": p,
			})
		}(p, cmdTmplString)
	}
	for _, f := range postHooks[postDeleteHook] {
		go f(p)
	}
}

type hookTemplateData struct {
	URL string
}

func executeTemplateCommand(hookType string, tmpl string, data map[string]interface{}) {
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
	if len(out) > 0 {
		log.Println("Output:")
		log.Print(string(out))
	}
}
