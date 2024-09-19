package main

import (
	"cmp"
	"path/filepath"
	"strings"

	"go.goblog.app/app/pkgs/utils"
)

func (a *goBlog) exportMarkdownFiles(dir string) error {
	posts, err := a.getPosts(&postsRequestConfig{
		withoutRenderedTitle: true,
	})
	if err != nil {
		return err
	}
	dir = cmp.Or(dir, "export")
	for _, p := range posts {
		filename := filepath.Join(dir, p.Path+".md")
		if err := utils.SaveToFile(strings.NewReader(p.contentWithParams()), filename); err != nil {
			return err
		}
	}
	return nil
}
