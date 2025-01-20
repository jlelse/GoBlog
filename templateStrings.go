package main

import (
	"embed"

	ts "git.jlel.se/jlelse/template-strings"
	"github.com/samber/lo"
)

//go:embed strings/*
var stringsFiles embed.FS

func (a *goBlog) initTemplateStrings() (err error) {
	blogLangs := lo.Uniq(lo.MapToSlice(a.cfg.Blogs, func(_ string, blog *configBlog) string {
		return blog.Lang
	}))
	a.ts, err = ts.InitTemplateStringsFS(stringsFiles, "strings", ".yaml", "default", blogLangs...)
	return err
}
