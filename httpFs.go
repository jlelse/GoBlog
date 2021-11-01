package main

import (
	"embed"
	"net/http"
	"path"
	"strings"

	"go.goblog.app/app/pkgs/contenttype"
)

func (a *goBlog) serveFs(fs embed.FS, basePath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fileName := strings.TrimPrefix(r.URL.Path, basePath)
		fb, err := fs.ReadFile(fileName)
		if err != nil {
			a.serve404(w, r)
			return
		}
		switch path.Ext(fileName) {
		case ".js":
			w.Header().Set(contentType, contenttype.JS)
			_, _ = a.min.Write(w, contenttype.JSUTF8, fb)
		case ".css":
			w.Header().Set(contentType, contenttype.CSS)
			_, _ = a.min.Write(w, contenttype.CSSUTF8, fb)
		default:
			w.Header().Set(contentType, http.DetectContentType(fb))
			_, _ = w.Write(fb)
		}
	}
}
