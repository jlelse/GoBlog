package main

import (
	"html/template"
	"log"
	"os/exec"
	"time"

	"go.goblog.app/app/pkgs/bufferpool"
	"go.goblog.app/app/pkgs/plugintypes"
)

func (a *goBlog) preStartHooks() {
	cfg := a.cfg.Hooks
	for _, cmd := range cfg.PreStart {
		func(cmd string) {
			executeHookCommand("pre-start", cfg.Shell, cmd)
		}(cmd)
	}
}

type postHookFunc func(*post)

func (a *goBlog) postPostHooks(p *post) {
	// Hooks after post published
	if hc := a.cfg.Hooks; hc != nil {
		for _, cmdTmplString := range hc.PostPost {
			go func(p *post, cmdTmplString string) {
				a.cfg.Hooks.executeTemplateCommand("post-post", cmdTmplString, map[string]any{
					"URL":  a.fullPostURL(p),
					"Post": p,
				})
			}(p, cmdTmplString)
		}
	}
	for _, f := range a.pPostHooks {
		go f(p)
	}
	for _, plugin := range a.getPlugins(pluginPostCreatedHookType) {
		go plugin.(plugintypes.PostCreatedHook).PostCreated(p)
	}
}

func (a *goBlog) postUpdateHooks(p *post) {
	// Hooks after post updated
	if hc := a.cfg.Hooks; hc != nil {
		for _, cmdTmplString := range hc.PostUpdate {
			go func(p *post, cmdTmplString string) {
				a.cfg.Hooks.executeTemplateCommand("post-update", cmdTmplString, map[string]any{
					"URL":  a.fullPostURL(p),
					"Post": p,
				})
			}(p, cmdTmplString)
		}
	}
	for _, f := range a.pUpdateHooks {
		go f(p)
	}
	for _, plugin := range a.getPlugins(pluginPostUpdatedHookType) {
		go plugin.(plugintypes.PostUpdatedHook).PostUpdated(p)
	}
}

func (a *goBlog) postDeleteHooks(p *post) {
	if hc := a.cfg.Hooks; hc != nil {
		for _, cmdTmplString := range hc.PostDelete {
			go func(p *post, cmdTmplString string) {
				a.cfg.Hooks.executeTemplateCommand("post-delete", cmdTmplString, map[string]any{
					"URL":  a.fullPostURL(p),
					"Post": p,
				})
			}(p, cmdTmplString)
		}
	}
	for _, f := range a.pDeleteHooks {
		go f(p)
	}
	for _, plugin := range a.getPlugins(pluginPostDeletedHookType) {
		go plugin.(plugintypes.PostDeletedHook).PostDeleted(p)
	}
}

func (a *goBlog) postUndeleteHooks(p *post) {
	if hc := a.cfg.Hooks; hc != nil {
		for _, cmdTmplString := range hc.PostUndelete {
			go func(p *post, cmdTmplString string) {
				a.cfg.Hooks.executeTemplateCommand("post-undelete", cmdTmplString, map[string]any{
					"URL":  a.fullPostURL(p),
					"Post": p,
				})
			}(p, cmdTmplString)
		}
	}
	for _, f := range a.pUndeleteHooks {
		go f(p)
	}
}

func (cfg *configHooks) executeTemplateCommand(hookType string, tmpl string, data map[string]any) {
	cmdTmpl, err := template.New("cmd").Parse(tmpl)
	if err != nil {
		log.Println("Failed to parse cmd template:", err.Error())
		return
	}
	cmdBuf := bufferpool.Get()
	defer bufferpool.Put(cmdBuf)
	if err = cmdTmpl.Execute(cmdBuf, data); err != nil {
		log.Println("Failed to execute cmd template:", err.Error())
		return
	}
	executeHookCommand(hookType, cfg.Shell, cmdBuf.String())
}

type hourlyHookFunc func()

func (a *goBlog) startHourlyHooks() {
	cfg := a.cfg.Hooks
	// Add configured hourly hooks
	for _, cmd := range cfg.Hourly {
		c := cmd
		f := func() {
			executeHookCommand("hourly", cfg.Shell, c)
		}
		a.hourlyHooks = append(a.hourlyHooks, f)
	}
	// When there are hooks, start ticker
	if len(a.hourlyHooks) > 0 {
		// Wait for next full hour
		tr := time.AfterFunc(time.Until(time.Now().Truncate(time.Hour).Add(time.Hour)), func() {
			// Execute once
			for _, f := range a.hourlyHooks {
				go f()
			}
			// Start ticker and execute regularly
			ticker := time.NewTicker(1 * time.Hour)
			a.shutdown.Add(func() {
				ticker.Stop()
				log.Println("Stopped hourly hooks")
			})
			for range ticker.C {
				for _, f := range a.hourlyHooks {
					go f()
				}
			}
		})
		a.shutdown.Add(func() {
			if tr.Stop() {
				log.Println("Canceled hourly hooks")
			}
		})
	}
}

func executeHookCommand(hookType, shell, cmd string) {
	log.Printf("Executing %v hook: %v", hookType, cmd)
	out, err := exec.Command(shell, "-c", cmd).CombinedOutput()
	if err != nil {
		log.Println("Failed to execute command:", err.Error())
	}
	if len(out) > 0 {
		log.Printf("Output:\n%v", string(out))
	}
}
