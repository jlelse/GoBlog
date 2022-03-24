package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"

	"github.com/carlmjohnson/requests"
	"github.com/davidbyttow/govips/v2/vips"
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
	imgBuffer := bufferpool.Get()
	defer bufferpool.Put(imgBuffer)
	err = requests.
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
		ToBytesBuffer(imgBuffer).
		Fetch(context.Background())
	if err != nil {
		log.Println("Tinify error:", err.Error())
		return "", tinifyErr
	}
	// Upload compressed file
	return uploadCompressedFile(fileExtension, imgBuffer, upload)
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
	imgBuffer := bufferpool.Get()
	defer bufferpool.Put(imgBuffer)
	err := requests.
		URL(fmt.Sprintf("https://www.cloudflare.com/cdn-cgi/image/f=jpeg,q=75,metadata=none,fit=scale-down,w=%d,h=%d/%s", defaultCompressionWidth, defaultCompressionHeight, url)).
		Client(hc).
		ToBytesBuffer(imgBuffer).
		Fetch(context.Background())
	if err != nil {
		log.Println("Cloudflare error:", err.Error())
		return "", errors.New("failed to compress image using cloudflare")
	}
	// Upload compressed file
	return uploadCompressedFile(fileExtension, imgBuffer, upload)
}

type localMediaCompressor struct{}

func (*localMediaCompressor) compress(url string, upload mediaStorageSaveFunc, hc *http.Client) (string, error) {
	// Check url
	fileExtension, allowed := urlHasExt(url, "jpg", "jpeg", "png")
	if !allowed {
		return "", nil
	}
	// Download image
	imgBuffer := bufferpool.Get()
	defer bufferpool.Put(imgBuffer)
	err := requests.
		URL(url).
		Client(hc).
		ToBytesBuffer(imgBuffer).
		Fetch(context.Background())
	if err != nil {
		log.Println("Local compressor error:", err.Error())
		return "", errors.New("failed to download image using local compressor")
	}
	// Compress image using bimg
	vips.Startup(&vips.Config{
		CollectStats:     false,
		MaxCacheFiles:    0,
		MaxCacheMem:      0,
		MaxCacheSize:     0,
		ConcurrencyLevel: 1,
	})
	img, err := vips.NewImageFromReader(imgBuffer)
	if err != nil {
		log.Println("Local compressor error:", err.Error())
		return "", errors.New("failed to compress image using local compressor")
	}
	// Auto rotate image
	err = img.AutoRotate()
	if err != nil {
		log.Println("Local compressor error:", err.Error())
		return "", errors.New("failed to compress image using local compressor")
	}
	// Get resize ratio to fit in max width and height
	ratioHeight := float64(defaultCompressionHeight) / float64(img.Height())
	ratioWidth := float64(defaultCompressionWidth) / float64(img.Width())
	ratio := math.Min(ratioHeight, ratioWidth)
	// Resize image (only downscale)
	if ratio < 1 {
		err = img.Resize(ratio, vips.KernelAuto)
		if err != nil {
			log.Println("Local compressor error:", err.Error())
			return "", errors.New("failed to compress image using local compressor")
		}
	}
	// Export resized image
	ep := vips.NewDefaultExportParams()
	ep.StripMetadata = true      // Strip metadata
	ep.Interlaced = true         // Progressive image
	ep.TrellisQuant = true       // Trellis quantization (JPEG only)
	ep.OptimizeCoding = true     // Optimize Huffman coding tables (JPEG only)
	ep.OptimizeScans = true      // Optimize progressive scans (JPEG only)
	ep.OvershootDeringing = true // Overshoot deringing (JPEG only)
	ep.Compression = 100         // Compression factor
	ep.Quality = 75              // Quality factor
	compressed, _, err := img.Export(ep)
	if err != nil {
		log.Println("Local compressor error:", err.Error())
		return "", errors.New("failed to compress image using local compressor")
	}
	// Upload compressed file
	return uploadCompressedFile(fileExtension, bytes.NewReader(compressed), upload)
}

func uploadCompressedFile(fileExtension string, r io.Reader, upload mediaStorageSaveFunc) (string, error) {
	// Copy file to temporary buffer to generate hash and filename
	hash := sha256.New()
	tempBuffer := bufferpool.Get()
	defer bufferpool.Put(tempBuffer)
	_, _ = io.Copy(io.MultiWriter(tempBuffer, hash), r)
	// Upload buffer
	return upload(fmt.Sprintf("%x.%s", hash.Sum(nil), fileExtension), tempBuffer)
}
