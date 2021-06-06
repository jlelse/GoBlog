package main

import (
	"bytes"
	"html/template"
	"log"
	"os/exec"
	"time"
)

func (a *goBlog) preStartHooks() {
	for _, cmd := range a.cfg.Hooks.PreStart {
		func(cmd string) {
			log.Println("Executing pre-start hook:", cmd)
			a.cfg.Hooks.executeCommand(cmd)
		}(cmd)
	}
}

type postHookFunc func(*post)

func (a *goBlog) postPostHooks(p *post) {
	// Hooks after post published
	for _, cmdTmplString := range a.cfg.Hooks.PostPost {
		go func(p *post, cmdTmplString string) {
			a.cfg.Hooks.executeTemplateCommand("post-post", cmdTmplString, map[string]interface{}{
				"URL":  a.fullPostURL(p),
				"Post": p,
			})
		}(p, cmdTmplString)
	}
	for _, f := range a.pPostHooks {
		go f(p)
	}
}

func (a *goBlog) postUpdateHooks(p *post) {
	// Hooks after post updated
	for _, cmdTmplString := range a.cfg.Hooks.PostUpdate {
		go func(p *post, cmdTmplString string) {
			a.cfg.Hooks.executeTemplateCommand("post-update", cmdTmplString, map[string]interface{}{
				"URL":  a.fullPostURL(p),
				"Post": p,
			})
		}(p, cmdTmplString)
	}
	for _, f := range a.pUpdateHooks {
		go f(p)
	}
}

func (a *goBlog) postDeleteHooks(p *post) {
	for _, cmdTmplString := range a.cfg.Hooks.PostDelete {
		go func(p *post, cmdTmplString string) {
			a.cfg.Hooks.executeTemplateCommand("post-delete", cmdTmplString, map[string]interface{}{
				"URL":  a.fullPostURL(p),
				"Post": p,
			})
		}(p, cmdTmplString)
	}
	for _, f := range a.pDeleteHooks {
		go f(p)
	}
}

func (cfg *configHooks) executeTemplateCommand(hookType string, tmpl string, data map[string]interface{}) {
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
	cfg.executeCommand(cmd)
}

var hourlyHooks = []func(){}

func (a *goBlog) startHourlyHooks() {
	// Add configured hourly hooks
	for _, cmd := range a.cfg.Hooks.Hourly {
		c := cmd
		f := func() {
			log.Println("Executing hourly hook:", c)
			a.cfg.Hooks.executeCommand(c)
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

func (cfg *configHooks) executeCommand(cmd string) {
	out, err := exec.Command(cfg.Shell, "-c", cmd).CombinedOutput()
	if err != nil {
		log.Println("Failed to execute command:", err.Error())
	}
	if len(out) > 0 {
		log.Println("Output:")
		log.Print(string(out))
	}
}
