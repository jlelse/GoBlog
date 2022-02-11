package main

import (
	"embed"

	ts "git.jlel.se/jlelse/template-strings"
)

//go:embed strings/*
var stringsFiles embed.FS

func (a *goBlog) initTemplateStrings() (err error) {
	var blogLangs []string
	for _, b := range a.cfg.Blogs {
		blogLangs = append(blogLangs, b.Lang)
	}
	a.ts, err = ts.InitTemplateStringsFS(stringsFiles, "strings", ".yaml", "default", blogLangs...)
	return err
}
