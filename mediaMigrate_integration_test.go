package main

import (
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.goblog.app/app/pkgs/bufferpool"
)

// --- Helpers ---

// createTestImage returns a solid-color image of the given dimensions.
// Useful when tests need images that are distinguishable by size but not by content.
func createTestImage(w, h int, c color.Color) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, c)
		}
	}
	return img
}

// createGradientImage returns a left-to-right red + top-to-bottom green gradient.
// Produces a consistent dHash across re-encodes (unlike solid colors that can
// produce different hashes after JPEG compression due to rounding).
func createGradientImage(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			r := uint8(x * 255 / w)
			g := uint8(y * 255 / h)
			img.Set(x, y, color.RGBA{R: r, G: g, B: 128, A: 255})
		}
	}
	return img
}

// createDiagonalGradientImage returns a diagonal gradient where each channel
// varies along a different axis (R: diagonal, G: horizontal, B: vertical).
// The asymmetry is essential for EXIF orientation tests: when the image is
// rotated 90° (orientation 6), the dHash changes, proving that the hash
// function respects EXIF orientation.
func createDiagonalGradientImage(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			r := uint8((x + y) * 255 / (w + h))
			g := uint8(x * 255 / w)
			b := uint8(y * 255 / h)
			img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}
	return img
}

func writeTestJPEG(t *testing.T, dir, name string, img image.Image) {
	t.Helper()
	buf := bufferpool.Get()
	defer bufferpool.Put(buf)
	require.NoError(t, jpeg.Encode(buf, img, &jpeg.Options{Quality: 80}))
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), buf.Bytes(), 0644))
}

func writeTestPNG(t *testing.T, dir, name string, img image.Image) {
	t.Helper()
	buf := bufferpool.Get()
	defer bufferpool.Put(buf)
	require.NoError(t, png.Encode(buf, img))
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), buf.Bytes(), 0644))
}

// writeTestJPEGWithExifOrientation encodes img as JPEG, then injects an EXIF
// APP1 marker with the given orientation tag. This is needed because Go's
// standard jpeg encoder does not write EXIF data. The function splices the
// EXIF segment between the SOI marker (0xFFD8) and the rest of the JPEG data.
func writeTestJPEGWithExifOrientation(t *testing.T, dir, name string, img image.Image, orientation uint16) {
	t.Helper()
	buf := bufferpool.Get()
	defer bufferpool.Put(buf)
	require.NoError(t, jpeg.Encode(buf, img, &jpeg.Options{Quality: 80}))
	jpegData := buf.Bytes()
	require.Greater(t, len(jpegData), 2)
	require.Equal(t, byte(0xff), jpegData[0])
	require.Equal(t, byte(0xd8), jpegData[1])
	exifData := buildExifAPP1(orientation)
	result := make([]byte, 0, len(exifData)+len(jpegData))
	result = append(result, 0xff, 0xd8)
	result = append(result, exifData...)
	result = append(result, jpegData[2:]...)
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), result, 0644))
}

// buildExifAPP1 constructs a minimal EXIF APP1 segment containing only an
// Orientation IFD entry. The binary layout is:
//
//	APP1 marker (0xFFE1) + length + "Exif\0\0" + TIFF header + IFD0 with one entry
//
// Big-endian byte order is used throughout. Only the orientation tag (0x0112)
// is written — enough for migrationHasExif and migrationHashFile to detect it.
func buildExifAPP1(orientation uint16) []byte {
	ifdEntries := []byte{
		0x01, 0x12, // tag: orientation
		0x00, 0x03, // type: SHORT
		0x00, 0x00, 0x00, 0x01, // count: 1
		byte(orientation >> 8), byte(orientation), 0x00, 0x00, // value
	}
	tiffHeader := []byte{
		'M', 'M', // big-endian
		0x00, 0x2A, // magic
		0x00, 0x00, 0x00, 0x08, // IFD0 offset
	}
	ifd0 := []byte{
		0x00, 0x01, // 1 entry
	}
	ifd0 = append(ifd0, ifdEntries...)
	ifd0 = append(ifd0, 0x00, 0x00, 0x00, 0x00) // next IFD offset
	exifContent := append(tiffHeader, ifd0...)
	exifHeader := []byte{'E', 'x', 'i', 'f', 0x00, 0x00}
	payload := append(exifHeader, exifContent...)
	app1 := make([]byte, 2)
	binary.BigEndian.PutUint16(app1, uint16(2+len(payload)))
	app1 = append(app1, payload...)
	return append([]byte{0xff, 0xe1}, app1...)
}

