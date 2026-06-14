package main

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupCheckFormatsTest(t *testing.T) (*goBlog, string) {
	t.Helper()
	storagePath := t.TempDir()
	app := newAppWithStorage(t, &localMediaStorage{
		path:     storagePath,
		mediaURL: "https://example.com/m",
	})
	app.cfg.MediaOptimization.Enabled = true
	app.cfg.MediaOptimization.Formats = []string{"avif", "jpeg"}
	app.cfg.MediaOptimization.Widths = []int{800, 1400, 2000}
	err := app.initConfig(false)
	require.NoError(t, err)
	app.initMediaOptimization()
	return app, storagePath
}

func Test_checkMediaFormats_notEnabled(t *testing.T) {
	app := &goBlog{cfg: createDefaultTestConfig(t)}
	app.cfg.MediaOptimization.Enabled = false
	err := app.initConfig(false)
	require.NoError(t, err)

	results, err := app.checkMediaFormats(false)
	assert.ErrorContains(t, err, "not enabled")
	assert.Nil(t, results)
}

func Test_checkMediaFormats_noVariants(t *testing.T) {
	app, storagePath := setupCheckFormatsTest(t)

	require.NoError(t, os.WriteFile(filepath.Join(storagePath, "abc123.jpg"), []byte("test"), 0644))

	results, err := app.checkMediaFormats(false)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "abc123", results[0].Hash)
	assert.Equal(t, ".jpg", results[0].Extension)

	sort.Strings(results[0].MissingVariants)
	assert.Equal(t, []string{
		"avif_1400", "avif_2000", "avif_800",
		"jpeg_1400", "jpeg_2000", "jpeg_800",
	}, results[0].MissingVariants)
}

func Test_checkMediaFormats_allOptimized(t *testing.T) {
	app, storagePath := setupCheckFormatsTest(t)

	require.NoError(t, os.WriteFile(filepath.Join(storagePath, "abc123.jpg"), []byte("test"), 0644))

	db := app.db
	for _, row := range []*mediaOptimizedRow{
		{OriginalHash: "abc123", VariantType: "avif_800", OptimizedHash: "h1", Width: 800, Height: 600},
		{OriginalHash: "abc123", VariantType: "avif_1400", OptimizedHash: "h2", Width: 1400, Height: 1050},
		{OriginalHash: "abc123", VariantType: "avif_2000", OptimizedHash: "h3", Width: 2000, Height: 1500},
		{OriginalHash: "abc123", VariantType: "jpeg_800", OptimizedHash: "h4", Width: 800, Height: 600},
		{OriginalHash: "abc123", VariantType: "jpeg_1400", OptimizedHash: "h5", Width: 1400, Height: 1050},
		{OriginalHash: "abc123", VariantType: "jpeg_2000", OptimizedHash: "h6", Width: 2000, Height: 1500},
	} {
		require.NoError(t, db.mediaOptimizedInsert(row))
	}

	results, err := app.checkMediaFormats(false)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func Test_checkMediaFormats_missingFormats(t *testing.T) {
	app, storagePath := setupCheckFormatsTest(t)

	require.NoError(t, os.WriteFile(filepath.Join(storagePath, "abc123.jpg"), []byte("test"), 0644))

	db := app.db
	require.NoError(t, db.mediaOptimizedInsert(&mediaOptimizedRow{
		OriginalHash: "abc123", VariantType: "jpeg_2000", OptimizedHash: "h4", Width: 2000, Height: 1500,
	}))

	results, err := app.checkMediaFormats(false)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "abc123", results[0].Hash)

	sort.Strings(results[0].MissingVariants)
	assert.Equal(t, []string{
		"avif_1400", "avif_2000", "avif_800",
		"jpeg_1400", "jpeg_800",
	}, results[0].MissingVariants)
}

func Test_checkMediaFormats_skipsVariants(t *testing.T) {
	app, storagePath := setupCheckFormatsTest(t)

	require.NoError(t, os.WriteFile(filepath.Join(storagePath, "abc123.jpg"), []byte("original"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(storagePath, "def456.avif"), []byte("variant"), 0644))

	db := app.db
	require.NoError(t, db.mediaOptimizedInsert(&mediaOptimizedRow{
		OriginalHash: "abc123", VariantType: "avif_2000", OptimizedHash: "def456", Width: 2000, Height: 1500,
	}))

	results, err := app.checkMediaFormats(false)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "abc123", results[0].Hash)

	sort.Strings(results[0].MissingVariants)
	assert.Equal(t, []string{
		"avif_1400", "avif_800",
		"jpeg_1400", "jpeg_2000", "jpeg_800",
	}, results[0].MissingVariants)
}

