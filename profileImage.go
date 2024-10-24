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
	"regexp"
	"strconv"
	"strings"
	"sync"

	_ "embed"

	"github.com/kovidgoyal/imaging"
	"go.goblog.app/app/pkgs/bodylimit"
	"go.goblog.app/app/pkgs/contenttype"
	"go.goblog.app/app/pkgs/utils"
)

type profileImageFormat string

const (
	profileImageFormatPNG  profileImageFormat = "png"
	profileImageFormatJPEG profileImageFormat = "jpg"

	profileImagePath             = "/profile"
	profileImagePathJPEG         = profileImagePath + "." + string(profileImageFormatJPEG)
	profileImagePathPNG          = profileImagePath + "." + string(profileImageFormatPNG)
	profileImageSizeRegexPattern = `(?P<width>\d+)(x(?P<height>\d+))?`

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
		encode = func(output io.Writer, img *image.NRGBA, _ int) error {
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
			imageReader, err = os.Open(a.cfg.User.ProfileImageFile)
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
	if a.profileImageHashGroup == nil {
		a.profileImageHashGroup = new(sync.Once)
	}
	a.profileImageHashGroup.Do(func() {
		if _, err := os.Stat(a.cfg.User.ProfileImageFile); err != nil {
			a.profileImageHashString = profileImageNoImageHash
			return
		}
		hash := sha256.New()
		file, err := os.Open(a.cfg.User.ProfileImageFile)
		if err != nil {
			a.profileImageHashString = profileImageNoImageHash
			return
		}
		_, _ = io.Copy(hash, file)
		_ = file.Close()
		a.profileImageHashString = fmt.Sprintf("%x", hash.Sum(nil))
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
	if err := r.ParseMultipartForm(10 * bodylimit.MB); err != nil {
		a.serveError(w, r, "Failed to parse multipart form", http.StatusBadRequest)
		return
	}
	// Get form file
	file, _, err := r.FormFile("file")
	if err != nil {
		a.serveError(w, r, "Failed to read file", http.StatusBadRequest)
		return
	}
	defer file.Close()
	// Save the file locally
	if err := utils.SaveToFileWithMode(file, a.cfg.User.ProfileImageFile, 0777, 0755); err != nil {
		a.serveError(w, r, "Failed to save to storage", http.StatusBadRequest)
		return
	}
	// Reset hash
	a.profileImageHashGroup = nil
	// Clear http cache
	a.cache.purge()
	// Redirect
	http.Redirect(w, r, a.profileImagePath(profileImageFormatJPEG, 0, 100), http.StatusFound)
}

func (a *goBlog) serveDeleteProfileImage(w http.ResponseWriter, r *http.Request) {
	a.profileImageHashGroup = nil
	if err := os.Remove(a.cfg.User.ProfileImageFile); err != nil && !errors.Is(err, os.ErrNotExist) {
		a.serveError(w, r, "Failed to delete profile image", http.StatusInternalServerError)
		return
	}
	a.cache.purge()
	http.Redirect(w, r, a.profileImagePath(profileImageFormatJPEG, 0, 100), http.StatusFound)
}