func mustParseUint64(t *testing.T, s string) uint64 {
	t.Helper()
	v, err := strconv.ParseUint(s, 16, 64)
	require.NoError(t, err)
	return v
}

// --- Tests ---

func Test_mediaMigrate_noStorage(t *testing.T) {
	app := &goBlog{cfg: createDefaultTestConfig(t)}
	app.mediaMigrate(&migrationConfig{yes: true})
}

func Test_mediaMigrate_noGroupsFound(t *testing.T) {
	storagePath := t.TempDir()
	storage := &localMediaStorage{path: storagePath}
	app := newAppWithStorage(t, storage)
	require.NoError(t, app.initConfig(false))

	img := createTestImage(4000, 3000, color.RGBA{R: 100, G: 150, B: 200, A: 255})
	writeTestJPEG(t, storagePath, "single.jpg", img)

	app.mediaMigrate(&migrationConfig{yes: true})

	_, err := os.Stat(filepath.Join(storagePath, "single.jpg"))
	assert.NoError(t, err, "single file should still exist when no groups found")
}

func Test_mediaMigrate_corruptImageSkipped(t *testing.T) {
	storagePath := t.TempDir()
	storage := &localMediaStorage{path: storagePath}
	app := newAppWithStorage(t, storage)
	require.NoError(t, app.initConfig(false))

	img := createTestImage(4000, 3000, color.RGBA{R: 100, G: 150, B: 200, A: 255})
	writeTestJPEG(t, storagePath, "abc123.jpg", img)
	require.NoError(t, os.WriteFile(filepath.Join(storagePath, "corrupt.jpg"), []byte("not a real image"), 0644))

	app.mediaMigrate(&migrationConfig{yes: true})

	_, err := os.Stat(filepath.Join(storagePath, "corrupt.jpg"))
	assert.NoError(t, err, "corrupt file should still exist (skipped, not grouped)")
}

func Test_mediaMigrate_dryRun(t *testing.T) {
	storagePath := t.TempDir()
	storage := &localMediaStorage{path: storagePath}
	app := newAppWithStorage(t, storage)
	require.NoError(t, app.initConfig(false))

	img := createTestImage(4000, 3000, color.RGBA{R: 100, G: 150, B: 200, A: 255})
	writeTestJPEG(t, storagePath, "abc123.jpg", img)
	writeTestJPEG(t, storagePath, "def456.jpg", img)

	require.NoError(t, app.db.savePost(&post{
		Path:      "/test/dryrun",
		Content:   "Image: ![photo](/m/def456.jpg)",
		Published: toLocalSafe("2024-01-15T10:00:00Z"),
		Updated:   toLocalSafe("2024-01-15T10:00:00Z"),
		Status:    statusPublished,
		Parameters: map[string][]string{
			"title": {"Dry run test"},
		},
	}, &postCreationOptions{isNew: true}))

	app.mediaMigrate(&migrationConfig{yes: true, dryRun: true, threshold: 6})

	_, err := os.Stat(filepath.Join(storagePath, "def456.jpg"))
	assert.NoError(t, err, "compressed file should still exist after dry run")
	_, err = os.Stat(filepath.Join(storagePath, "abc123.jpg"))
	assert.NoError(t, err, "original should still exist after dry run")

	p, err := app.getPost("/test/dryrun")
	require.NoError(t, err)
	assert.Contains(t, p.Content, "/m/def456.jpg", "post should still reference compressed file after dry run")
}

