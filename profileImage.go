package main

import (
	"bytes"
	"context"
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
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "embed"

	"github.com/carlmjohnson/requests"
	"github.com/go-chi/chi/v5"
	"github.com/kovidgoyal/imaging"
	"go.goblog.app/app/pkgs/bodylimit"
	"go.goblog.app/app/pkgs/contenttype"
	"go.goblog.app/app/pkgs/utils"
)

type profileImageFormat string

const (
	profileImageFormatPNG  profileImageFormat = "png"
	profileImageFormatJPEG profileImageFormat = "jpg"
	profileImageFormatAVIF profileImageFormat = "avif"

	profileImagePath             = "/profile"
	profileImagePathJPEG         = profileImagePath + "." + string(profileImageFormatJPEG)
	profileImagePathPNG          = profileImagePath + "." + string(profileImageFormatPNG)
	profileImagePathAVIF         = profileImagePath + "." + string(profileImageFormatAVIF)
	profileImageOriginalPath     = "/profile-original"
	profileImageSizeRegexPattern = `(?P<width>\d+)(x(?P<height>\d+))?`

	profileImageNoImageHash = "x"

	settingsUpdateProfileImagePath = "/updateprofileimage"
	settingsDeleteProfileImagePath = "/deleteprofileimage"
)

//go:embed logo/GoBlog.png
var defaultLogo []byte

func (a *goBlog) serveProfileImage(format profileImageFormat) http.HandlerFunc {
	var mediaType string
	var encode func(output io.Writer, img image.Image) error
	switch format {
	case profileImageFormatPNG:
		mediaType = contenttype.PNG
		encode = func(output io.Writer, img image.Image) error {
			return imaging.Encode(output, img, imaging.PNG, imaging.PNGCompressionLevel(png.BestCompression))
		}
	case profileImageFormatAVIF:
		mediaType = contenttype.AVIF
	default:
		mediaType = contenttype.JPEG
		encode = func(output io.Writer, img image.Image) error {
			return imaging.Encode(output, img, imaging.JPEG, imaging.JPEGQuality(85))
		}
	}
	return func(w http.ResponseWriter, r *http.Request) {
		// Get requested size
		width, height := 0, 0
		sizeFormValue := r.FormValue("s") //nolint:gosec
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
		// Use imgproxy if configured
		if a.mediaOptimizationImgproxyConfigured() {
			if err := a.serveProfileImageViaImgproxy(w, r, format, width, height); err != nil {
				a.error("Failed to serve profile image via imgproxy", "err", err)
				a.serveProfileImageFallback(w, r, format, mediaType, encode, width, height)
			}
			return
		}
		// Fallback to local processing
		a.serveProfileImageFallback(w, r, format, mediaType, encode, width, height)
	}
}

func (a *goBlog) serveProfileImageViaImgproxy(w http.ResponseWriter, _ *http.Request, format profileImageFormat, width, height int) (err error) {
	defer func() {
		if r := recover(); r != nil {
			a.error("Panic while requesting profile image via imgproxy", "panic", r, "stack", string(debug.Stack()))
			err = fmt.Errorf("panic while requesting profile image via imgproxy: %v", r)
		}
	}()
	imgproxyURL := strings.TrimRight(a.cfg.MediaOptimization.ImgproxyURL, "/")
	sourceURL := a.getFullAddress(a.profileImageOriginalURL())
	imgproxyFormat := string(format)
	imgproxyURL = fmt.Sprintf("%s/fit/w:%d/h:%d/f:%s/plain/%s", imgproxyURL, width, height, imgproxyFormat, sourceURL)
	client := &http.Client{Transport: a.httpClient.Transport, Timeout: 5 * time.Minute}
	w.Header().Set(contentType, profileImageMediaType(format))
	return requests.URL(imgproxyURL).
		Client(client).
		ToWriter(w).
		Fetch(context.Background())
}

func (a *goBlog) serveProfileImageFallback(w http.ResponseWriter, r *http.Request, format profileImageFormat, mediaType string, encode func(output io.Writer, img image.Image) error, width, height int) {
	if format == profileImageFormatAVIF {
		a.serveError(w, r, "AVIF format not supported without imgproxy", http.StatusNotImplemented)
		return
	}
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
	img, err := imaging.Decode(imageReader, imaging.AutoOrientation(true))
	_ = imageReader.Close()
	if err != nil {
		a.serveError(w, r, "Failed to decode image", http.StatusInternalServerError)
		return
	}
	resizedImage := imaging.Fit(img, width, height, imaging.Lanczos)
	pr, pw := io.Pipe()
	go func() {
		_ = pw.CloseWithError(encode(pw, resizedImage))
	}()
	w.Header().Set(contentType, mediaType)
	_, err = io.Copy(w, pr)
	_ = pr.CloseWithError(err)
}

func (a *goBlog) serveProfileImageOriginal(w http.ResponseWriter, r *http.Request) {
	if chi.URLParam(r, "secret") != a.profileImageSecret {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
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
	defer imageReader.Close()
	w.Header().Set(contentType, "application/octet-stream")
	_, _ = io.Copy(w, imageReader)
}

func profileImageMediaType(format profileImageFormat) string {
	switch format {
	case profileImageFormatPNG:
		return contenttype.PNG
	case profileImageFormatAVIF:
		return contenttype.AVIF
	default:
		return contenttype.JPEG
	}
}

func (a *goBlog) initProfileImageSecret() {
	if a.profileImageSecret == "" {
		a.profileImageSecret = randomString(32)
	}
}

func (a *goBlog) profileImageOriginalURL() string {
	return fmt.Sprintf("%s/%s", profileImageOriginalPath, a.profileImageSecret)
}

func (a *goBlog) profileImagePath(format profileImageFormat, size int) string {
	if !a.hasProfileImage() {
		return fmt.Sprintf("%s.%s", profileImagePath, format)
	}
	query := url.Values{}
	query.Set("v", a.profileImageHash())
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
	if err := r.ParseMultipartForm(10 * bodylimit.MB); err != nil { //nolint:gosec
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
	a.purgeCache()
	// Redirect
	http.Redirect(w, r, a.profileImagePath(profileImageFormatJPEG, 0), http.StatusFound)
}

func (a *goBlog) serveDeleteProfileImage(w http.ResponseWriter, r *http.Request) {
	a.profileImageHashGroup = nil
	if err := os.Remove(a.cfg.User.ProfileImageFile); err != nil && !errors.Is(err, os.ErrNotExist) {
		a.serveError(w, r, "Failed to delete profile image", http.StatusInternalServerError)
		return
	}
	a.purgeCache()
	http.Redirect(w, r, a.profileImagePath(profileImageFormatJPEG, 0), http.StatusFound)
}
