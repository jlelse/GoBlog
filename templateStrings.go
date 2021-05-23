package main

import (
	ts "git.jlel.se/jlelse/template-strings"
)

var appTs *ts.TemplateStrings

func initTemplateStrings() (err error) {
	var blogLangs []string
	for _, b := range appConfig.Blogs {
		blogLangs = append(blogLangs, b.Lang)
	}
	appTs, err = ts.InitTemplateStrings("templates/strings", ".yaml", "default", blogLangs...)
	return err
}