func Test_mediaMigrate_discoverOnly(t *testing.T) {
	storagePath := t.TempDir()
	storage := &localMediaStorage{path: storagePath}
	app := newAppWithStorage(t, storage)
	require.NoError(t, app.initConfig(false))

	originalImg := createTestImage(4000, 3000, color.RGBA{R: 100, G: 150, B: 200, A: 255})
	compressedImg := createTestImage(800, 600, color.RGBA{R: 100, G: 150, B: 200, A: 255})
	writeTestJPEG(t, storagePath, "abc123.jpg", originalImg)
	writeTestJPEG(t, storagePath, "def456.jpg", compressedImg)
	writeTestPNG(t, storagePath, "jkl012.png", compressedImg)

	app.mediaMigrate(&migrationConfig{discoverOnly: true, threshold: 6})

	_, err := os.Stat(filepath.Join(storagePath, "def456.jpg"))
	assert.NoError(t, err, "compressed file should still exist after discover-only")
	_, err = os.Stat(filepath.Join(storagePath, "jkl012.png"))
	assert.NoError(t, err, "compressed PNG should still exist after discover-only")
}

func Test_mediaMigrate_fullMigration(t *testing.T) {
	storagePath := t.TempDir()
	storage := &localMediaStorage{path: storagePath}
	app := newAppWithStorage(t, storage)
	require.NoError(t, app.initConfig(false))

	originalImg := createTestImage(4000, 3000, color.RGBA{R: 100, G: 150, B: 200, A: 255})
	compressedImg := createTestImage(800, 600, color.RGBA{R: 100, G: 150, B: 200, A: 255})
	writeTestJPEG(t, storagePath, "abc123.jpg", originalImg)
	writeTestJPEG(t, storagePath, "def456.jpg", compressedImg)
	writeTestPNG(t, storagePath, "jkl012.png", compressedImg)

	now := toLocalSafe("2024-01-15T10:00:00Z")
	require.NoError(t, app.db.savePost(&post{
		Path:      "/test/post1",
		Content:   "Here is an image: ![photo](/m/def456.jpg)",
		Published: now,
		Updated:   now,
		Status:    statusPublished,
		Parameters: map[string][]string{
			"title": {"Post with compressed image"},
		},
	}, &postCreationOptions{isNew: true}))
	require.NoError(t, app.db.savePost(&post{
		Path:      "/test/post2",
		Content:   "Another image: ![photo](/m/jkl012.png)",
		Published: now,
		Updated:   now,
		Status:    statusPublished,
		Parameters: map[string][]string{
			"title": {"Post with compressed PNG"},
			"photos": {"/m/jkl012.png"},
		},
	}, &postCreationOptions{isNew: true}))
	require.NoError(t, app.db.savePost(&post{
		Path:      "/test/post3",
		Content:   "No image in content",
		Published: now,
		Updated:   now,
		Status:    statusPublished,
		Parameters: map[string][]string{
			"title":   {"Post with partial URL in param"},
			"summary": {"Look at this: /m/def456.jpg and more text"},
			"photos":  {"other.jpg", "/m/def456.jpg", "another.png"},
		},
	}, &postCreationOptions{isNew: true}))

	app.mediaMigrate(&migrationConfig{yes: true, threshold: 6})

	_, err := os.Stat(filepath.Join(storagePath, "abc123.jpg"))
	assert.NoError(t, err, "original should still exist")
	_, err = os.Stat(filepath.Join(storagePath, "def456.jpg"))
	assert.ErrorIs(t, err, os.ErrNotExist, "compressed JPG should be deleted")
	_, err = os.Stat(filepath.Join(storagePath, "jkl012.png"))
	assert.ErrorIs(t, err, os.ErrNotExist, "compressed PNG should be deleted")

	p1, err := app.getPost("/test/post1")
	require.NoError(t, err)
	assert.Contains(t, p1.Content, "/m/abc123.jpg")
	assert.NotContains(t, p1.Content, "/m/def456.jpg")

	p2, err := app.getPost("/test/post2")
	require.NoError(t, err)
	assert.Contains(t, p2.Content, "/m/abc123.jpg")
	assert.NotContains(t, p2.Content, "/m/jkl012.png")
	assert.Contains(t, p2.Parameters["photos"], "/m/abc123.jpg")

	p3, err := app.getPost("/test/post3")
	require.NoError(t, err)
	assert.Contains(t, p3.Parameters["summary"], "Look at this: /m/abc123.jpg and more text")
	assert.Equal(t, []string{"other.jpg", "/m/abc123.jpg", "another.png"}, p3.Parameters["photos"])
}

