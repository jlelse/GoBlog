package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_migrationSimilarAspectRatio(t *testing.T) {
	tests := []struct {
		name string
		a, b *migrationFile
		want bool
	}{
		{
			name: "identical",
			a:    &migrationFile{Width: 4000, Height: 3000},
			b:    &migrationFile{Width: 800, Height: 600},
			want: true,
		},
		{
			name: "within tolerance",
			a:    &migrationFile{Width: 4000, Height: 3000},
			b:    &migrationFile{Width: 800, Height: 620},
			want: true,
		},
		{
			name: "outside tolerance",
			a:    &migrationFile{Width: 4000, Height: 3000},
			b:    &migrationFile{Width: 800, Height: 800},
			want: false,
		},
		{
			name: "zero height a",
			a:    &migrationFile{Width: 4000, Height: 0},
			b:    &migrationFile{Width: 800, Height: 600},
			want: true,
		},
		{
			name: "zero height b",
			a:    &migrationFile{Width: 4000, Height: 3000},
			b:    &migrationFile{Width: 800, Height: 0},
			want: true,
		},
		{
			name: "portrait vs landscape",
			a:    &migrationFile{Width: 3000, Height: 4000},
			b:    &migrationFile{Width: 600, Height: 800},
			want: true,
		},
		{
			name: "square vs landscape",
			a:    &migrationFile{Width: 3000, Height: 3000},
			b:    &migrationFile{Width: 800, Height: 600},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, migrationSimilarAspectRatio(tt.a, tt.b))
		})
	}
}

func Test_migrationMinDistance(t *testing.T) {
	a := &migrationFile{DHash: 0x00, DHashRaw: 0xff}
	b := &migrationFile{DHash: 0x01, DHashRaw: 0xfe}

	d := migrationMinDistance(a, b)
	assert.Equal(t, 1, d) // min(1, 1)
}

func Test_migrationMinDistance_prefersCloser(t *testing.T) {
	a := &migrationFile{DHash: 0x00, DHashRaw: 0x00}
	b := &migrationFile{DHash: 0x01, DHashRaw: 0xff}

	d := migrationMinDistance(a, b)
	assert.Equal(t, 1, d) // min(1, 64)
}

func Test_migrationIdentifyOriginal_byExif(t *testing.T) {
	files := []*migrationFile{
		{Name: "a.jpg", Width: 800, Height: 600, Bytes: 100000, HasExif: false},
		{Name: "b.jpg", Width: 4000, Height: 3000, Bytes: 5000000, HasExif: true},
	}
	orig, others := migrationIdentifyOriginal(files)
	require.NotNil(t, orig)
	assert.Equal(t, "b.jpg", orig.Name)
	assert.Len(t, others, 1)
	assert.Equal(t, "a.jpg", others[0].Name)
}

func Test_migrationIdentifyOriginal_byDimensions(t *testing.T) {
	files := []*migrationFile{
		{Name: "a.jpg", Width: 800, Height: 600, Bytes: 100000},
		{Name: "b.jpg", Width: 4000, Height: 3000, Bytes: 5000000},
	}
	orig, others := migrationIdentifyOriginal(files)
	require.NotNil(t, orig)
	assert.Equal(t, "b.jpg", orig.Name)
	assert.Len(t, others, 1)
}

func Test_migrationIdentifyOriginal_byBytes(t *testing.T) {
	files := []*migrationFile{
		{Name: "a.jpg", Width: 4000, Height: 3000, Bytes: 100000},
		{Name: "b.jpg", Width: 4000, Height: 3000, Bytes: 5000000},
	}
	orig, others := migrationIdentifyOriginal(files)
	require.NotNil(t, orig)
	assert.Equal(t, "b.jpg", orig.Name)
	assert.Len(t, others, 1)
}

