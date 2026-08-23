package main

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_initProfileImageSecret(t *testing.T) {
	t.Run("generates secret when empty", func(t *testing.T) {
		app := &goBlog{cfg: createDefaultTestConfig(t)}
		assert.Empty(t, app.profileImageSecret)
		app.initProfileImageSecret()
		assert.NotEmpty(t, app.profileImageSecret)
		assert.Len(t, app.profileImageSecret, 32)
	})

	t.Run("does not overwrite existing secret", func(t *testing.T) {
		app := &goBlog{cfg: createDefaultTestConfig(t)}
		app.profileImageSecret = "existing-secret"
		app.initProfileImageSecret()
		assert.Equal(t, "existing-secret", app.profileImageSecret)
	})
}

func Test_profileImageOriginalURL(t *testing.T) {
	app := &goBlog{cfg: createDefaultTestConfig(t)}
	app.profileImageSecret = "test-secret-123"
	assert.Equal(t, "/profile-original/test-secret-123", app.profileImageOriginalURL())
}

func Test_profileImageMediaType(t *testing.T) {
	tests := []struct {
		format   profileImageFormat
		expected string
	}{
		{profileImageFormatJPEG, "image/jpeg"},
		{profileImageFormatPNG, "image/png"},
		{profileImageFormatAVIF, "image/avif"},
	}

	for _, tt := range tests {
		t.Run(string(tt.format), func(t *testing.T) {
			assert.Equal(t, tt.expected, profileImageMediaType(tt.format))
		})
	}
}

