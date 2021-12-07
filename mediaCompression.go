package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"go.goblog.app/app/pkgs/contenttype"
)

const defaultCompressionWidth = 2000
const defaultCompressionHeight = 3000

type mediaCompression interface {
	compress(url string, save mediaStorageSaveFunc, hc *http.Client) (location string, err error)
}

func (a *goBlog) compressMediaFile(url string) (location string, err error) {
	// Init compressors
	a.compressorsInit.Do(a.initMediaCompressors)
	// Try all compressors until success
	for _, c := range a.compressors {
		location, err = c.compress(url, a.saveMediaFile, a.httpClient)
		if location != "" && err == nil {
			break
		}
	}
	// Return result
	return location, err
}

func (a *goBlog) initMediaCompressors() {
	config := a.cfg.Micropub.MediaStorage
	if config == nil {
		return
	}
	if key := config.ShortPixelKey; key != "" {
		a.compressors = append(a.compressors, &shortpixel{key})
	}
	if key := config.TinifyKey; key != "" {
		a.compressors = append(a.compressors, &tinify{key})
	}
	if config.CloudflareCompressionEnabled {
		a.compressors = append(a.compressors, &cloudflare{})
	}
}

type shortpixel struct {
	key string
}

var _ mediaCompression = &shortpixel{}

func (sp *shortpixel) compress(url string, upload mediaStorageSaveFunc, hc *http.Client) (location string, err error) {
	// Check url
	fileExtension, allowed := urlHasExt(url, "jpg", "jpeg", "png")
	if !allowed {
		return "", nil
	}
	// Compress
	j, _ := json.Marshal(map[string]interface{}{
		"key":            sp.key,
		"plugin_version": "GB001",
		"lossy":          1,
		"resize":         3,
		"resize_width":   defaultCompressionWidth,
		"resize_height":  defaultCompressionHeight,
		"cmyk2rgb":       1,
		"keep_exif":      0,
		"url":            url,
	})
	req, err := http.NewRequest(http.MethodPut, "https://api.shortpixel.com/v2/reducer-sync.php", bytes.NewReader(j))
	if err != nil {
		return "", err
	}
	resp, err := hc.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("shortpixel failed to compress image, status code %d", resp.StatusCode)
	}
	tmpFile, err := os.CreateTemp("", "tiny-*."+fileExtension)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
	}()
	if _, err = io.Copy(tmpFile, resp.Body); err != nil {
		return "", err
	}
	fileName, err := getSHA256(tmpFile)
	if err != nil {
		return "", err
	}
	// Upload compressed file
	location, err = upload(fileName+"."+fileExtension, tmpFile)
	return
}

type tinify struct {
	key string
}

var _ mediaCompression = &tinify{}

func (tf *tinify) compress(url string, upload mediaStorageSaveFunc, hc *http.Client) (location string, err error) {
	// Check url
	fileExtension, allowed := urlHasExt(url, "jpg", "jpeg", "png")
	if !allowed {
		return "", nil
	}
	// Compress
	j, _ := json.Marshal(map[string]interface{}{
		"source": map[string]interface{}{
			"url": url,
		},
	})
	req, err := http.NewRequest(http.MethodPost, "https://api.tinify.com/shrink", bytes.NewReader(j))
	if err != nil {
		return "", err
	}
	req.SetBasicAuth("api", tf.key)
	req.Header.Set(contentType, contenttype.JSON)
	resp, err := hc.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("failed to compress image, status code %d", resp.StatusCode)
	}
	compressedLocation := resp.Header.Get("Location")
	if compressedLocation == "" {
		return "", errors.New("tinify didn't return compressed location")
	}
	// Resize and download image
	j, _ = json.Marshal(map[string]interface{}{
		"resize": map[string]interface{}{
			"method": "fit",
			"width":  defaultCompressionWidth,
			"height": defaultCompressionHeight,
		},
	})
	downloadReq, err := http.NewRequest(http.MethodPost, compressedLocation, bytes.NewReader(j))
	if err != nil {
		return "", err
	}
	downloadReq.SetBasicAuth("api", tf.key)
	downloadReq.Header.Set(contentType, contenttype.JSON)
	downloadResp, err := hc.Do(downloadReq)
	if err != nil {
		return "", err
	}
	defer downloadResp.Body.Close()
	if downloadResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("tinify failed to resize image, status code %d", downloadResp.StatusCode)
	}
	tmpFile, err := os.CreateTemp("", "tiny-*."+fileExtension)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
	}()
	if _, err = io.Copy(tmpFile, downloadResp.Body); err != nil {
		return "", err
	}
	fileName, err := getSHA256(tmpFile)
	if err != nil {
		return "", err
	}
	// Upload compressed file
	location, err = upload(fileName+"."+fileExtension, tmpFile)
	return
}

type cloudflare struct {
}

var _ mediaCompression = &cloudflare{}

func (cf *cloudflare) compress(url string, upload mediaStorageSaveFunc, hc *http.Client) (location string, err error) {
	// Check url
	_, allowed := urlHasExt(url, "jpg", "jpeg", "png")
	if !allowed {
		return "", nil
	}
	// Force jpeg
	fileExtension := "jpeg"
	// Compress
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://www.cloudflare.com/cdn-cgi/image/f=jpeg,q=75,metadata=none,fit=scale-down,w=%d,h=%d/%s", defaultCompressionWidth, defaultCompressionHeight, url), nil)
	if err != nil {
		return "", err
	}
	resp, err := hc.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("cloudflare failed to compress image, status code %d", resp.StatusCode)
	}
	tmpFile, err := os.CreateTemp("", "tiny-*."+fileExtension)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
	}()
	if _, err = io.Copy(tmpFile, resp.Body); err != nil {
		return "", err
	}
	fileName, err := getSHA256(tmpFile)
	if err != nil {
		return "", err
	}
	// Upload compressed file
	location, err = upload(fileName+"."+fileExtension, tmpFile)
	return
}
