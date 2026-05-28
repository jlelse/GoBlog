package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.goblog.app/app/pkgs/bufferpool"
	"go.goblog.app/app/pkgs/htmlbuilder"
)

func Test_variantTypeAllowedForSource(t *testing.T) {
	tests := []struct {
		source string
		target string
		allow  bool
	}{
		{source: "jpg", target: "jpeg", allow: true},
		{source: "jpg", target: "avif", allow: true},
		{source: "jpg", target: "png", allow: false},
		{source: "jpeg", target: "jpeg", allow: true},
		{source: "jpeg", target: "avif", allow: true},
		{source: "jpeg", target: "png", allow: false},
		{source: "png", target: "png", allow: true},
		{source: "png", target: "avif", allow: true},
		{source: "png", target: "jpeg", allow: false},
		{source: "png", target: "jpg", allow: false},
		{source: "gif", target: "avif", allow: false},
		{source: "gif", target: "jpeg", allow: false},
		{source: "svg", target: "jpeg", allow: false},
		{source: "webp", target: "avif", allow: false},
		{source: "bmp", target: "jpeg", allow: false},
		{source: "tiff", target: "jpeg", allow: false},
		{source: "avif", target: "avif", allow: false},
	}

	for _, tt := range tests {
		t.Run(tt.source+"->"+tt.target, func(t *testing.T) {
			assert.Equal(t, tt.allow, variantTypeAllowedForSource(tt.source, tt.target))
		})
	}
}

func Test_extractMediaHashFromURL(t *testing.T) {
	t.Run("extracts hash from file.ext URLs", func(t *testing.T) {
		cfg := createDefaultTestConfig(t)
		app := &goBlog{cfg: cfg}

		tests := []struct {
			url  string
			want string
		}{
			{url: "/m/abc123.jpg", want: "abc123"},
			{url: "https://example.com/m/abc123.jpg", want: "abc123"},
			{url: "https://example.net/m/abc123.jpg", want: "abc123"},
			{url: "https://media.example.com/media/abc123.jpg", want: "abc123"},
			{url: "https://media.example.com/media/sub/abc.jpg", want: "abc"},
			{url: "https://media.example.com/other/abc.jpg", want: "abc"},
			{url: "https://media.example.net/media/abc.jpg", want: "abc"},
			{url: "", want: ""},
			{url: "invalid://", want: ""},
		}

		for _, tt := range tests {
			t.Run(tt.url, func(t *testing.T) {
				assert.Equal(t, tt.want, app.extractMediaHashFromURL(tt.url))
			})
		}
	})

	t.Run("rejects URLs without hex hash", func(t *testing.T) {
		app := &goBlog{cfg: createDefaultTestConfig(t)}

		tests := []string{
			"/m/",
			"/",
			"https://example.com",
			"https://example.com/anything-else",
			"https://example.com/file.tar.gz",
			"https://example.com/a.b.c.jpg",
			"https://example.net/path/image.png",
		}

		for _, tt := range tests {
			t.Run(tt, func(t *testing.T) {
				assert.Equal(t, "", app.extractMediaHashFromURL(tt))
			})
		}
	})
}

func Test_parseVariantType(t *testing.T) {
	for _, tc := range []struct {
		vt     string
		format string
		width  int
	}{
		{"avif_2000", "avif", 2000},
		{"jpeg_800", "jpeg", 800},
		{"jpeg_2000", "jpeg", 2000},
		{"", "", 0},
		{"avif", "avif", 0},
		{"_2000", "", 2000},
	} {
		f, w := parseVariantType(tc.vt)
		assert.Equal(t, tc.format, f, "format for %q", tc.vt)
		assert.Equal(t, tc.width, w, "width for %q", tc.vt)
	}
}

func Test_mediaOptimizationEnabled(t *testing.T) {
	app := &goBlog{cfg: createDefaultTestConfig(t)}

	assert.False(t, app.mediaOptimizationEnabled())

	app.cfg.MediaOptimization.Enabled = true
	assert.True(t, app.mediaOptimizationEnabled())

	app.cfg.MediaOptimization = nil
	assert.False(t, app.mediaOptimizationEnabled())
}