func Test_checkMediaFormats_pngSource(t *testing.T) {
	app, storagePath := setupCheckFormatsTest(t)
	app.cfg.MediaOptimization.Formats = []string{"avif", "jpeg", "png"}
	app.initMediaOptimization()

	require.NoError(t, os.WriteFile(filepath.Join(storagePath, "abc123.png"), []byte("test"), 0644))

	db := app.db
	require.NoError(t, db.mediaOptimizedInsert(&mediaOptimizedRow{
		OriginalHash: "abc123", VariantType: "avif_2000", OptimizedHash: "h1", Width: 2000, Height: 1500,
	}))

	results, err := app.checkMediaFormats(false)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "abc123", results[0].Hash)
	assert.Equal(t, ".png", results[0].Extension)

	sort.Strings(results[0].MissingVariants)
	assert.Equal(t, []string{
		"avif_1400", "avif_800",
		"png_1400", "png_2000", "png_800",
	}, results[0].MissingVariants)
}

func Test_checkMediaFormats_multipleImages(t *testing.T) {
	app, storagePath := setupCheckFormatsTest(t)

	require.NoError(t, os.WriteFile(filepath.Join(storagePath, "img1.jpg"), []byte("test"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(storagePath, "img2.jpg"), []byte("test"), 0644))

	db := app.db
	for _, row := range []*mediaOptimizedRow{
		{OriginalHash: "img1", VariantType: "avif_800", OptimizedHash: "h1", Width: 800, Height: 600},
		{OriginalHash: "img1", VariantType: "avif_1400", OptimizedHash: "h2", Width: 1400, Height: 1050},
		{OriginalHash: "img1", VariantType: "avif_2000", OptimizedHash: "h3", Width: 2000, Height: 1500},
		{OriginalHash: "img1", VariantType: "jpeg_800", OptimizedHash: "h4", Width: 800, Height: 600},
		{OriginalHash: "img1", VariantType: "jpeg_1400", OptimizedHash: "h5", Width: 1400, Height: 1050},
		{OriginalHash: "img1", VariantType: "jpeg_2000", OptimizedHash: "h6", Width: 2000, Height: 1500},
	} {
		require.NoError(t, db.mediaOptimizedInsert(row))
	}

	results, err := app.checkMediaFormats(false)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "img2", results[0].Hash)

	sort.Strings(results[0].MissingVariants)
	assert.Equal(t, []string{
		"avif_1400", "avif_2000", "avif_800",
		"jpeg_1400", "jpeg_2000", "jpeg_800",
	}, results[0].MissingVariants)
}

func Test_checkMediaFormats_emptyStorage(t *testing.T) {
	app, _ := setupCheckFormatsTest(t)

	results, err := app.checkMediaFormats(false)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func Test_checkMediaFormats_partialSizes(t *testing.T) {
	app, storagePath := setupCheckFormatsTest(t)

	require.NoError(t, os.WriteFile(filepath.Join(storagePath, "abc123.jpg"), []byte("test"), 0644))

	db := app.db
	for _, row := range []*mediaOptimizedRow{
		{OriginalHash: "abc123", VariantType: "avif_800", OptimizedHash: "h1", Width: 800, Height: 600},
		{OriginalHash: "abc123", VariantType: "avif_2000", OptimizedHash: "h2", Width: 2000, Height: 1500},
		{OriginalHash: "abc123", VariantType: "jpeg_800", OptimizedHash: "h3", Width: 800, Height: 600},
		{OriginalHash: "abc123", VariantType: "jpeg_1400", OptimizedHash: "h4", Width: 1400, Height: 1050},
	} {
		require.NoError(t, db.mediaOptimizedInsert(row))
	}

	results, err := app.checkMediaFormats(false)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "abc123", results[0].Hash)

	sort.Strings(results[0].MissingVariants)
	assert.Equal(t, []string{
		"avif_1400", "jpeg_2000",
	}, results[0].MissingVariants)
}

func Test_checkMediaFormats_hasVariantsOnly(t *testing.T) {
	app, storagePath := setupCheckFormatsTest(t)

	require.NoError(t, os.WriteFile(filepath.Join(storagePath, "abc.jpg"), []byte("test"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(storagePath, "def.jpg"), []byte("test"), 0644))

	db := app.db
	require.NoError(t, db.mediaOptimizedInsert(&mediaOptimizedRow{
		OriginalHash: "abc", VariantType: "avif_2000", OptimizedHash: "h1", Width: 2000, Height: 1500,
	}))

	// Without filter: both images should appear
	results, err := app.checkMediaFormats(false)
	require.NoError(t, err)
	require.Len(t, results, 2)

	// With filter: only abc.jpg (has one variant) should appear, def.jpg (zero variants) is skipped
	results, err = app.checkMediaFormats(true)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "abc", results[0].Hash)
}
