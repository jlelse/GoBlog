package main

import (
	"context"
	"net/http"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
)

const (
	editorFilesPath               = "/files"
	editorFileViewPath            = editorFilesPath + "/view"
	editorFileUsesPath            = editorFilesPath + "/uses"
	editorFileUsesPathPlaceholder = "/{filename}"
	editorFileDeletePath          = editorFilesPath + "/delete"
	editorFileOptimizePath        = editorFilesPath + "/optimize"
)

func (a *goBlog) serveEditorFiles(w http.ResponseWriter, r *http.Request) {
	// Get files
	files, err := a.mediaFiles()
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}

	// Query optimized variant and original hashes
	optimizedVariants := map[string]bool{}
	originals := map[string]bool{}
	if a.mediaOptimizationEnabled() {
		if o, v, err := a.db.mediaOptimizedHashSets(); err == nil {
			originals = o
			optimizedVariants = v
		}
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
			files:             files,
			optimizedVariants: optimizedVariants,
			originals:         originals,
		},
	})
}

func (a *goBlog) serveEditorFilesView(w http.ResponseWriter, r *http.Request) {
	filename := r.FormValue("filename") //nolint:gosec
	if filename == "" {
		a.serveError(w, r, "No file selected", http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, a.mediaFileLocation(filename), http.StatusFound)
}

func (a *goBlog) serveEditorFilesUses(w http.ResponseWriter, r *http.Request) {
	filename := r.FormValue("filename") //nolint:gosec
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
		allBlogs:     true,
	})))
}

func (a *goBlog) serveEditorFilesDelete(w http.ResponseWriter, r *http.Request) {
	filename := r.FormValue("filename") //nolint:gosec
	if filename == "" {
		a.serveError(w, r, "No file selected", http.StatusBadRequest)
		return
	}
	if err := a.deleteMediaFile(filename); err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	a.purgeCache()
	_, bc := a.getBlog(r)
	http.Redirect(w, r, bc.getRelativePath(editorPath+editorFilesPath), http.StatusFound)
}

func (a *goBlog) serveEditorFilesOptimize(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	filename := r.FormValue("filename") //nolint:gosec
	if filename == "" {
		a.serveError(w, r, "No file selected", http.StatusBadRequest)
		return
	}
	ext := filepath.Ext(filename)
	hash := strings.TrimSuffix(filename, ext)
	a.optimizeMediaFile(hash, ext)
	a.purgeCache()
	_, bc := a.getBlog(r)
	http.Redirect(w, r, bc.getRelativePath(editorPath+editorFilesPath), http.StatusFound)
}