func Test_mediaOptimizationImgproxyConfigured(t *testing.T) {
	app := &goBlog{cfg: createDefaultTestConfig(t)}

	assert.False(t, app.mediaOptimizationImgproxyConfigured())

	app.cfg.MediaOptimization.ImgproxyURL = "http://localhost:8080"
	assert.True(t, app.mediaOptimizationImgproxyConfigured())

	app.cfg.MediaOptimization = nil
	assert.False(t, app.mediaOptimizationImgproxyConfigured())
}

func mediaOptimizedTestDatabase(t *testing.T) *database {
	t.Helper()
	app := &goBlog{cfg: createDefaultTestConfig(t)}
	err := app.initConfig(false)
	require.NoError(t, err)
	return app.db
}

func Test_mediaOptimizedDatabase(t *testing.T) {
	db := mediaOptimizedTestDatabase(t)

	row := &mediaOptimizedRow{
		OriginalHash:  "abc123",
		VariantType:   "avif_800",
		OptimizedHash: "def456",
		Width:         800,
		Height:        600,
	}

	err := db.mediaOptimizedInsert(row)
	require.NoError(t, err)

	rows, err := db.mediaOptimizedByOriginal("abc123")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "abc123", rows[0].OriginalHash)
	assert.Equal(t, "avif_800", rows[0].VariantType)
	assert.Equal(t, "def456", rows[0].OptimizedHash)
	assert.Equal(t, 800, rows[0].Width)
	assert.Equal(t, 600, rows[0].Height)

	// Query by optimized hash
	variantRows, err := db.mediaOptimizedByOptimized("def456")
	require.NoError(t, err)
	require.Len(t, variantRows, 1)
	assert.Equal(t, "abc123", variantRows[0].OriginalHash)
	assert.Equal(t, "avif_800", variantRows[0].VariantType)
	assert.Equal(t, "def456", variantRows[0].OptimizedHash)
	assert.Equal(t, 800, variantRows[0].Width)
	assert.Equal(t, 600, variantRows[0].Height)

	// Delete by optimized hash
	err = db.mediaOptimizedDeleteByOptimized("def456")
	require.NoError(t, err)

	rows, err = db.mediaOptimizedByOriginal("abc123")
	require.NoError(t, err)
	assert.Empty(t, rows)

	// Re-insert and test delete by original
	err = db.mediaOptimizedInsert(row)
	require.NoError(t, err)

	err = db.mediaOptimizedDeleteByOriginal("abc123")
	require.NoError(t, err)

	rows, err = db.mediaOptimizedByOriginal("abc123")
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func Test_writePictureElement_noOptimization(t *testing.T) {
	app := &goBlog{cfg: createDefaultTestConfig(t)}
	app.cfg.MediaOptimization.Enabled = false

	buf := bufferpool.Get()
	defer bufferpool.Put(buf)
	hb := htmlbuilder.NewHTMLBuilder(buf)

	app.writePictureElement(hb, "https://example.com/m/pic.jpg", "A photo", "", "u-photo", "", false)

	output := buf.String()
	assert.Contains(t, output, `<a href="https://example.com/m/pic.jpg"`)
	assert.Contains(t, output, `<img`)
	assert.Contains(t, output, `src="https://example.com/m/pic.jpg"`)
	assert.Contains(t, output, `alt="A photo"`)
	assert.Contains(t, output, `class="u-photo"`)
	assert.Contains(t, output, `loading="lazy"`)
	assert.NotContains(t, output, `<picture>`)
	assert.NotContains(t, output, `<source`)
}

func Test_writePictureElement_noVariants(t *testing.T) {
	app := &goBlog{cfg: createDefaultTestConfig(t)}
	app.cfg.MediaOptimization.Enabled = true
	err := app.initConfig(false)
	require.NoError(t, err)

	buf := bufferpool.Get()
	defer bufferpool.Put(buf)
	hb := htmlbuilder.NewHTMLBuilder(buf)

	app.writePictureElement(hb, "https://example.com/m/pic.jpg", "A photo", "", "", "", false)

	output := buf.String()
	assert.Contains(t, output, `<a href="https://example.com/m/pic.jpg"`)
	assert.Contains(t, output, `<img`)
	assert.Contains(t, output, `src="https://example.com/m/pic.jpg"`)
	assert.NotContains(t, output, `<picture>`)
}

