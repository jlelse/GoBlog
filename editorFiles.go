package main

import (
	"net/http"
	"sort"

	"github.com/samber/lo"
)

func (a *goBlog) serveEditorFiles(w http.ResponseWriter, r *http.Request) {
	// Get files
	files, err := a.mediaFiles()
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	// Check if files at all
	if len(files) == 0 {
		a.render(w, r, a.renderEditorFiles, &renderData{
			Data: &editorFilesRenderData{},
		})
		return
	}
	// Sort files time desc
	sort.Slice(files, func(i, j int) bool {
		return files[i].Time.After(files[j].Time)
	})
	// Find uses
	fileNames := lo.Map(files, func(f *mediaFile, _ int) string {
		return f.Name
	})
	uses, err := a.db.usesOfMediaFile(fileNames...)
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	// Serve HTML
	a.render(w, r, a.renderEditorFiles, &renderData{
		Data: &editorFilesRenderData{
			files: files,
			uses:  uses,
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
	_, bc := a.getBlog(r)
	http.Redirect(w, r, bc.getRelativePath("/editor/files"), http.StatusFound)
}