func Test_mediaMigrate_crossExtension(t *testing.T) {
	storagePath := t.TempDir()
	storage := &localMediaStorage{path: storagePath}
	app := newAppWithStorage(t, storage)
	require.NoError(t, app.initConfig(false))

	img := createTestImage(4000, 3000, color.RGBA{R: 100, G: 150, B: 200, A: 255})
	writeTestJPEG(t, storagePath, "abc123.jpg", img)
	writeTestPNG(t, storagePath, "def456.png", img)

	require.NoError(t, app.db.savePost(&post{
		Path:      "/test/cross-ext",
		Content:   "Image: ![photo](/m/def456.png)",
		Published: toLocalSafe("2024-01-15T10:00:00Z"),
		Updated:   toLocalSafe("2024-01-15T10:00:00Z"),
		Status:    statusPublished,
		Parameters: map[string][]string{
			"title": {"Cross-extension test"},
		},
	}, &postCreationOptions{isNew: true}))

	app.mediaMigrate(&migrationConfig{yes: true, threshold: 6})

	_, err := os.Stat(filepath.Join(storagePath, "abc123.jpg"))
	assert.NoError(t, err, "original JPG should still exist")
	_, err = os.Stat(filepath.Join(storagePath, "def456.png"))
	assert.ErrorIs(t, err, os.ErrNotExist, "compressed PNG should be deleted")

	p, err := app.getPost("/test/cross-ext")
	require.NoError(t, err)
	assert.Contains(t, p.Content, "/m/abc123.jpg")
	assert.NotContains(t, p.Content, "/m/def456.png")
}

// Test_mediaMigrate_skipsAlreadyOptimized verifies that files whose original
// already has optimized variants in the media_optimized table are skipped.
// This prevents re-migration of files that have already been processed and
// had their quality/compression variants generated.
func Test_mediaMigrate_skipsAlreadyOptimized(t *testing.T) {
	storagePath := t.TempDir()
	storage := &localMediaStorage{path: storagePath}
	app := newAppWithStorage(t, storage)
	require.NoError(t, app.initConfig(false))

	img := createTestImage(4000, 3000, color.RGBA{R: 100, G: 150, B: 200, A: 255})
	writeTestJPEG(t, storagePath, "abc123.jpg", img)
	writeTestJPEG(t, storagePath, "def456.jpg", img)

	require.NoError(t, app.db.mediaOptimizedInsert(&mediaOptimizedRow{
		OriginalHash:  "abc123",
		VariantType:   "avif_800",
		OptimizedHash: "opt1",
		Width:         800,
		Height:        600,
	}))

	require.NoError(t, app.db.savePost(&post{
		Path:      "/test/skip-opt",
		Content:   "Image: ![photo](/m/def456.jpg)",
		Published: toLocalSafe("2024-01-15T10:00:00Z"),
		Updated:   toLocalSafe("2024-01-15T10:00:00Z"),
		Status:    statusPublished,
		Parameters: map[string][]string{
			"title": {"Skip optimized test"},
		},
	}, &postCreationOptions{isNew: true}))

	app.mediaMigrate(&migrationConfig{yes: true, threshold: 6})

	_, err := os.Stat(filepath.Join(storagePath, "abc123.jpg"))
	assert.NoError(t, err, "original should still exist")
	_, err = os.Stat(filepath.Join(storagePath, "def456.jpg"))
	assert.NoError(t, err, "compressed file should not be deleted when original already optimized")

	p, err := app.getPost("/test/skip-opt")
	require.NoError(t, err)
	assert.Contains(t, p.Content, "/m/def456.jpg", "post should be unchanged")
}