func Test_writePictureElement_withVariants(t *testing.T) {
	storagePath := t.TempDir()
	app := newAppWithStorage(t, &localMediaStorage{
		path:     storagePath,
		mediaURL: "https://example.com/m",
	})
	app.cfg.MediaOptimization.Enabled = true
	app.cfg.MediaOptimization.Formats = []string{
		"avif",
		"jpeg",
	}
	app.cfg.MediaOptimization.Widths = []int{800, 1400, 2000}
	err := app.initConfig(false)
	require.NoError(t, err)
	app.initMediaOptimization()

	db := app.db

	for _, row := range []*mediaOptimizedRow{
		{OriginalHash: "abc123", VariantType: "avif_2000", OptimizedHash: "h1", Width: 2000, Height: 1500},
		{OriginalHash: "abc123", VariantType: "avif_1400", OptimizedHash: "h7", Width: 1400, Height: 1050},
		{OriginalHash: "abc123", VariantType: "avif_800", OptimizedHash: "h5", Width: 800, Height: 600},
		{OriginalHash: "abc123", VariantType: "jpeg_2000", OptimizedHash: "h4", Width: 2000, Height: 1500},
		{OriginalHash: "abc123", VariantType: "jpeg_1400", OptimizedHash: "h8", Width: 1400, Height: 1050},
		{OriginalHash: "abc123", VariantType: "jpeg_800", OptimizedHash: "h6", Width: 800, Height: 600},
	} {
		err = db.mediaOptimizedInsert(row)
		require.NoError(t, err)
	}

	buf := bufferpool.Get()
	defer bufferpool.Put(buf)
	hb := htmlbuilder.NewHTMLBuilder(buf)

	app.writePictureElement(hb, "https://example.com/m/abc123.jpg", "Photo alt", "", "u-photo extra-class", "", false)

	output := buf.String()
	assert.Contains(t, output, `<a href="https://example.com/m/h4.jpeg"`)
	assert.Contains(t, output, `<picture>`)
	assert.Contains(t, output, `type="image/avif"`)
	assert.Contains(t, output, `type="image/jpeg"`)
	assert.Contains(t, output, `srcset="https://example.com/m/h1.avif 2000w, https://example.com/m/h7.avif 1400w, https://example.com/m/h5.avif 800w"`)
	assert.Contains(t, output, `srcset="https://example.com/m/h4.jpeg 2000w, https://example.com/m/h8.jpeg 1400w, https://example.com/m/h6.jpeg 800w"`)
	assert.Contains(t, output, `src="https://example.com/m/h4.jpeg"`)
	assert.Contains(t, output, `alt="Photo alt"`)
	assert.Contains(t, output, `class="u-photo extra-class"`)
	assert.Contains(t, output, `width="2000"`)
	assert.Contains(t, output, `height="1500"`)
	assert.Contains(t, output, `loading="lazy"`)
	assert.NotContains(t, output, `media=`)
	assert.Contains(t, output, `sizes="(max-width: 700px) 100vw, 700px"`)
}

func Test_writePictureElement_customClass(t *testing.T) {
	app := &goBlog{cfg: createDefaultTestConfig(t)}
	app.cfg.MediaOptimization.Enabled = false
	err := app.initConfig(false)
	require.NoError(t, err)
	app.initMediaOptimization()

	buf := bufferpool.Get()
	defer bufferpool.Put(buf)
	hb := htmlbuilder.NewHTMLBuilder(buf)

	app.writePictureElement(hb, "/m/test.png", "test", "", "u-photo hello world", "", false)

	output := buf.String()
	assert.Contains(t, output, `class="u-photo hello world"`)
	assert.Contains(t, output, `<a href="/m/test.png"`)
}

