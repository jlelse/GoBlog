package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
)

const defaultCompressionWidth = 2000
const defaultCompressionHeight = 3000

func tinify(url string, config *configMicropubMedia) (location string, err error) {
	// Check config
	if config == nil || config.TinifyKey == "" {
		return "", errors.New("service Tinify not configured")
	}
	// Check url
	fileExtension, allowed := compressionIsSupported(url, "jpg", "jpeg", "png")
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
	req.SetBasicAuth("api", config.TinifyKey)
	req.Header.Set(contentType, contentTypeJSON)
	resp, err := appHttpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
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
	downloadReq.SetBasicAuth("api", config.TinifyKey)
	downloadReq.Header.Set(contentType, contentTypeJSON)
	downloadResp, err := appHttpClient.Do(downloadReq)
	if err != nil {
		return "", err
	}
	defer downloadResp.Body.Close()
	if downloadResp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, downloadResp.Body)
		return "", fmt.Errorf("tinify failed to resize image, status code %d", downloadResp.StatusCode)
	}
	tmpFile, err := os.CreateTemp("", "tiny-*."+fileExtension)
	if err != nil {
		_, _ = io.Copy(io.Discard, downloadResp.Body)
		return "", err
	}
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
	}()
	if _, err = io.Copy(tmpFile, downloadResp.Body); err != nil {
		_, _ = io.Copy(io.Discard, downloadResp.Body)
		return "", err
	}
	fileName, err := getSHA256(tmpFile)
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
		return "", errors.New("service ShortPixel not configured")
	}
	// Check url
	fileExtension, allowed := compressionIsSupported(url, "jpg", "jpeg", "png")
	if !allowed {
		return "", nil
	}
	// Compress
	j, _ := json.Marshal(map[string]interface{}{
		"key":            config.ShortPixelKey,
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
	resp, err := appHttpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return "", fmt.Errorf("shortpixel failed to compress image, status code %d", resp.StatusCode)
	}
	tmpFile, err := os.CreateTemp("", "tiny-*."+fileExtension)
	if err != nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return "", err
	}
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
	}()
	if _, err = io.Copy(tmpFile, resp.Body); err != nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return "", err
	}
	fileName, err := getSHA256(tmpFile)
	if err != nil {
		return "", err
	}
	// Upload compressed file
	location, err = uploadFile(fileName+"."+fileExtension, tmpFile)
	return
}
