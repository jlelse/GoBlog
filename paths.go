package main

import "strings"

func (a *goBlog) getRelativePath(blog, path string) string {
	// Get blog
	bc := a.cfg.Blogs[blog]
	if bc == nil {
		return ""
	}
	// Get relative path
	return bc.getRelativePath(path)
}

func (blog *configBlog) getRelativePath(path string) string {
	// Check if path is absolute
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	// Check if blog uses subpath
	if blog.Path != "" && blog.Path != "/" {
		// Check if path is root
		if path == "/" {
			path = blog.Path
		} else {
			path = blog.Path + path
		}
	}
	return path
}

func (a *goBlog) getFullAddress(path string) string {
	// Call method with just the relevant config
	return a.cfg.Server.getFullAddress(path)
}

func (cfg *configServer) getFullAddress(path string) string {
	// Check if it is already an absolute URL
	if isAbsoluteURL(path) {
		return path
	}
	// Remove trailing slash
	pa := strings.TrimSuffix(cfg.PublicAddress, "/")
	// Check if path is root => blank path
	if path == "/" {
		path = ""
	}
	return pa + path
}

func (a *goBlog) getInstanceRootURL() string {
	return a.getFullAddress("") + "/"
}