func Test_migrationIdentifyOriginal_tooFew(t *testing.T) {
	files := []*migrationFile{
		{Name: "a.jpg", Width: 800, Height: 600, Bytes: 100000},
	}
	orig, others := migrationIdentifyOriginal(files)
	assert.Nil(t, orig)
	assert.Nil(t, others)
}

func Test_migrationIdentifyOriginal_multipleCompressed(t *testing.T) {
	files := []*migrationFile{
		{Name: "original.jpg", Width: 4000, Height: 3000, Bytes: 5000000, HasExif: true},
		{Name: "compressed1.jpg", Width: 800, Height: 600, Bytes: 50000},
		{Name: "compressed2.jpg", Width: 1200, Height: 900, Bytes: 80000},
	}
	orig, others := migrationIdentifyOriginal(files)
	require.NotNil(t, orig)
	assert.Equal(t, "original.jpg", orig.Name)
	assert.Len(t, others, 2)
}

func Test_migrationHasExif_jpeg(t *testing.T) {
	// EXIF marker in JPEG
	data := []byte{0xff, 0xd8, 0xff, 0xe1, 0x00, 0x00, 'E', 'x', 'i', 'f', 0x00, 0x00}
	assert.True(t, migrationHasExif("jpeg", data))
}

func Test_migrationHasExif_jpegNoExif(t *testing.T) {
	data := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x00, 'J', 'F', 'I', 'F'}
	assert.False(t, migrationHasExif("jpeg", data))
}

func Test_migrationHasExif_png(t *testing.T) {
	data := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 'e', 'X', 'I', 'f'}
	assert.True(t, migrationHasExif("png", data))
}

func Test_migrationHasExif_pngNoExif(t *testing.T) {
	data := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 'I', 'H', 'D', 'R'}
	assert.False(t, migrationHasExif("png", data))
}

func Test_migrationHasExif_unsupportedFormat(t *testing.T) {
	data := []byte("anything")
	assert.False(t, migrationHasExif("gif", data))
}

// Test_migrationCache_roundTrip verifies that a migrationCache survives a
// JSON marshal/unmarshal cycle. This is important because the cache is stored
// as a JSON file on disk between runs. All fields must round-trip correctly
// including zero-value edge cases (e.g. Bytes: 0).
func Test_migrationCache_roundTrip(t *testing.T) {
	orig := &migrationCache{Entries: map[string]*migrationEntry{
		"test.jpg": {
			DHash:    "deadbeef",
			DHashRaw: "cafebabe",
			Width:    800,
			Height:   600,
			Bytes:    12345,
			ModTime:  time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC).Format(time.RFC3339),
			HasExif:  true,
		},
		"other.png": {
			DHash:    "1234567890abcdef",
			DHashRaw: "fedcba0987654321",
			Width:    400,
			Height:   300,
			Bytes:    0,
			ModTime:  time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
			HasExif:  false,
		},
	}}

	data, err := json.Marshal(orig)
	require.NoError(t, err)

	restored := &migrationCache{Entries: map[string]*migrationEntry{}}
	err = json.Unmarshal(data, restored)
	require.NoError(t, err)

	assert.Len(t, restored.Entries, 2)
	assert.Equal(t, orig.Entries["test.jpg"], restored.Entries["test.jpg"])
	assert.Equal(t, orig.Entries["other.png"], restored.Entries["other.png"])
}

func Test_migrationHasExif_jpg(t *testing.T) {
	exifMarker := []byte("Exif\x00\x00")
	data := make([]byte, 0, 20)
	data = append(data, 0xff, 0xd8, 0xff, 0xe1)
	data = append(data, exifMarker...)

	assert.True(t, migrationHasExif("jpg", data), "jpg with EXIF marker should be detected")
	assert.True(t, migrationHasExif("jpeg", data), "jpeg with EXIF marker should be detected")
	assert.False(t, migrationHasExif("png", data), "png format should not match JPEG EXIF")
	assert.False(t, migrationHasExif("jpg", []byte{0xff, 0xd8, 0xff, 0xe0}), "JPEG without EXIF should return false")
}