func Test_callImgproxy_invalidURL(t *testing.T) {
	app := &goBlog{cfg: createDefaultTestConfig(t)}
	app.cfg.MediaOptimization.Enabled = true
	app.cfg.MediaOptimization.ImgproxyURL = "http://127.0.0.1:1"

	buf := bufferpool.Get()
	defer bufferpool.Put(buf)
	err := app.callImgproxy("http://example.com/img.jpg", &variantType{Format: "avif", Width: 800}, buf)
	assert.Error(t, err)
}

func Test_callImgproxy_success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/fit/w:800/h:0/f:avif/plain/http://example.com/img.jpg")
		w.Header().Set("Content-Type", "image/avif")
		_, _ = w.Write([]byte("fake-avif-data"))
	}))
	defer ts.Close()

	app := &goBlog{cfg: createDefaultTestConfig(t)}
	app.cfg.MediaOptimization.Enabled = true
	app.cfg.MediaOptimization.ImgproxyURL = ts.URL
	app.httpClient = ts.Client()

	buf := bufferpool.Get()
	defer bufferpool.Put(buf)
	err := app.callImgproxy("http://example.com/img.jpg", &variantType{Format: "avif", Width: 800}, buf)
	require.NoError(t, err)
	assert.Equal(t, []byte("fake-avif-data"), buf.Bytes())
}

func Test_deleteOptimizedMediaFile_disabled(t *testing.T) {
	storagePath := t.TempDir()
	app := newAppWithStorage(t, &localMediaStorage{path: storagePath})
	app.cfg.MediaOptimization.Enabled = false

	testContent := []byte("test data")
	filename := "abc123.txt"
	require.NoError(t, os.WriteFile(filepath.Join(storagePath, filename), testContent, 0644))

	err := app.deleteOptimizedMediaFile(filename)
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(storagePath, filename))
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func Test_deleteOptimizedMediaFile_withVariants(t *testing.T) {
	storagePath := t.TempDir()
	app := newAppWithStorage(t, &localMediaStorage{path: storagePath})
	app.cfg.MediaOptimization.Enabled = true
	err := app.initConfig(false)
	require.NoError(t, err)

	db := app.db

	err = db.mediaOptimizedInsert(&mediaOptimizedRow{
		OriginalHash: "abc123", VariantType: "avif_800",
		OptimizedHash: "var1", Width: 800, Height: 600,
	})
	require.NoError(t, err)
	err = db.mediaOptimizedInsert(&mediaOptimizedRow{
		OriginalHash: "abc123", VariantType: "avif_2000",
		OptimizedHash: "var2", Width: 2000, Height: 1500,
	})
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(storagePath, "abc123.jpg"), []byte("original"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(storagePath, "var1.avif"), []byte("variant1"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(storagePath, "var2.avif"), []byte("variant2"), 0644))

	err = app.deleteOptimizedMediaFile("abc123.jpg")
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(storagePath, "abc123.jpg"))
	assert.ErrorIs(t, err, os.ErrNotExist, "original should be deleted")
	_, err = os.Stat(filepath.Join(storagePath, "var1.avif"))
	assert.ErrorIs(t, err, os.ErrNotExist, "variant1 should be deleted")
	_, err = os.Stat(filepath.Join(storagePath, "var2.avif"))
	assert.ErrorIs(t, err, os.ErrNotExist, "variant2 should be deleted")

	rows, err := db.mediaOptimizedByOriginal("abc123")
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func Test_deleteOptimizedMediaFile_variant(t *testing.T) {
	storagePath := t.TempDir()
	app := newAppWithStorage(t, &localMediaStorage{path: storagePath})
	app.cfg.MediaOptimization.Enabled = true
	err := app.initConfig(false)
	require.NoError(t, err)

	db := app.db

	err = db.mediaOptimizedInsert(&mediaOptimizedRow{
		OriginalHash: "abc123", VariantType: "avif_800",
		OptimizedHash: "var1", Width: 800, Height: 600,
	})
	require.NoError(t, err)
	err = db.mediaOptimizedInsert(&mediaOptimizedRow{
		OriginalHash: "abc123", VariantType: "avif_2000",
		OptimizedHash: "var2", Width: 2000, Height: 1500,
	})
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(storagePath, "var1.avif"), []byte("variant1"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(storagePath, "var2.avif"), []byte("variant2"), 0644))

	// Delete a variant file (not the original)
	err = app.deleteOptimizedMediaFile("var1.avif")
	require.NoError(t, err)

	// Variant file should be deleted
	_, err = os.Stat(filepath.Join(storagePath, "var1.avif"))
	assert.ErrorIs(t, err, os.ErrNotExist, "deleted variant should be removed")
	// Other variant should remain
	_, err = os.Stat(filepath.Join(storagePath, "var2.avif"))
	assert.NoError(t, err, "other variant should remain")

	// DB record for deleted variant should be removed
	rows, err := db.mediaOptimizedByOriginal("abc123")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "var2", rows[0].OptimizedHash)

	// DB record for other variant should remain
	variantRows, err := db.mediaOptimizedByOptimized("var2")
	require.NoError(t, err)
	assert.Len(t, variantRows, 1)
}