// Test_mediaMigrate_limit verifies that the --limit flag restricts how many
// groups are processed. Two groups are created with different modtimes so the
// processing order is deterministic (newer group first). With limit=1, exactly
// one file is deleted and one post is updated. Different-sized images are used
// (4000x3000 vs 800x600) so migrationIdentifyOriginal can deterministically
// pick the original by dimensions.
func Test_mediaMigrate_limit(t *testing.T) {
	storagePath := t.TempDir()
	storage := &localMediaStorage{path: storagePath}
	app := newAppWithStorage(t, storage)
	require.NoError(t, app.initConfig(false))

	img1 := createTestImage(4000, 3000, color.RGBA{R: 100, G: 150, B: 200, A: 255})
	img2 := createGradientImage(4000, 3000)
	img1Small := createTestImage(800, 600, color.RGBA{R: 100, G: 150, B: 200, A: 255})
	img2Small := createGradientImage(800, 600)
	writeTestJPEG(t, storagePath, "orig1.jpg", img1)
	writeTestJPEG(t, storagePath, "orig2.jpg", img2)
	writeTestJPEG(t, storagePath, "comp1.jpg", img1Small)
	writeTestJPEG(t, storagePath, "comp2.jpg", img2Small)

	older := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, os.Chtimes(filepath.Join(storagePath, "orig1.jpg"), older, older))
	require.NoError(t, os.Chtimes(filepath.Join(storagePath, "comp1.jpg"), older, older))
	require.NoError(t, os.Chtimes(filepath.Join(storagePath, "orig2.jpg"), newer, newer))
	require.NoError(t, os.Chtimes(filepath.Join(storagePath, "comp2.jpg"), newer, newer))

	now := toLocalSafe("2024-01-15T10:00:00Z")
	require.NoError(t, app.db.savePost(&post{
		Path:      "/test/limit1",
		Content:   "Image: ![photo](/m/comp1.jpg)",
		Published: now,
		Updated:   now,
		Status:    statusPublished,
		Parameters: map[string][]string{
			"title": {"Limit test 1"},
		},
	}, &postCreationOptions{isNew: true}))
	require.NoError(t, app.db.savePost(&post{
		Path:      "/test/limit2",
		Content:   "Image: ![photo](/m/comp2.jpg)",
		Published: now,
		Updated:   now,
		Status:    statusPublished,
		Parameters: map[string][]string{
			"title": {"Limit test 2"},
		},
	}, &postCreationOptions{isNew: true}))

	app.mediaMigrate(&migrationConfig{yes: true, limit: 1, threshold: 6})

	files := []string{"orig1.jpg", "comp1.jpg", "orig2.jpg", "comp2.jpg"}
	deleted := 0
	for _, f := range files {
		if _, err := os.Stat(filepath.Join(storagePath, f)); os.IsNotExist(err) {
			deleted++
		}
	}
	assert.Equal(t, 1, deleted, "exactly one file should be deleted with limit=1")

	// The newer group should be processed first (sorted by original modtime desc).
	// One post should reference the original of its group, the other should be untouched.
	p1, err := app.getPost("/test/limit1")
	require.NoError(t, err)
	p2, err := app.getPost("/test/limit2")
	require.NoError(t, err)

	changed1 := p1.Content != "Image: ![photo](/m/comp1.jpg)"
	changed2 := p2.Content != "Image: ![photo](/m/comp2.jpg)"
	assert.True(t, changed1 || changed2, "at least one post should have been updated")
	assert.False(t, changed1 && changed2, "only one group should be processed with limit=1")
}

// Test_mediaMigrate_fullURLReplacement verifies that posts referencing files
// by full URL (e.g. https://example.com/media/hash.jpg) are correctly updated
// during migration, not just bare filenames.
func Test_mediaMigrate_fullURLReplacement(t *testing.T) {
	storagePath := t.TempDir()
	storage := &localMediaStorage{path: storagePath}
	app := newAppWithStorage(t, storage)
	require.NoError(t, app.initConfig(false))

	img := createTestImage(4000, 3000, color.RGBA{R: 100, G: 150, B: 200, A: 255})
	imgSmall := createTestImage(800, 600, color.RGBA{R: 100, G: 150, B: 200, A: 255})
	writeTestJPEG(t, storagePath, "orig.jpg", img)
	writeTestJPEG(t, storagePath, "comp.jpg", imgSmall)

	now := toLocalSafe("2024-01-15T10:00:00Z")
	// Post references file by full URL
	require.NoError(t, app.db.savePost(&post{
		Path:      "/test/fullurl",
		Content:   "![photo](http://localhost:8080/m/comp.jpg)",
		Published: now,
		Updated:   now,
		Status:    statusPublished,
		Parameters: map[string][]string{
			"title":  {"Full URL test"},
			"params": {"http://localhost:8080/m/comp.jpg"},
		},
	}, &postCreationOptions{isNew: true}))

	app.mediaMigrate(&migrationConfig{yes: true, threshold: 6})

	p, err := app.getPost("/test/fullurl")
	require.NoError(t, err)
	assert.Contains(t, p.Content, "http://localhost:8080/m/orig.jpg")
	assert.NotContains(t, p.Content, "http://localhost:8080/m/comp.jpg")
	assert.Contains(t, p.Parameters["params"][0], "http://localhost:8080/m/orig.jpg")
	assert.NotContains(t, p.Parameters["params"][0], "http://localhost:8080/m/comp.jpg")
}

