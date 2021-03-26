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

func executeTemplateCommand(hookType string, tmpl string, data map[string]interface{}) {
	cmdTmpl, err := template.New("cmd").Parse(tmpl)
	if err != nil {
		log.Println("Failed to parse cmd template:", err.Error())
		return
	}
	var cmdBuf bytes.Buffer
	if err = cmdTmpl.Execute(&cmdBuf, data); err != nil {
		log.Println("Failed to execute cmd template:", err.Error())
		return
	}
	cmd := cmdBuf.String()
	log.Println("Executing "+hookType+" hook:", cmd)
	executeCommand(cmd)
}

var hourlyHooks = []func(){}

func startHourlyHooks() {
	// Add configured hourly hooks
	for _, cmd := range appConfig.Hooks.Hourly {
		c := cmd
		f := func() {
			log.Println("Executing hourly hook:", c)
			executeCommand(c)
		}
		hourlyHooks = append(hourlyHooks, f)
	}
	// Calculate waiting time for first exec
	n := time.Now()
	f := time.Date(n.Year(), n.Month(), n.Day(), n.Hour(), 0, 0, 0, n.Location()).Add(time.Hour)
	w := f.Sub(n)
	// When there are hooks, start ticker
	if len(hourlyHooks) > 0 {
		go func() {
			// Wait for next hour to begin
			time.Sleep(w)
			// Execute once
			for _, f := range hourlyHooks {
				go f()
			}
			// Start ticker and execute regularly
			ticker := time.NewTicker(1 * time.Hour)
			for range ticker.C {
				for _, f := range hourlyHooks {
					go f()
				}
			}
		}()
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
