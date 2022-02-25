package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"go.goblog.app/app/pkgs/bufferpool"
	"go.goblog.app/app/pkgs/contenttype"
)

const micropubMediaSubPath = "/media"

func (a *goBlog) serveMicropubMedia(w http.ResponseWriter, r *http.Request) {
	// Check scope
	if !strings.Contains(r.Context().Value(indieAuthScope).(string), "media") {
		a.serveError(w, r, "media scope missing", http.StatusForbidden)
		return
	}
	// Check if request is multipart
	if ct := r.Header.Get(contentType); !strings.Contains(ct, contenttype.MultipartForm) {
		a.serveError(w, r, "wrong content-type", http.StatusBadRequest)
		return
	}
	// Parse multipart form
	err := r.ParseMultipartForm(0)
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusBadRequest)
		return
	}
	// Get file
	file, header, err := r.FormFile("file")
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusBadRequest)
		return
	}
	// Read the file into temporary buffer and generate sha256 hash
	hash := sha256.New()
	buffer := bufferpool.Get()
	defer bufferpool.Put(buffer)
	_, _ = io.Copy(buffer, io.TeeReader(file, hash))
	_ = file.Close()
	_ = r.Body.Close()
	// Get file extension
	fileExtension := filepath.Ext(header.Filename)
	if fileExtension == "" {
		// Find correct file extension if original filename does not contain one
		mimeType := header.Header.Get(contentType)
		if len(mimeType) > 0 {
			allExtensions, _ := mime.ExtensionsByType(mimeType)
			if len(allExtensions) > 0 {
				fileExtension = allExtensions[0]
			}
		}
	}
	// Generate the file name
	fileName := fmt.Sprintf("%x%s", hash.Sum(nil), fileExtension)
	// Save file
	location, err := a.saveMediaFile(fileName, buffer)
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