func Test_mediaOptimizeUploadFlow_integration(t *testing.T) {
	storagePath := t.TempDir()
	app := newAppWithStorage(t, &localMediaStorage{path: storagePath})
	app.cfg.MediaOptimization.Enabled = true
	app.cfg.MediaOptimization.ImgproxyURL = "http://localhost:1"
	err := app.initConfig(false)
	require.NoError(t, err)
	app.initMediaOptimization()

	content := []byte("test image content for sha256")
	hash := sha256.Sum256(content)
	hashHex := fmt.Sprintf("%x", hash)
	filename := hashHex + ".jpg"

	loc, err := app.saveMediaFile(filename, bytes.NewReader(content))
	require.NoError(t, err)
	assert.Contains(t, loc, filename)

	app.optimizeMediaFile(hashHex, ".jpg")

	written, err := os.ReadFile(filepath.Join(storagePath, filename))
	require.NoError(t, err)
	assert.Equal(t, content, written)
}

func Test_mediaOptimizedByOriginal_empty(t *testing.T) {
	db := mediaOptimizedTestDatabase(t)

	rows, err := db.mediaOptimizedByOriginal("nope")
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func Test_isImageExtension(t *testing.T) {
	tests := []struct {
		ext  string
		want bool
	}{
		{".jpg", true},
		{".jpeg", true},
		{".png", true},
		{".JPG", true},
		{".JPEG", true},
		{".PNG", true},
		{".gif", false},
		{".webp", false},
		{".avif", false},
		{".svg", false},
		{".bmp", false},
		{".tiff", false},
		{".pdf", false},
		{".txt", false},
		{"", false},
		{".", false},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			assert.Equal(t, tt.want, isImageExtension(tt.ext))
		})
	}
}

