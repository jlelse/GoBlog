package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
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
	if ms := appConfig.Micropub.MediaStorage; ms != nil {
		serveCompressionError := func(ce error) {
			serveError(w, r, "failed to compress file: "+ce.Error(), http.StatusInternalServerError)
		}
		var compressedLocation string
		var compressionErr error
		// Default ShortPixel
		if ms.ShortPixelKey != "" {
			compressedLocation, compressionErr = shortPixel(location, ms)
		}
		if compressionErr != nil {
			serveCompressionError(compressionErr)
			return
		}
		// Fallback Tinify
		if compressedLocation == "" && ms.TinifyKey != "" {
			compressedLocation, compressionErr = tinify(location, ms)
		}
		if compressionErr != nil {
			serveCompressionError(compressionErr)
			return
		}
		if compressedLocation != "" {
			location = compressedLocation
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
	return appConfig.Server.PublicAddress + loc, nil
}

func uploadToBunny(filename string, f io.Reader, config *configMicropubMedia) (location string, err error) {
	if config == nil || config.BunnyStorageName == "" || config.BunnyStorageKey == "" || config.MediaURL == "" {
		return "", errors.New("Bunny storage not completely configured")
	}
	req, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("https://storage.bunnycdn.com/%s/%s", url.PathEscape(config.BunnyStorageName), url.PathEscape(filename)), f)
	req.Header.Add("AccessKey", config.BunnyStorageKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusCreated {
		return "", errors.New("failed to upload file to BunnyCDN")
	}
	return config.MediaURL + "/" + filename, nil
}

func tinify(url string, config *configMicropubMedia) (location string, err error) {
	// Check config
	if config == nil || config.TinifyKey == "" {
		return "", errors.New("Tinify not configured")
	}
	// Check url
	fileExtension, allowed := compressionIsSupported(url, "jpg", "jpeg", "png")
	if !allowed {
		return "", nil
	}
	// Compress
	tfgo.SetKey(config.TinifyKey)
	s, err := tfgo.FromUrl(url)
	if err != nil {
		return "", err
	}
	if err = s.Resize(&tfgo.ResizeOption{
		Method: tfgo.ResizeMethodScale,
		Width:  2000,
	}); err != nil {
		return "", err
	}
	tmpFile, err := ioutil.TempFile("", "tiny-*."+fileExtension)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
	}()
	if err = s.ToFile(tmpFile.Name()); err != nil {
		return "", err
	}
	fileName, err := hashFile(tmpFile.Name())
	if err != nil {
		return "", err
	}
	// Upload compressed file
	location, err = uploadFile(fileName+"."+fileExtension, tmpFile)
	return
}

func shortPixel(url string, config *configMicropubMedia) (location string, err error) {
	// Check config
	if config == nil || config.ShortPixelKey == "" {
		return "", errors.New("ShortPixel not configured")
	}
	// Check url
	fileExtension, allowed := compressionIsSupported(url, "jpg", "jpeg", "png")
	if !allowed {
		return "", nil
	}
	// Compress
	var buf bytes.Buffer
	_ = json.NewEncoder(&buf).Encode(map[string]interface{}{
		"key":            config.ShortPixelKey,
		"plugin_version": "GB001",
		"lossy":          1,
		"resize":         3,
		"resize_width":   2000,
		"resize_height":  3000,
		"cmyk2rgb":       1,
		"keep_exif":      0,
		"url":            url,
	})
	req, err := http.NewRequest(http.MethodPut, "https://api.shortpixel.com/v2/reducer-sync.php", &buf)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	} else if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to compress image, status code %d", resp.StatusCode)
	}
	tmpFile, err := ioutil.TempFile("", "tiny-*."+fileExtension)
	if err != nil {
		return "", err
	}
	tmpFileName := tmpFile.Name()
	defer func() {
		_ = resp.Body.Close()
		_ = tmpFile.Close()
		_ = os.Remove(tmpFileName)
	}()
	if _, err = io.Copy(tmpFile, resp.Body); err != nil {
		return "", err
	}
	fileName, err := hashFile(tmpFileName)
	if err != nil {
		return "", err
	}
	// Reopen tmp file
	_ = tmpFile.Close()
	tmpFile, err = os.Open(tmpFileName)
	if err != nil {
		return "", err
	}
	// Upload compressed file
	location, err = uploadFile(fileName+"."+fileExtension, tmpFile)
	return
}

func compressionIsSupported(url string, allowed ...string) (string, bool) {
	spliced := strings.Split(url, ".")
	ext := spliced[len(spliced)-1]
	sort.Strings(allowed)
	if i := sort.SearchStrings(allowed, strings.ToLower(ext)); i >= len(allowed) || allowed[i] != strings.ToLower(ext) {
		return ext, false
	}
	return ext, true
}

func getSHA256(file multipart.File) (filename string, err error) {
	h := sha256.New()
	if _, err = io.Copy(h, file); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func hashFile(filename string) (string, error) {
	hashFile, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = hashFile.Close()
	}()
	fn, err := getSHA256(hashFile)
	if err != nil {
		return "", err
	}
	return fn, nil
}