// Test_mediaMigrate_preservesTimestamp verifies that migration does not update
// the Updated timestamp of posts when replacing file references.
func Test_mediaMigrate_preservesTimestamp(t *testing.T) {
	storagePath := t.TempDir()
	storage := &localMediaStorage{path: storagePath}
	app := newAppWithStorage(t, storage)
	require.NoError(t, app.initConfig(false))

	img := createTestImage(4000, 3000, color.RGBA{R: 100, G: 150, B: 200, A: 255})
	imgSmall := createTestImage(800, 600, color.RGBA{R: 100, G: 150, B: 200, A: 255})
	writeTestJPEG(t, storagePath, "orig.jpg", img)
	writeTestJPEG(t, storagePath, "comp.jpg", imgSmall)

	pastTime := toLocalSafe("2020-06-15T12:00:00Z")
	require.NoError(t, app.db.savePost(&post{
		Path:      "/test/timestamp",
		Content:   "![photo](/m/comp.jpg)",
		Published: pastTime,
		Updated:   pastTime,
		Status:    statusPublished,
		Parameters: map[string][]string{
			"title": {"Timestamp test"},
		},
	}, &postCreationOptions{isNew: true}))

	app.mediaMigrate(&migrationConfig{yes: true, threshold: 6})

	p, err := app.getPost("/test/timestamp")
	require.NoError(t, err)
	assert.Equal(t, pastTime, p.Published, "Published timestamp should be preserved")
	assert.Equal(t, pastTime, p.Updated, "Updated timestamp should be preserved")
	assert.Contains(t, p.Content, "/m/orig.jpg")
}

// Test_mediaMigrate_exifOrientation verifies that the hash function handles
// 1. "hashes differ" — a normal image and an EXIF-rotated image produce
//    different DHash vs DHashRaw values, proving EXIF is respected.
// 2. "migrationMinDistance matches" — despite different hashes, the min-distance
//    function selects the closer of the two hashes, allowing the pair to group.
func Test_mediaMigrate_exifOrientation(t *testing.T) {
	storagePath := t.TempDir()
	storage := &localMediaStorage{path: storagePath}
	app := newAppWithStorage(t, storage)
	require.NoError(t, app.initConfig(false))

	img := createDiagonalGradientImage(400, 300)
	writeTestJPEG(t, storagePath, "normal.jpg", img)
	writeTestJPEGWithExifOrientation(t, storagePath, "exif_rotated.jpg", img, 6)

	t.Run("hashes differ when exif orientation present", func(t *testing.T) {
		normalEntry, err := app.migrationHashFile(&mediaFile{Name: "normal.jpg"})
		require.NoError(t, err)
		exifEntry, err := app.migrationHashFile(&mediaFile{Name: "exif_rotated.jpg"})
		require.NoError(t, err)

		assert.False(t, normalEntry.HasExif, "normal image should have no EXIF")
		assert.True(t, exifEntry.HasExif, "EXIF image should be detected (regression: .jpg extension)")
		assert.Equal(t, normalEntry.DHash, normalEntry.DHashRaw,
			"normal image: oriented and raw hashes should be identical")
		assert.NotEqual(t, exifEntry.DHash, exifEntry.DHashRaw,
			"EXIF-oriented image: oriented and raw hashes should differ")
	})

	t.Run("migrationMinDistance matches across exif boundary", func(t *testing.T) {
		normalEntry, err := app.migrationHashFile(&mediaFile{Name: "normal.jpg"})
		require.NoError(t, err)
		exifEntry, err := app.migrationHashFile(&mediaFile{Name: "exif_rotated.jpg"})
		require.NoError(t, err)

		normalFile := &migrationFile{
			DHash:    mustParseUint64(t, normalEntry.DHash),
			DHashRaw: mustParseUint64(t, normalEntry.DHashRaw),
			Width:    normalEntry.Width,
			Height:   normalEntry.Height,
		}
		exifFile := &migrationFile{
			DHash:    mustParseUint64(t, exifEntry.DHash),
			DHashRaw: mustParseUint64(t, exifEntry.DHashRaw),
			Width:    exifEntry.Width,
			Height:   exifEntry.Height,
		}

		dist := migrationMinDistance(normalFile, exifFile)
		assert.LessOrEqual(t, dist, 6,
			"min distance of same image should match within threshold (distance=%d)", dist)
	})
}

