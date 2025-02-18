package main

import (
	"context"
	"net/http"
	"sort"

	"github.com/go-chi/chi/v5"
)

const (
	editorFilesPath               = "/files"
	editorFileViewPath            = editorFilesPath + "/view"
	editorFileUsesPath            = editorFilesPath + "/uses"
	editorFileUsesPathPlaceholder = "/{filename}"
	editorFileDeletePath          = editorFilesPath + "/delete"
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
	// Serve HTML
	a.render(w, r, a.renderEditorFiles, &renderData{
		Data: &editorFilesRenderData{
			files: files,
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

func (a *goBlog) serveEditorFilesUses(w http.ResponseWriter, r *http.Request) {
	filename := r.FormValue("filename")
	if filename == "" {
		a.serveError(w, r, "No file selected", http.StatusBadRequest)
		return
	}
	_, bc := a.getBlog(r)
	http.Redirect(w, r, bc.getRelativePath(editorPath)+editorFileUsesPath+"/"+filename, http.StatusFound)
}

func (a *goBlog) serveEditorFilesUsesResults(w http.ResponseWriter, r *http.Request) {
	filename := chi.URLParam(r, "filename")
	if filename == "" {
		a.serveError(w, r, "No file selected", http.StatusBadRequest)
		return
	}
	_, bc := a.getBlog(r)
	a.serveIndex(w, r.WithContext(context.WithValue(r.Context(), indexConfigKey, &indexConfig{
		path:         bc.getRelativePath(editorPath + editorFileUsesPath + "/" + filename),
		usesFile:     filename,
		withoutFeeds: true,
	})))
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
	http.Redirect(w, r, bc.getRelativePath(editorPath+editorFilesPath), http.StatusFound)
}
