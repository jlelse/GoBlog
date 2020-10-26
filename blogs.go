package main

import "strings"

func (blog *configBlog) getRelativePath(path string) string {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if blog.Path != "/" {
		return blog.Path + path
	}
	return path
}