func Test_serveProfileImageOriginal(t *testing.T) {
	t.Run("returns 403 when secret does not match", func(t *testing.T) {
		app := &goBlog{cfg: createDefaultTestConfig(t)}
		app.profileImageSecret = "correct-secret"
		_ = app.initConfig(false)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("secret", "wrong-secret")
		req := httptest.NewRequest("GET", "/profile-original/wrong-secret", nil)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		rr := httptest.NewRecorder()

		app.serveProfileImageOriginal(rr, req)

		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("serves default logo when no profile image set", func(t *testing.T) {
		app := &goBlog{cfg: createDefaultTestConfig(t)}
		app.profileImageSecret = "test-secret"
		_ = app.initConfig(false)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("secret", "test-secret")
		req := httptest.NewRequest("GET", "/profile-original/test-secret", nil)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		rr := httptest.NewRecorder()

		app.serveProfileImageOriginal(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "application/octet-stream", rr.Header().Get("Content-Type"))
		assert.Equal(t, defaultLogo, rr.Body.Bytes())
	})

	t.Run("serves profile image when set", func(t *testing.T) {
		app := &goBlog{cfg: createDefaultTestConfig(t)}
		app.profileImageSecret = "test-secret"
		_ = app.initConfig(false)

		requestBody := &bytes.Buffer{}
		writer := multipart.NewWriter(requestBody)
		fileWriter, err := writer.CreateFormFile("file", "profile.jpg")
		require.NoError(t, err)
		_, err = fileWriter.Write(defaultLogo)
		require.NoError(t, err)
		writer.Close()

		uploadReq := httptest.NewRequest("POST", "/updateprofileimage", requestBody)
		uploadReq.Header.Set("Content-Type", writer.FormDataContentType())
		uploadRR := httptest.NewRecorder()
		app.serveUpdateProfileImage(uploadRR, uploadReq)
		require.Equal(t, http.StatusFound, uploadRR.Code)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("secret", "test-secret")
		req := httptest.NewRequest("GET", "/profile-original/test-secret", nil)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		rr := httptest.NewRecorder()

		app.serveProfileImageOriginal(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "application/octet-stream", rr.Header().Get("Content-Type"))
		assert.Equal(t, defaultLogo, rr.Body.Bytes())
	})
}

func Test_profileImagePath(t *testing.T) {
	app := &goBlog{cfg: createDefaultTestConfig(t)}
	_ = app.initConfig(false)

	t.Run("without profile image", func(t *testing.T) {
		assert.Equal(t, "/profile.jpg", app.profileImagePath(profileImageFormatJPEG, 0))
		assert.Equal(t, "/profile.png", app.profileImagePath(profileImageFormatPNG, 0))
		assert.Equal(t, "/profile.avif", app.profileImagePath(profileImageFormatAVIF, 0))
		assert.Equal(t, "/profile.jpg", app.profileImagePath(profileImageFormatJPEG, 256))
	})

	t.Run("with profile image", func(t *testing.T) {
		requestBody := &bytes.Buffer{}
		writer := multipart.NewWriter(requestBody)
		fileWriter, err := writer.CreateFormFile("file", "profile.jpg")
		require.NoError(t, err)
		_, err = fileWriter.Write(defaultLogo)
		require.NoError(t, err)
		writer.Close()

		uploadReq := httptest.NewRequest("POST", "/updateprofileimage", requestBody)
		uploadReq.Header.Set("Content-Type", writer.FormDataContentType())
		uploadRR := httptest.NewRecorder()
		app.serveUpdateProfileImage(uploadRR, uploadReq)
		require.Equal(t, http.StatusFound, uploadRR.Code)

		path := app.profileImagePath(profileImageFormatJPEG, 0)
		assert.Contains(t, path, "/profile.jpg?v=")
		assert.Contains(t, path, "v=e3da5a2d765ff693e7eb54cff717ae0f79ec79c06ed3e3adf6054a46c2824f32")

		pathWithSize := app.profileImagePath(profileImageFormatJPEG, 256)
		assert.Contains(t, pathWithSize, "s=256")
		assert.Contains(t, pathWithSize, "v=e3da5a2d765ff693e7eb54cff717ae0f79ec79c06ed3e3adf6054a46c2824f32")
	})
}

func Test_serveProfileImage_fallback(t *testing.T) {
	app := &goBlog{cfg: createDefaultTestConfig(t)}
	_ = app.initConfig(false)

	t.Run("serves JPEG", func(t *testing.T) {
		handler := app.serveProfileImage(profileImageFormatJPEG)
		req := httptest.NewRequest("GET", "/profile.jpg", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "image/jpeg", rr.Header().Get("Content-Type"))
		assert.NotEmpty(t, rr.Body.Bytes())
	})

	t.Run("serves PNG", func(t *testing.T) {
		handler := app.serveProfileImage(profileImageFormatPNG)
		req := httptest.NewRequest("GET", "/profile.png", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "image/png", rr.Header().Get("Content-Type"))
		assert.NotEmpty(t, rr.Body.Bytes())
	})

	t.Run("AVIF returns 501 without imgproxy", func(t *testing.T) {
		handler := app.serveProfileImage(profileImageFormatAVIF)
		req := httptest.NewRequest("GET", "/profile.avif", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNotImplemented, rr.Code)
	})
}

func Test_serveProfileImage_withImgproxy(t *testing.T) {
	t.Run("uses imgproxy when configured", func(t *testing.T) {
		imgproxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Contains(t, r.URL.Path, "/fit/")
			assert.Contains(t, r.URL.Path, "/f:avif/")
			assert.Contains(t, r.URL.Path, "/profile-original/test-secret")
			w.Header().Set("Content-Type", "image/avif")
			_, _ = w.Write([]byte("fake-avif-data"))
		}))
		defer imgproxyServer.Close()

		app := &goBlog{cfg: createDefaultTestConfig(t)}
		app.cfg.MediaOptimization = &configMediaOptimization{
			Enabled:     true,
			ImgproxyURL: imgproxyServer.URL,
		}
		app.profileImageSecret = "test-secret"
		app.httpClient = imgproxyServer.Client()
		_ = app.initConfig(false)

		handler := app.serveProfileImage(profileImageFormatAVIF)
		req := httptest.NewRequest("GET", "/profile.avif", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "image/avif", rr.Header().Get("Content-Type"))
	})

	t.Run("falls back to local on imgproxy error", func(t *testing.T) {
		app := &goBlog{cfg: createDefaultTestConfig(t)}
		app.cfg.MediaOptimization = &configMediaOptimization{
			Enabled:     true,
			ImgproxyURL: "http://localhost:1",
		}
		app.profileImageSecret = "test-secret"
		app.httpClient = newHTTPClient()
		_ = app.initConfig(false)

		handler := app.serveProfileImage(profileImageFormatJPEG)
		req := httptest.NewRequest("GET", "/profile.jpg", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "image/jpeg", rr.Header().Get("Content-Type"))
	})
}

