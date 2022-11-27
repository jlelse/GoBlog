package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	_ "embed"

	"github.com/disintegration/imaging"
	"go.goblog.app/app/pkgs/bufferpool"
	"go.goblog.app/app/pkgs/contenttype"
)

type profileImageFormat string

const (
	profileImageFormatPNG  profileImageFormat = "png"
	profileImageFormatJPEG profileImageFormat = "jpg"

	profileImagePath             = "/profile"
	profileImagePathJPEG         = profileImagePath + "." + string(profileImageFormatJPEG)
	profileImagePathPNG          = profileImagePath + "." + string(profileImageFormatPNG)
	profileImageSizeRegexPattern = `(?P<width>\d+)(x(?P<height>\d+))?`
	profileImageCacheName        = "profileImage"
	profileImageHashCacheName    = "profileImageHash"

	settingsUpdateProfileImagePath = "/updateprofileimage"
	settingsDeleteProfileImagePath = "/deleteprofileimage"
)

//go:embed logo/GoBlog.png
var defaultLogo []byte

func (a *goBlog) serveProfileImage(format profileImageFormat) http.HandlerFunc {
	var mediaType string
	var encode func(output io.Writer, img *image.NRGBA, quality int) error
	switch format {
	case profileImageFormatPNG:
		mediaType = "image/png"
		encode = func(output io.Writer, img *image.NRGBA, quality int) error {
			return imaging.Encode(output, img, imaging.PNG, imaging.PNGCompressionLevel(png.BestCompression))
		}
	default:
		mediaType = "image/jpeg"
		encode = func(output io.Writer, img *image.NRGBA, quality int) error {
			return imaging.Encode(output, img, imaging.JPEG, imaging.JPEGQuality(quality))
		}
	}
	return func(w http.ResponseWriter, r *http.Request) {
		// Get requested size
		width, height := 0, 0
		sizeFormValue := r.FormValue("s")
		re := regexp.MustCompile(profileImageSizeRegexPattern)
		if re.MatchString(sizeFormValue) {
			matches := re.FindStringSubmatch(sizeFormValue)
			widthIndex := re.SubexpIndex("width")
			if widthIndex != -1 {
				width, _ = strconv.Atoi(matches[widthIndex])
			}
			heightIndex := re.SubexpIndex("height")
			if heightIndex != -1 {
				height, _ = strconv.Atoi(matches[heightIndex])
			}
		}
		if width == 0 || width > 512 {
			width = 512
		}
		if height == 0 || height > 512 {
			height = width
		}
		// Get requested quality
		quality := 0
		qualityFormValue := r.FormValue("q")
		if qualityFormValue != "" {
			quality, _ = strconv.Atoi(qualityFormValue)
		}
		if quality == 0 || quality > 100 {
			quality = 75
		}
		// Read from database
		var imageBytes []byte
		if a.hasProfileImage() {
			var err error
			imageBytes, err = a.db.retrievePersistentCacheContext(r.Context(), profileImageCacheName)
			if err != nil || imageBytes == nil {
				a.serveError(w, r, "Failed to retrieve image", http.StatusInternalServerError)
				return
			}
		} else {
			imageBytes = defaultLogo
		}
		// Decode image
		img, err := imaging.Decode(bytes.NewReader(imageBytes), imaging.AutoOrientation(true))
		if err != nil {
			a.serveError(w, r, "Failed to decode image", http.StatusInternalServerError)
			return
		}
		// Resize image
		resizedImage := imaging.Fit(img, width, height, imaging.Lanczos)
		// Encode
		resizedBuffer := bufferpool.Get()
		defer bufferpool.Put(resizedBuffer)
		err = encode(resizedBuffer, resizedImage, quality)
		if err != nil {
			a.serveError(w, r, "Failed to encode image", http.StatusInternalServerError)
			return
		}
		// Return
		w.Header().Set(contentType, mediaType)
		_, _ = io.Copy(w, resizedBuffer)
	}
}

func (a *goBlog) profileImagePath(format profileImageFormat, size, quality int) string {
	if !a.hasProfileImage() {
		return string(profileImagePathJPEG)
	}
	query := url.Values{}
	hashBytes, _ := a.db.retrievePersistentCache(profileImageHashCacheName)
	query.Set("v", string(hashBytes))
	if quality != 0 {
		query.Set("q", fmt.Sprintf("%d", quality))
	}
	if size != 0 {
		query.Set("s", fmt.Sprintf("%d", size))
	}
	return fmt.Sprintf("%s.%s?%s", profileImagePath, format, query.Encode())
}

func (a *goBlog) hasProfileImage() bool {
	a.hasProfileImageInit.Do(func() {
		a.hasProfileImageBool = a.db.hasPersistantCache(profileImageHashCacheName) && a.db.hasPersistantCache(profileImageCacheName)
	})
	return a.hasProfileImageBool
}

func (a *goBlog) serveUpdateProfileImage(w http.ResponseWriter, r *http.Request) {
	// Check if request is multipart
	if ct := r.Header.Get(contentType); !strings.Contains(ct, contenttype.MultipartForm) {
		a.serveError(w, r, "wrong content-type", http.StatusBadRequest)
		return
	}
	// Parse multipart form
	err := r.ParseMultipartForm(0)
	if err != nil {
		a.serveError(w, r, "Failed to parse multipart form", http.StatusBadRequest)
		return
	}
	// Get file
	file, _, err := r.FormFile("file")
	if err != nil {
		a.serveError(w, r, "Failed to read file", http.StatusBadRequest)
		return
	}
	// Read the file into temporary buffer and generate sha256 hash
	hash := sha256.New()
	buffer := bufferpool.Get()
	defer bufferpool.Put(buffer)
	_, _ = io.Copy(io.MultiWriter(buffer, hash), file)
	_ = file.Close()
	_ = r.Body.Close()
	// Cache
	err = a.db.cachePersistentlyContext(r.Context(), profileImageHashCacheName, []byte(fmt.Sprintf("%x", hash.Sum(nil))))
	if err != nil {
		a.serveError(w, r, "Failed to persist hash", http.StatusBadRequest)
		return
	}
	err = a.db.cachePersistentlyContext(r.Context(), profileImageCacheName, buffer.Bytes())
	if err != nil {
		a.serveError(w, r, "Failed to persist image", http.StatusBadRequest)
		return
	}
	// Set bool
	a.hasProfileImageBool = true
	// Clear http cache
	a.cache.purge()
	// Redirect
	http.Redirect(w, r, a.profileImagePath(profileImageFormatJPEG, 0, 100), http.StatusFound)
}

func (a *goBlog) serveDeleteProfileImage(w http.ResponseWriter, r *http.Request) {
	a.hasProfileImageBool = false
	err := a.db.clearPersistentCache(profileImageHashCacheName)
	if err != nil {
		a.serveError(w, r, "Failed to delete hash of profile image", http.StatusInternalServerError)
		return
	}
	err = a.db.clearPersistentCache(profileImageCacheName)
	if err != nil {
		a.serveError(w, r, "Failed to delete profile image", http.StatusInternalServerError)
		return
	}
	a.cache.purge()
	// Redirect
	http.Redirect(w, r, a.profileImagePath(profileImageFormatJPEG, 0, 100), http.StatusFound)
}