// Test_mediaMigrate_exifOriginalPreferred verifies that EXIF presence takes
// priority over larger dimensions when selecting the original. A small image
// WITH EXIF should be preferred over a larger image WITHOUT EXIF, because
// EXIF indicates the file was likely the camera output (not a re-encoded copy).
func Test_mediaMigrate_exifOriginalPreferred(t *testing.T) {
	storagePath := t.TempDir()
	storage := &localMediaStorage{path: storagePath}
	app := newAppWithStorage(t, storage)
	require.NoError(t, app.initConfig(false))

	img := createDiagonalGradientImage(400, 300)
	writeTestJPEGWithExifOrientation(t, storagePath, "exif_small.jpg", img, 1)
	largeImg := createDiagonalGradientImage(800, 600)
	writeTestJPEG(t, storagePath, "large_noexif.jpg", largeImg)

	require.NoError(t, app.db.savePost(&post{
		Path:      "/test/exif-pref",
		Content:   "Image: ![photo](/m/large_noexif.jpg)",
		Published: toLocalSafe("2024-01-15T10:00:00Z"),
		Updated:   toLocalSafe("2024-01-15T10:00:00Z"),
		Status:    statusPublished,
		Parameters: map[string][]string{
			"title": {"EXIF preference test"},
		},
	}, &postCreationOptions{isNew: true}))

	app.mediaMigrate(&migrationConfig{yes: true, threshold: 6})

	_, err := os.Stat(filepath.Join(storagePath, "exif_small.jpg"))
	assert.NoError(t, err, "EXIF file should be kept as original despite smaller dimensions")
	_, err = os.Stat(filepath.Join(storagePath, "large_noexif.jpg"))
	assert.ErrorIs(t, err, os.ErrNotExist, "larger file without EXIF should be deleted")

	p, err := app.getPost("/test/exif-pref")
	require.NoError(t, err)
	assert.Contains(t, p.Content, "/m/exif_small.jpg")
}

func Test_mediaMigrate_multiplePostsSameFile(t *testing.T) {
	storagePath := t.TempDir()
	storage := &localMediaStorage{path: storagePath}
	app := newAppWithStorage(t, storage)
	require.NoError(t, app.initConfig(false))

	originalImg := createTestImage(4000, 3000, color.RGBA{R: 100, G: 150, B: 200, A: 255})
	compressedImg := createTestImage(800, 600, color.RGBA{R: 100, G: 150, B: 200, A: 255})
	writeTestJPEG(t, storagePath, "abc123.jpg", originalImg)
	writeTestJPEG(t, storagePath, "def456.jpg", compressedImg)

	now := toLocalSafe("2024-01-15T10:00:00Z")
	require.NoError(t, app.db.savePost(&post{
		Path:      "/test/multi-a",
		Content:   "First: ![photo](/m/def456.jpg)",
		Published: now,
		Updated:   now,
		Status:    statusPublished,
		Parameters: map[string][]string{
			"title": {"Post A"},
		},
	}, &postCreationOptions{isNew: true}))
	require.NoError(t, app.db.savePost(&post{
		Path:      "/test/multi-b",
		Content:   "Second: ![photo](/m/def456.jpg)",
		Published: now,
		Updated:   now,
		Status:    statusPublished,
		Parameters: map[string][]string{
			"title": {"Post B"},
		},
	}, &postCreationOptions{isNew: true}))

	app.mediaMigrate(&migrationConfig{yes: true, threshold: 6})

	p1, err := app.getPost("/test/multi-a")
	require.NoError(t, err)
	assert.Contains(t, p1.Content, "/m/abc123.jpg")
	assert.NotContains(t, p1.Content, "/m/def456.jpg")

	p2, err := app.getPost("/test/multi-b")
	require.NoError(t, err)
	assert.Contains(t, p2.Content, "/m/abc123.jpg")
	assert.NotContains(t, p2.Content, "/m/def456.jpg")
}

