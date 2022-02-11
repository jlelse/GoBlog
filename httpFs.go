package main

import (
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"go.goblog.app/app/pkgs/contenttype"
)

func (a *goBlog) serveFs(f fs.FS, basePath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fileName := strings.TrimPrefix(r.URL.Path, basePath)
		file, err := f.Open(fileName)
		if err != nil {
			a.serve404(w, r)
			return
		}
		var read io.Reader = file
		switch path.Ext(fileName) {
		case ".js":
			w.Header().Set(contentType, contenttype.JSUTF8)
			read = a.min.Reader(contenttype.JS, read)
		case ".css":
			w.Header().Set(contentType, contenttype.CSSUTF8)
			read = a.min.Reader(contenttype.CSS, read)
		}
		_, _ = io.Copy(w, read)
	}
}