func Test_hasProfileImage(t *testing.T) {
	app := &goBlog{cfg: createDefaultTestConfig(t)}
	_ = app.initConfig(false)

	t.Run("returns false when no profile image", func(t *testing.T) {
		assert.False(t, app.hasProfileImage())
	})

	t.Run("returns true when profile image exists", func(t *testing.T) {
		err := os.WriteFile(app.cfg.User.ProfileImageFile, defaultLogo, 0o644)
		require.NoError(t, err)
		app.profileImageHashGroup = nil
		assert.True(t, app.hasProfileImage())
	})
}

func Test_profileImageHash(t *testing.T) {
	app := &goBlog{cfg: createDefaultTestConfig(t)}
	_ = app.initConfig(false)

	t.Run("returns no-image hash when file does not exist", func(t *testing.T) {
		assert.Equal(t, profileImageNoImageHash, app.profileImageHash())
	})

	t.Run("returns hash when file exists", func(t *testing.T) {
		err := os.WriteFile(app.cfg.User.ProfileImageFile, defaultLogo, 0o644)
		require.NoError(t, err)
		app.profileImageHashGroup = nil
		hash := app.profileImageHash()
		assert.NotEqual(t, profileImageNoImageHash, hash)
		assert.Len(t, hash, 64)
	})

	t.Run("caches hash", func(t *testing.T) {
		hash1 := app.profileImageHash()
		hash2 := app.profileImageHash()
		assert.Equal(t, hash1, hash2)
	})
}

func Test_profileImageSizeParsing(t *testing.T) {
	tests := []struct {
		name           string
		query          string
		expectedWidth  int
		expectedHeight int
	}{
		{
			name:           "no size parameter defaults to 512x512",
			query:          "",
			expectedWidth:  512,
			expectedHeight: 512,
		},
		{
			name:           "width only sets height to same value",
			query:          "s=100",
			expectedWidth:  100,
			expectedHeight: 100,
		},
		{
			name:           "width x height format",
			query:          "s=100x200",
			expectedWidth:  100,
			expectedHeight: 200,
		},
		{
			name:           "width greater than 512 is capped",
			query:          "s=1000",
			expectedWidth:  512,
			expectedHeight: 512,
		},
		{
			name:           "height greater than 512 is set to width",
			query:          "s=200x1000",
			expectedWidth:  200,
			expectedHeight: 200,
		},
		{
			name:           "invalid format defaults to 512x512",
			query:          "s=invalid",
			expectedWidth:  512,
			expectedHeight: 512,
		},
		{
			name:           "zero width defaults to 512",
			query:          "s=0",
			expectedWidth:  512,
			expectedHeight: 512,
		},
		{
			name:           "negative values extract digits",
			query:          "s=-100",
			expectedWidth:  100,
			expectedHeight: 100,
		},
		{
			name:           "very large numbers are handled",
			query:          "s=999999x999999",
			expectedWidth:  512,
			expectedHeight: 512,
		},
		{
			name:           "partial format like 100x is handled",
			query:          "s=100x",
			expectedWidth:  100,
			expectedHeight: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedWidth, capturedHeight int

			imgproxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Parse the imgproxy URL to extract width and height
				// Format: /fit/w:512/h:512/f:jpg/plain/...
				path := r.URL.Path
				if _, after, ok := strings.Cut(path, "/w:"); ok {
					fmt.Sscanf(after, "%d", &capturedWidth)
				}
				if _, after, ok := strings.Cut(path, "/h:"); ok {
					fmt.Sscanf(after, "%d", &capturedHeight)
				}
				w.Header().Set("Content-Type", "image/jpeg")
				_, _ = w.Write([]byte("fake-image-data"))
			}))
			defer imgproxyServer.Close()

			app := &goBlog{cfg: createDefaultTestConfig(t)}
			app.cfg.MediaOptimization = &configMediaOptimization{
				Enabled:     true,
				ImgproxyURL: imgproxyServer.URL,
			}
			app.profileImageSecret = "test-secret"
			app.httpClient = imgproxyServer.Client()
			_ = app.initConfig(false)

			handler := app.serveProfileImage(profileImageFormatJPEG)
			req := httptest.NewRequest("GET", "/profile.jpg", nil)
			if tt.query != "" {
				req.URL.RawQuery = tt.query
			}
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)
			assert.Equal(t, tt.expectedWidth, capturedWidth, "width mismatch")
			assert.Equal(t, tt.expectedHeight, capturedHeight, "height mismatch")
		})
	}
}
