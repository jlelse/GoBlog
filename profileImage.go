package main

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	_ "embed"

	"github.com/disintegration/imaging"
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
	profileImageFile             = "data/profileImage"

	profileImageNoImageHash = "x"

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
		var imageReader io.ReadCloser
		if a.hasProfileImage() {
			var err error
			imageReader, err = os.Open(profileImageFile)
			if err != nil {
				a.serveError(w, r, "Failed to open image file", http.StatusInternalServerError)
				return
			}
		} else {
			imageReader = io.NopCloser(bytes.NewReader(defaultLogo))
		}
		// Decode image
		img, err := imaging.Decode(imageReader, imaging.AutoOrientation(true))
		_ = imageReader.Close()
		if err != nil {
			a.serveError(w, r, "Failed to decode image", http.StatusInternalServerError)
			return
		}
		// Resize image
		resizedImage := imaging.Fit(img, width, height, imaging.Lanczos)
		// Encode
		pr, pw := io.Pipe()
		go func() {
			_ = pw.CloseWithError(encode(pw, resizedImage, quality))
		}()
		// Return
		w.Header().Set(contentType, mediaType)
		_, err = io.Copy(w, pr)
		_ = pr.CloseWithError(err)
	}
}

func (a *goBlog) profileImagePath(format profileImageFormat, size, quality int) string {
	if !a.hasProfileImage() {
		return fmt.Sprintf("%s.%s", profileImagePath, format)
	}
	query := url.Values{}
	query.Set("v", a.profileImageHash())
	if quality != 0 {
		query.Set("q", fmt.Sprintf("%d", quality))
	}
	if size != 0 {
		query.Set("s", fmt.Sprintf("%d", size))
	}
	return fmt.Sprintf("%s.%s?%s", profileImagePath, format, query.Encode())
}

func (a *goBlog) hasProfileImage() bool {
	return a.profileImageHash() != profileImageNoImageHash
}

func (a *goBlog) profileImageHash() string {
	_, _, _ = a.profileImageHashGroup.Do("", func() (interface{}, error) {
		if a.profileImageHashString != "" {
			return nil, nil
		}
		if _, err := os.Stat(profileImageFile); err != nil {
			a.profileImageHashString = profileImageNoImageHash
			return nil, nil
		}
		hash := sha256.New()
		file, err := os.Open(profileImageFile)
		if err != nil {
			a.profileImageHashString = profileImageNoImageHash
			return nil, nil
		}
		_, _ = io.Copy(hash, file)
		a.profileImageHashString = fmt.Sprintf("%x", hash.Sum(nil))
		return nil, nil
	})
	return a.profileImageHashString
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
	// Save the file locally
	err = os.MkdirAll(filepath.Dir(profileImageFile), 0777)
	if err != nil {
		a.serveError(w, r, "Failed to create directories", http.StatusBadRequest)
		return
	}
	dataFile, err := os.OpenFile(profileImageFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		a.serveError(w, r, "Failed to open local file", http.StatusBadRequest)
		return
	}
	_, err = io.Copy(dataFile, file)
	_ = file.Close()
	if err != nil {
		a.serveError(w, r, "Failed to save image", http.StatusBadRequest)
		return
	}
	// Reset hash
	a.profileImageHashString = ""
	// Clear http cache
	a.cache.purge()
	// Redirect
	http.Redirect(w, r, a.profileImagePath(profileImageFormatJPEG, 0, 100), http.StatusFound)
}

func (a *goBlog) serveDeleteProfileImage(w http.ResponseWriter, r *http.Request) {
	a.profileImageHashString = ""
	if err := os.Remove(profileImageFile); err != nil && !errors.Is(err, os.ErrNotExist) {
		a.serveError(w, r, "Failed to delete profile image", http.StatusInternalServerError)
		return
	}
	a.cache.purge()
	http.Redirect(w, r, a.profileImagePath(profileImageFormatJPEG, 0, 100), http.StatusFound)
}