func Test_initMediaOptimization(t *testing.T) {
	t.Run("disabled when not enabled", func(t *testing.T) {
		app := &goBlog{cfg: createDefaultTestConfig(t)}
		app.initMediaOptimization()
		assert.Nil(t, app.mediaOptimizationVariants)
	})

	t.Run("populates variants from config", func(t *testing.T) {
		app := &goBlog{cfg: createDefaultTestConfig(t)}
		app.cfg.MediaOptimization.Enabled = true
		app.cfg.MediaOptimization.Formats = []string{"avif", "jpeg"}
		app.cfg.MediaOptimization.Widths = []int{800, 1400, 2000}
		app.initMediaOptimization()
		require.Len(t, app.mediaOptimizationVariants, 6)
		assert.Equal(t, "avif", app.mediaOptimizationVariants[0].Format)
		assert.Equal(t, 800, app.mediaOptimizationVariants[0].Width)
		assert.Equal(t, "avif", app.mediaOptimizationVariants[1].Format)
		assert.Equal(t, 1400, app.mediaOptimizationVariants[1].Width)
		assert.Equal(t, "avif", app.mediaOptimizationVariants[2].Format)
		assert.Equal(t, 2000, app.mediaOptimizationVariants[2].Width)
		assert.Equal(t, "jpeg", app.mediaOptimizationVariants[3].Format)
		assert.Equal(t, 800, app.mediaOptimizationVariants[3].Width)
		assert.Equal(t, "jpeg", app.mediaOptimizationVariants[4].Format)
		assert.Equal(t, 1400, app.mediaOptimizationVariants[4].Width)
		assert.Equal(t, "jpeg", app.mediaOptimizationVariants[5].Format)
		assert.Equal(t, 2000, app.mediaOptimizationVariants[5].Width)
	})

	t.Run("uses all non-empty format strings", func(t *testing.T) {
		app := &goBlog{cfg: createDefaultTestConfig(t)}
		app.cfg.MediaOptimization.Enabled = true
		app.cfg.MediaOptimization.Formats = []string{"avif", "", "jpeg", "malformed"}
		app.cfg.MediaOptimization.Widths = []int{800}
		app.initMediaOptimization()
		require.Len(t, app.mediaOptimizationVariants, 3)
		assert.Equal(t, "avif", app.mediaOptimizationVariants[0].Format)
		assert.Equal(t, "jpeg", app.mediaOptimizationVariants[1].Format)
		assert.Equal(t, "malformed", app.mediaOptimizationVariants[2].Format)
	})

	t.Run("no variants when formats empty", func(t *testing.T) {
		app := &goBlog{cfg: createDefaultTestConfig(t)}
		app.cfg.MediaOptimization.Enabled = true
		app.cfg.MediaOptimization.Formats = []string{}
		app.initMediaOptimization()
		assert.Empty(t, app.mediaOptimizationVariants)
	})
}

func Test_groupMediaVariants(t *testing.T) {
	t.Run("single variant", func(t *testing.T) {
		g := groupMediaVariants([]*mediaOptimizedRow{
			{OriginalHash: "a", VariantType: "jpeg_800", OptimizedHash: "h1", Width: 800, Height: 600},
		})
		require.NotNil(t, g)
		assert.Equal(t, []string{"jpeg"}, g.sortedFormats)
		assert.Equal(t, "jpeg", g.fallbackFormat)
		assert.Equal(t, "h1", g.fallbackRow.OptimizedHash)
	})

	t.Run("multiple formats correct priority order", func(t *testing.T) {
		g := groupMediaVariants([]*mediaOptimizedRow{
			{OriginalHash: "a", VariantType: "avif_800", OptimizedHash: "h1", Width: 800, Height: 600},
			{OriginalHash: "a", VariantType: "jpeg_800", OptimizedHash: "h2", Width: 800, Height: 600},
			{OriginalHash: "a", VariantType: "png_800", OptimizedHash: "h3", Width: 800, Height: 600},
		})
		require.NotNil(t, g)
		assert.Equal(t, []string{"avif", "jpeg", "png"}, g.sortedFormats)
	})

	t.Run("fallback is lowest priority format with largest width", func(t *testing.T) {
		g := groupMediaVariants([]*mediaOptimizedRow{
			{OriginalHash: "a", VariantType: "avif_2000", OptimizedHash: "h1", Width: 2000, Height: 1500},
			{OriginalHash: "a", VariantType: "avif_800", OptimizedHash: "h2", Width: 800, Height: 600},
			{OriginalHash: "a", VariantType: "png_2000", OptimizedHash: "h3", Width: 2000, Height: 1500},
			{OriginalHash: "a", VariantType: "png_800", OptimizedHash: "h4", Width: 800, Height: 600},
		})
		require.NotNil(t, g)
		assert.Equal(t, "png", g.fallbackFormat)
		assert.Equal(t, "h3", g.fallbackRow.OptimizedHash)
	})

	t.Run("single format with multiple widths", func(t *testing.T) {
		g := groupMediaVariants([]*mediaOptimizedRow{
			{OriginalHash: "a", VariantType: "jpeg_2000", OptimizedHash: "h1", Width: 2000, Height: 1500},
			{OriginalHash: "a", VariantType: "jpeg_800", OptimizedHash: "h2", Width: 800, Height: 600},
			{OriginalHash: "a", VariantType: "jpeg_400", OptimizedHash: "h3", Width: 400, Height: 300},
		})
		require.NotNil(t, g)
		assert.Equal(t, []string{"jpeg"}, g.sortedFormats)
		assert.Equal(t, "jpeg", g.fallbackFormat)
		assert.Equal(t, "h1", g.fallbackRow.OptimizedHash)
	})

	t.Run("fallback picks maximum width in fallback format", func(t *testing.T) {
		g := groupMediaVariants([]*mediaOptimizedRow{
			{OriginalHash: "a", VariantType: "avif_2000", OptimizedHash: "h1", Width: 2000, Height: 1500},
			{OriginalHash: "a", VariantType: "jpeg_400", OptimizedHash: "h2", Width: 400, Height: 300},
			{OriginalHash: "a", VariantType: "jpeg_800", OptimizedHash: "h3", Width: 800, Height: 600},
		})
		require.NotNil(t, g)
		assert.Equal(t, "jpeg", g.fallbackFormat)
		assert.Equal(t, "h3", g.fallbackRow.OptimizedHash)
	})
}

