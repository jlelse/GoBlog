package main

import (
	"os"
	"path/filepath"
)

func (a *goBlog) exportMarkdownFiles(dir string) error {
	posts, err := a.getPosts(&postsRequestConfig{
		withoutRenderedTitle: true,
	})
	if err != nil {
		return err
	}
	dir = defaultIfEmpty(dir, "export")
	for _, p := range posts {
		filename := filepath.Join(dir, p.Path+".md")
		filedir := filepath.Dir(filename)
		err = os.MkdirAll(filedir, 0644)
		if err != nil {
			return err
		}
		err = os.WriteFile(filename, []byte(p.contentWithParams()), 0644)
		if err != nil {
			return err
		}
	}
	return nil
}
