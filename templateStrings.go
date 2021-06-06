package main

import (
	ts "git.jlel.se/jlelse/template-strings"
)

func (a *goBlog) initTemplateStrings() (err error) {
	var blogLangs []string
	for _, b := range a.cfg.Blogs {
		blogLangs = append(blogLangs, b.Lang)
	}
	a.ts, err = ts.InitTemplateStrings("templates/strings", ".yaml", "default", blogLangs...)
	return err
}
