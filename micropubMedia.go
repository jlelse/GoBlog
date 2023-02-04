package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"go.goblog.app/app/pkgs/contenttype"
)

const micropubMediaSubPath = "/media"

func (a *goBlog) serveMicropubMedia(w http.ResponseWriter, r *http.Request) {
	// Check scope
	if !a.micropubCheckScope(w, r, "media") {
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
		a.serveError(w, r, "failed to parse multipart form", http.StatusBadRequest)
		return
	}
	// Get file
	file, header, err := r.FormFile("file")
	if err != nil {
		a.serveError(w, r, "failed to get multipart file", http.StatusBadRequest)
		return
	}
	defer file.Close()
	// Generate sha256 hash for file
	hash := sha256.New()
	_, err = io.Copy(hash, file)
	if err != nil {
		a.serveError(w, r, "failed to get file hash", http.StatusBadRequest)
		return
	}
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
	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		a.serveError(w, r, "failed to read multipart file", http.StatusInternalServerError)
		return
	}
	location, err := a.saveMediaFile(fileName, file)
	if err != nil {
		a.serveError(w, r, "failed to save original file", http.StatusInternalServerError)
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
