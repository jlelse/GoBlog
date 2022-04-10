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
		switch path.Ext(fileName) {
		case ".js":
			w.Header().Set(contentType, contenttype.JSUTF8)
			_ = a.min.Get().Minify(contenttype.JS, w, file)
		case ".css":
			w.Header().Set(contentType, contenttype.CSSUTF8)
			_ = a.min.Get().Minify(contenttype.CSS, w, file)
		default:
			_, _ = io.Copy(w, file)
		}
	}
}
