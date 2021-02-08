package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tfgo "codeberg.org/jlelse/tinify"
)

const micropubMediaSubPath = "/media"

func serveMicropubMedia(w http.ResponseWriter, r *http.Request) {
	if !strings.Contains(r.Context().Value(indieAuthScope).(string), "media") {
		serveError(w, r, "media scope missing", http.StatusForbidden)
		return
	}
	if appConfig.Micropub.MediaStorage == nil {
		serveError(w, r, "Not configured", http.StatusNotImplemented)
		return
	}
	if ct := r.Header.Get(contentType); !strings.Contains(ct, contentTypeMultipartForm) {
		serveError(w, r, "wrong content-type", http.StatusBadRequest)
		return
	}
	err := r.ParseMultipartForm(0)
	if err != nil {
		serveError(w, r, err.Error(), http.StatusBadRequest)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		serveError(w, r, err.Error(), http.StatusBadRequest)
		return
	}
	defer func() { _ = file.Close() }()
	hashFile, _, _ := r.FormFile("file")
	defer func() { _ = hashFile.Close() }()
	fileName, err := getSHA256(hashFile)
	if err != nil {
		serveError(w, r, err.Error(), http.StatusInternalServerError)
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
	location, err := uploadFile(fileName, file)
	if err != nil {
		serveError(w, r, "failed to save original file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	// Try to compress file
	if ms := appConfig.Micropub.MediaStorage; ms != nil && ms.TinifyKey != "" {
		compressedLocation, err := tinify(location, ms)
		if err != nil {
			serveError(w, r, "failed to compress file: "+err.Error(), http.StatusInternalServerError)
			return
		} else if compressedLocation != "" {
			location = compressedLocation
		} else {
			serveError(w, r, "No compressed location", http.StatusInternalServerError)
		}
	}
	http.Redirect(w, r, location, http.StatusCreated)
}

func uploadFile(filename string, f io.Reader) (string, error) {
	ms := appConfig.Micropub.MediaStorage
	if ms != nil && ms.BunnyStorageKey != "" && ms.BunnyStorageName != "" {
		return uploadToBunny(filename, f, ms)
	}
	loc, err := saveMediaFile(filename, f)
	if err != nil {
		return "", err
	}
	if ms != nil && ms.MediaURL != "" {
		return ms.MediaURL + loc, nil
	}
	return loc, nil
}

func uploadToBunny(filename string, f io.Reader, config *configMicropubMedia) (location string, err error) {
	req, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("https://storage.bunnycdn.com/%s/%s", url.PathEscape(config.BunnyStorageName), url.PathEscape(filename)), f)
	req.Header.Add("AccessKey", config.BunnyStorageKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusCreated {
		return "", errors.New("failed to upload file to BunnyCDN")
	}
	return config.MediaURL + "/" + filename, nil
}

func tinify(url string, config *configMicropubMedia) (location string, err error) {
	fileExtension := func() string {
		spliced := strings.Split(url, ".")
		return spliced[len(spliced)-1]
	}()
	supportedTypes := []string{"jpg", "jpeg", "png"}
	sort.Strings(supportedTypes)
	i := sort.SearchStrings(supportedTypes, strings.ToLower(fileExtension))
	if !(i < len(supportedTypes) && supportedTypes[i] == strings.ToLower(fileExtension)) {
		return "", nil
	}
	tfgo.SetKey(config.TinifyKey)
	s, err := tfgo.FromUrl(url)
	if err != nil {
		return "", err
	}
	err = s.Resize(&tfgo.ResizeOption{
		Method: tfgo.ResizeMethodScale,
		Width:  2000,
	})
	if err != nil {
		return "", err
	}
	file, err := ioutil.TempFile("", "tiny-*."+fileExtension)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = file.Close()
		_ = os.Remove(file.Name())
	}()
	err = s.ToFile(file.Name())
	if err != nil {
		return "", err
	}
	hashFile, err := os.Open(file.Name())
	defer func() { _ = hashFile.Close() }()
	if err != nil {
		return "", err
	}
	fileName, err := getSHA256(hashFile)
	if err != nil {
		return "", err
	}
	location, err = uploadFile(fileName+"."+fileExtension, file)
	return
}

func getSHA256(file multipart.File) (filename string, err error) {
	h := sha256.New()
	if _, err = io.Copy(h, file); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
