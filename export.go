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
		_ = os.MkdirAll(filedir, 0777)
		//nolint:gosec
		err = os.WriteFile(filename, []byte(p.contentWithParams()), 0666)
		if err != nil {
			return err
		}
	}
	return nil
}
