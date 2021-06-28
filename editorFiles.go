package main

import (
	"net/http"
	"sort"
)

func (a *goBlog) serveEditorFiles(w http.ResponseWriter, r *http.Request) {
	files, err := a.mediaFiles()
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	// Sort files time desc
	sort.Slice(files, func(i, j int) bool {
		return files[i].Time.After(files[j].Time)
	})
	// Serve HTML
	blog := r.Context().Value(blogContextKey).(string)
	a.render(w, r, templateEditorFiles, &renderData{
		BlogString: blog,
		Data: map[string]interface{}{
			"Files": files,
		},
	})
}

func (a *goBlog) serveEditorFilesView(w http.ResponseWriter, r *http.Request) {
	filename := r.FormValue("filename")
	if filename == "" {
		a.serveError(w, r, "No file selected", http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, a.mediaFileLocation(filename), http.StatusFound)
}

func (a *goBlog) serveEditorFilesDelete(w http.ResponseWriter, r *http.Request) {
	filename := r.FormValue("filename")
	if filename == "" {
		a.serveError(w, r, "No file selected", http.StatusBadRequest)
		return
	}
	if err := a.deleteMediaFile(filename); err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, a.getRelativePath(r.Context().Value(blogContextKey).(string), "/editor/files"), http.StatusFound)
}
