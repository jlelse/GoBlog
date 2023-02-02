package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"image/png"
	"io"
	"log"
	"net/http"

	"github.com/carlmjohnson/requests"
	"github.com/disintegration/imaging"
	"go.goblog.app/app/pkgs/bufferpool"
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
	if key := config.TinifyKey; key != "" {
		a.compressors = append(a.compressors, &tinify{key})
	}
	if config.CloudflareCompressionEnabled {
		a.compressors = append(a.compressors, &cloudflare{})
	}
	if config.LocalCompressionEnabled {
		a.compressors = append(a.compressors, &localMediaCompressor{})
	}
}

type tinify struct {
	key string
}

func (tf *tinify) compress(url string, upload mediaStorageSaveFunc, hc *http.Client) (string, error) {
	tinifyErr := errors.New("failed to compress image using tinify")
	// Check url
	fileExtension, allowed := urlHasExt(url, "jpg", "jpeg", "png")
	if !allowed {
		return "", nil
	}
	// Compress
	headers := http.Header{}
	err := requests.
		URL("https://api.tinify.com/shrink").
		Client(hc).
		Method(http.MethodPost).
		BasicAuth("api", tf.key).
		BodyJSON(map[string]any{
			"source": map[string]any{
				"url": url,
			},
		}).
		ToHeaders(headers).
		Fetch(context.Background())
	if err != nil {
		log.Println("Tinify error:", err.Error())
		return "", tinifyErr
	}
	compressedLocation := headers.Get("Location")
	if compressedLocation == "" {
		log.Println("Tinify error: location header missing")
		return "", tinifyErr
	}
	// Resize and download image
	pr, pw := io.Pipe()
	go func() {
		_ = pw.CloseWithError(requests.
			URL(compressedLocation).
			Client(hc).
			Method(http.MethodPost).
			BasicAuth("api", tf.key).
			BodyJSON(map[string]any{
				"resize": map[string]any{
					"method": "fit",
					"width":  defaultCompressionWidth,
					"height": defaultCompressionHeight,
				},
			}).
			ToWriter(pw).
			Fetch(context.Background()))
	}()
	// Upload compressed file
	res, err := uploadCompressedFile(fileExtension, pr, upload)
	_ = pr.CloseWithError(err)
	return res, err
}

type cloudflare struct{}

func (*cloudflare) compress(url string, upload mediaStorageSaveFunc, hc *http.Client) (string, error) {
	// Check url
	if _, allowed := urlHasExt(url, "jpg", "jpeg", "png"); !allowed {
		return "", nil
	}
	// Force jpeg
	fileExtension := "jpeg"
	// Compress
	pr, pw := io.Pipe()
	go func() {
		_ = pw.CloseWithError(requests.
			URL(fmt.Sprintf("https://www.cloudflare.com/cdn-cgi/image/f=jpeg,q=75,metadata=none,fit=scale-down,w=%d,h=%d/%s", defaultCompressionWidth, defaultCompressionHeight, url)).
			Client(hc).
			ToWriter(pw).
			Fetch(context.Background()))
	}()
	// Upload compressed file
	res, err := uploadCompressedFile(fileExtension, pr, upload)
	_ = pr.CloseWithError(err)
	return res, err
}

type localMediaCompressor struct{}

func (*localMediaCompressor) compress(url string, upload mediaStorageSaveFunc, hc *http.Client) (string, error) {
	// Check url
	fileExtension, allowed := urlHasExt(url, "jpg", "jpeg", "png")
	if !allowed {
		return "", nil
	}
	// Download and decode image
	pr, pw := io.Pipe()
	go func() {
		_ = pw.CloseWithError(requests.URL(url).Client(hc).ToWriter(pw).Fetch(context.Background()))
	}()
	img, err := imaging.Decode(pr, imaging.AutoOrientation(true))
	_ = pr.CloseWithError(err)
	if err != nil {
		log.Println("Local compressor error:", err.Error())
		return "", errors.New("failed to compress image using local compressor")
	}
	// Resize image
	resizedImage := imaging.Fit(img, defaultCompressionWidth, defaultCompressionHeight, imaging.Lanczos)
	// Encode image
	pr, pw = io.Pipe()
	go func() {
		switch fileExtension {
		case "png":
			_ = pw.CloseWithError(imaging.Encode(pw, resizedImage, imaging.PNG, imaging.PNGCompressionLevel(png.BestCompression)))
		default:
			_ = pw.CloseWithError(imaging.Encode(pw, resizedImage, imaging.JPEG, imaging.JPEGQuality(75)))
		}
	}()
	// Upload compressed file
	res, err := uploadCompressedFile(fileExtension, pr, upload)
	_ = pr.CloseWithError(err)
	return res, err
}

func uploadCompressedFile(fileExtension string, r io.Reader, upload mediaStorageSaveFunc) (string, error) {
	// Copy file to temporary buffer to generate hash and filename
	hash := sha256.New()
	tempBuffer := bufferpool.Get()
	defer bufferpool.Put(tempBuffer)
	_, err := io.Copy(io.MultiWriter(tempBuffer, hash), r)
	if err != nil {
		return "", err
	}
	// Upload buffer
	return upload(fmt.Sprintf("%x.%s", hash.Sum(nil), fileExtension), tempBuffer)
}