func Test_mediaMigrate_postWithNoReferences(t *testing.T) {
	storagePath := t.TempDir()
	storage := &localMediaStorage{path: storagePath}
	app := newAppWithStorage(t, storage)
	require.NoError(t, app.initConfig(false))

	originalImg := createTestImage(4000, 3000, color.RGBA{R: 100, G: 150, B: 200, A: 255})
	compressedImg := createTestImage(800, 600, color.RGBA{R: 100, G: 150, B: 200, A: 255})
	writeTestJPEG(t, storagePath, "abc123.jpg", originalImg)
	writeTestJPEG(t, storagePath, "def456.jpg", compressedImg)

	now := toLocalSafe("2024-01-15T10:00:00Z")
	require.NoError(t, app.db.savePost(&post{
		Path:      "/test/no-refs",
		Content:   "No images here at all, just text.",
		Published: now,
		Updated:   now,
		Status:    statusPublished,
		Parameters: map[string][]string{
			"title":   {"Unrelated post"},
			"summary": {"some value without any filenames"},
		},
	}, &postCreationOptions{isNew: true}))

	app.mediaMigrate(&migrationConfig{yes: true, threshold: 6})

	p, err := app.getPost("/test/no-refs")
	require.NoError(t, err)
	assert.Equal(t, "No images here at all, just text.", p.Content)
	assert.Equal(t, map[string][]string{
		"title":   {"Unrelated post"},
		"summary": {"some value without any filenames"},
	}, p.Parameters)
}

func Test_mediaMigrate_corruptWithValidImages(t *testing.T) {
	storagePath := t.TempDir()
	storage := &localMediaStorage{path: storagePath}
	app := newAppWithStorage(t, storage)
	require.NoError(t, app.initConfig(false))

	originalImg := createTestImage(4000, 3000, color.RGBA{R: 100, G: 150, B: 200, A: 255})
	compressedImg := createTestImage(800, 600, color.RGBA{R: 100, G: 150, B: 200, A: 255})
	writeTestJPEG(t, storagePath, "abc123.jpg", originalImg)
	writeTestJPEG(t, storagePath, "def456.jpg", compressedImg)
	require.NoError(t, os.WriteFile(filepath.Join(storagePath, "corrupt.jpg"), []byte("not a real image"), 0644))

	require.NoError(t, app.db.savePost(&post{
		Path:      "/test/corrupt-group",
		Content:   "Image: ![photo](/m/def456.jpg)",
		Published: toLocalSafe("2024-01-15T10:00:00Z"),
		Updated:   toLocalSafe("2024-01-15T10:00:00Z"),
		Status:    statusPublished,
		Parameters: map[string][]string{
			"title": {"Corrupt group test"},
		},
	}, &postCreationOptions{isNew: true}))

	app.mediaMigrate(&migrationConfig{yes: true, threshold: 6})

	_, err := os.Stat(filepath.Join(storagePath, "abc123.jpg"))
	assert.NoError(t, err, "original should still exist")
	_, err = os.Stat(filepath.Join(storagePath, "def456.jpg"))
	assert.ErrorIs(t, err, os.ErrNotExist, "compressed should be deleted")
	_, err = os.Stat(filepath.Join(storagePath, "corrupt.jpg"))
	assert.NoError(t, err, "corrupt file should be untouched")

	p, err := app.getPost("/test/corrupt-group")
	require.NoError(t, err)
	assert.Contains(t, p.Content, "/m/abc123.jpg")
	assert.NotContains(t, p.Content, "/m/def456.jpg")
}