func Test_mediaFallbackURL(t *testing.T) {
	storagePath := t.TempDir()
	app := newAppWithStorage(t, &localMediaStorage{
		path:     storagePath,
		mediaURL: "https://example.com/m",
	})
	app.cfg.MediaOptimization.Enabled = true
	err := app.initConfig(false)
	require.NoError(t, err)

	t.Run("returns original when optimization disabled", func(t *testing.T) {
		app2 := &goBlog{cfg: createDefaultTestConfig(t)}
		assert.Equal(t, "/m/pic.jpg", app2.mediaFallbackURL("/m/pic.jpg"))
	})

	t.Run("returns original when no hash in URL", func(t *testing.T) {
		assert.Equal(t, "/not-a-hash.jpg", app.mediaFallbackURL("/not-a-hash.jpg"))
	})

	t.Run("returns original when no variants in DB", func(t *testing.T) {
		app.mediaFallbackURL("https://example.com/m/unknown.jpg")
		result := app.mediaFallbackURL("https://example.com/m/unknown.jpg")
		assert.Equal(t, "https://example.com/m/unknown.jpg", result)
	})

	t.Run("returns optimized variant URL when variants exist", func(t *testing.T) {
		db := app.db
		require.NoError(t, db.mediaOptimizedInsert(&mediaOptimizedRow{
			OriginalHash: "abc123", VariantType: "avif_2000", OptimizedHash: "h1", Width: 2000, Height: 1500,
		}))
		require.NoError(t, db.mediaOptimizedInsert(&mediaOptimizedRow{
			OriginalHash: "abc123", VariantType: "jpeg_800", OptimizedHash: "h2", Width: 800, Height: 600,
		}))

		result := app.mediaFallbackURL("https://example.com/m/abc123.jpg")
		assert.Equal(t, "https://example.com/m/h2.jpeg", result)
	})
}

func Test_mediaOptimizedHashSets(t *testing.T) {
	db := mediaOptimizedTestDatabase(t)

	t.Run("empty table", func(t *testing.T) {
		o, v, err := db.mediaOptimizedHashSets()
		require.NoError(t, err)
		assert.Empty(t, o)
		assert.Empty(t, v)
	})

	t.Run("returns originals and variants", func(t *testing.T) {
		require.NoError(t, db.mediaOptimizedInsert(&mediaOptimizedRow{
			OriginalHash: "abc", VariantType: "avif_800", OptimizedHash: "h1", Width: 800, Height: 600,
		}))
		require.NoError(t, db.mediaOptimizedInsert(&mediaOptimizedRow{
			OriginalHash: "abc", VariantType: "avif_2000", OptimizedHash: "h2", Width: 2000, Height: 1500,
		}))

		o, v, err := db.mediaOptimizedHashSets()
		require.NoError(t, err)
		assert.Len(t, o, 1)
		assert.True(t, o["abc"])
		assert.Len(t, v, 2)
		assert.True(t, v["h1"])
		assert.True(t, v["h2"])
	})
}

