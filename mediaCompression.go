package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/carlmjohnson/requests"
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
	if a.cfg.Micropub == nil || a.cfg.Micropub.MediaStorage == nil {
		return
	}
	config := a.cfg.Micropub.MediaStorage
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

func (sp *shortpixel) compress(url string, upload mediaStorageSaveFunc, hc *http.Client) (string, error) {
	// Check url
	fileExtension, allowed := urlHasExt(url, "jpg", "jpeg", "png")
	if !allowed {
		return "", nil
	}
	// Compress
	var imgBuffer bytes.Buffer
	err := requests.
		URL("https://api.shortpixel.com/v2/reducer-sync.php").
		Client(hc).
		Method(http.MethodPost).
		BodyJSON(map[string]interface{}{
			"key":            sp.key,
			"plugin_version": "GB001",
			"lossy":          1,
			"resize":         3,
			"resize_width":   defaultCompressionWidth,
			"resize_height":  defaultCompressionHeight,
			"cmyk2rgb":       1,
			"keep_exif":      0,
			"url":            url,
		}).
		ToBytesBuffer(&imgBuffer).
		Fetch(context.Background())
	if err != nil {
		log.Println("Shortpixel error:", err.Error())
		return "", errors.New("failed to compress image using shortpixel")
	}
	// Upload compressed file
	return uploadCompressedFile(fileExtension, &imgBuffer, upload)
}

type tinify struct {
	key string
}

func (tf *tinify) compress(url string, upload mediaStorageSaveFunc, hc *http.Client) (string, error) {
	// Check url
	fileExtension, allowed := urlHasExt(url, "jpg", "jpeg", "png")
	if !allowed {
		return "", nil
	}
	// Compress
	compressedLocation := ""
	err := requests.
		URL("https://api.tinify.com/shrink").
		Client(hc).
		Method(http.MethodPost).
		BasicAuth("api", tf.key).
		BodyJSON(map[string]interface{}{
			"source": map[string]interface{}{
				"url": url,
			},
		}).
		Handle(func(r *http.Response) error {
			compressedLocation = r.Header.Get("Location")
			if compressedLocation == "" {
				return errors.New("location header missing")
			}
			return nil
		}).
		Fetch(context.Background())
	if err != nil {
		log.Println("Tinify error:", err.Error())
		return "", errors.New("failed to compress image using tinify")
	}
	// Resize and download image
	var imgBuffer bytes.Buffer
	err = requests.
		URL(compressedLocation).
		Client(hc).
		Method(http.MethodPost).
		BasicAuth("api", tf.key).
		BodyJSON(map[string]interface{}{
			"resize": map[string]interface{}{
				"method": "fit",
				"width":  defaultCompressionWidth,
				"height": defaultCompressionHeight,
			},
		}).
		ToBytesBuffer(&imgBuffer).
		Fetch(context.Background())
	if err != nil {
		log.Println("Tinify error:", err.Error())
		return "", errors.New("failed to compress image using tinify")
	}
	// Upload compressed file
	return uploadCompressedFile(fileExtension, &imgBuffer, upload)
}

type cloudflare struct{}

func (cf *cloudflare) compress(url string, upload mediaStorageSaveFunc, hc *http.Client) (string, error) {
	// Check url
	if _, allowed := urlHasExt(url, "jpg", "jpeg", "png"); !allowed {
		return "", nil
	}
	// Force jpeg
	fileExtension := "jpeg"
	// Compress
	var imgBuffer bytes.Buffer
	err := requests.
		URL(fmt.Sprintf("https://www.cloudflare.com/cdn-cgi/image/f=jpeg,q=75,metadata=none,fit=scale-down,w=%d,h=%d/%s", defaultCompressionWidth, defaultCompressionHeight, url)).
		Client(hc).
		ToBytesBuffer(&imgBuffer).
		Fetch(context.Background())
	if err != nil {
		log.Println("Cloudflare error:", err.Error())
		return "", errors.New("failed to compress image using cloudflare")
	}
	// Upload compressed file
	return uploadCompressedFile(fileExtension, &imgBuffer, upload)
}

func uploadCompressedFile(fileExtension string, imgBuffer *bytes.Buffer, upload mediaStorageSaveFunc) (string, error) {
	// Create reader from buffer
	imgReader := bytes.NewReader(imgBuffer.Bytes())
	// Get hash of compressed file
	fileName, err := getSHA256(imgReader)
	if err != nil {
		return "", err
	}
	// Upload compressed file
	return upload(fileName+"."+fileExtension, imgReader)
}
