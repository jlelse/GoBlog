package main

import (
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"go.goblog.app/app/pkgs/contenttype"
)

const micropubMediaSubPath = "/media"

func (a *goBlog) serveMicropubMedia(w http.ResponseWriter, r *http.Request) {
	if !strings.Contains(r.Context().Value(indieAuthScope).(string), "media") {
		a.serveError(w, r, "media scope missing", http.StatusForbidden)
		return
	}
	if ct := r.Header.Get(contentType); !strings.Contains(ct, contenttype.MultipartForm) {
		a.serveError(w, r, "wrong content-type", http.StatusBadRequest)
		return
	}
	err := r.ParseMultipartForm(0)
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusBadRequest)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusBadRequest)
		return
	}
	defer func() { _ = file.Close() }()
	hashFile, _, _ := r.FormFile("file")
	defer func() { _ = hashFile.Close() }()
	fileName, err := getSHA256(hashFile)
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	fileExtension := filepath.Ext(header.Filename)
	if len(fileExtension) == 0 {
		// Find correct file extension if original filename does not contain one
		mimeType := header.Header.Get(contentType)
		if len(mimeType) > 0 {
			allExtensions, _ := mime.ExtensionsByType(mimeType)
			if len(allExtensions) > 0 {
				fileExtension = allExtensions[0]
			}
		}
	}
	fileName += strings.ToLower(fileExtension)
	// Save file
	location, err := a.saveMediaFile(fileName, file)
	if err != nil {
		a.serveError(w, r, "failed to save original file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	// Try to compress file (only when not in private mode)
	if !a.isPrivate() {
		compressedLocation, compressionErr := a.compressMediaFile(location)
		if compressionErr != nil {
			a.serveError(w, r, "failed to compress file: "+compressionErr.Error(), http.StatusInternalServerError)
			return
		}
		// Overwrite location
		if compressedLocation != "" {
			location = compressedLocation
		}
	}
	http.Redirect(w, r, location, http.StatusCreated)
}