func Test_writeImgElement(t *testing.T) {
	app := &goBlog{cfg: createDefaultTestConfig(t)}
	app.cfg.MediaOptimization.Enabled = true
	app.initMediaOptimization()

	t.Run("basic img with src and alt", func(t *testing.T) {
		buf := bufferpool.Get()
		defer bufferpool.Put(buf)
		hb := htmlbuilder.NewHTMLBuilder(buf)
		app.writeImgElement(hb, "/m/pic.jpg", "Alt text", "", "my-class", 0, 0, "")
		output := buf.String()
		assert.Contains(t, output, `<img`)
		assert.Contains(t, output, `src="/m/pic.jpg"`)
		assert.Contains(t, output, `alt="Alt text"`)
		assert.Contains(t, output, `class="my-class"`)
		assert.Contains(t, output, `loading="lazy"`)
		assert.NotContains(t, output, `width=`)
		assert.NotContains(t, output, `height=`)
		assert.NotContains(t, output, `title=`)
		assert.NotContains(t, output, `srcset=`)
	})

	t.Run("with title attribute", func(t *testing.T) {
		buf := bufferpool.Get()
		defer bufferpool.Put(buf)
		hb := htmlbuilder.NewHTMLBuilder(buf)
		app.writeImgElement(hb, "/m/pic.jpg", "Alt", "A title", "", 0, 0, "")
		output := buf.String()
		assert.Contains(t, output, `title="A title"`)
	})

	t.Run("with srcset", func(t *testing.T) {
		buf := bufferpool.Get()
		defer bufferpool.Put(buf)
		hb := htmlbuilder.NewHTMLBuilder(buf)
		app.writeImgElement(hb, "/m/pic.jpg", "Alt", "", "", 0, 0, "/m/pic-800w.jpg 800w, /m/pic-2000w.jpg 2000w")
		output := buf.String()
		assert.Contains(t, output, `srcset="/m/pic-800w.jpg 800w, /m/pic-2000w.jpg 2000w"`)
		assert.Contains(t, output, `sizes="(max-width: 700px) 100vw, 700px"`)
	})

	t.Run("with width and height", func(t *testing.T) {
		buf := bufferpool.Get()
		defer bufferpool.Put(buf)
		hb := htmlbuilder.NewHTMLBuilder(buf)
		app.writeImgElement(hb, "/m/pic.jpg", "Alt", "", "", 800, 600, "")
		output := buf.String()
		assert.Contains(t, output, `width="800"`)
		assert.Contains(t, output, `height="600"`)
	})

	t.Run("all attributes together", func(t *testing.T) {
		buf := bufferpool.Get()
		defer bufferpool.Put(buf)
		hb := htmlbuilder.NewHTMLBuilder(buf)
		app.writeImgElement(hb, "/m/pic.jpg", "Alt", "Title", "class-a class-b", 800, 600, "srcset.jpg 800w")
		output := buf.String()
		assert.Contains(t, output, `src="/m/pic.jpg"`)
		assert.Contains(t, output, `alt="Alt"`)
		assert.Contains(t, output, `title="Title"`)
		assert.Contains(t, output, `class="class-a class-b"`)
		assert.Contains(t, output, `loading="lazy"`)
		assert.Contains(t, output, `width="800"`)
		assert.Contains(t, output, `height="600"`)
		assert.Contains(t, output, `srcset="srcset.jpg 800w"`)
		assert.Contains(t, output, `sizes="(max-width: 700px) 100vw, 700px"`)
	})

	t.Run("empty alt and class", func(t *testing.T) {
		buf := bufferpool.Get()
		defer bufferpool.Put(buf)
		hb := htmlbuilder.NewHTMLBuilder(buf)
		app.writeImgElement(hb, "/m/pic.jpg", "", "", "", 0, 0, "")
		output := buf.String()
		assert.Contains(t, output, `alt=""`)
		assert.Contains(t, output, `class=""`)
	})
}
