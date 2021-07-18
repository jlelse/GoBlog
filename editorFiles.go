package main

import (
	"net/http"
	"sort"

	"github.com/thoas/go-funk"
)

func (a *goBlog) serveEditorFiles(w http.ResponseWriter, r *http.Request) {
	blog := r.Context().Value(blogContextKey).(string)
	// Get files
	files, err := a.mediaFiles()
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	// Check if files at all
	if len(files) == 0 {
		a.render(w, r, templateEditorFiles, &renderData{
			BlogString: blog,
			Data:       map[string]interface{}{},
		})
		return
	}
	// Sort files time desc
	sort.Slice(files, func(i, j int) bool {
		return files[i].Time.After(files[j].Time)
	})
	// Find uses
	fileNames, ok := funk.Map(files, func(f *mediaFile) string {
		return f.Name
	}).([]string)
	if !ok {
		a.serveError(w, r, "Failed to get file names", http.StatusInternalServerError)
		return
	}
	uses, err := a.db.usesOfMediaFile(fileNames...)
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	// Serve HTML
	a.render(w, r, templateEditorFiles, &renderData{
		BlogString: blog,
		Data: map[string]interface{}{
			"Files": files,
			"Uses":  uses,
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
