package gpxhelper

import (
	"testing"
	"time"

	"github.com/tkrajina/gpxgo/gpx"
)

func TestMergeGpx_NoArgs(t *testing.T) {
	_, err := MergeGpx()
	if err == nil {
		t.Fatalf("expected error when no GPX files provided")
	}
}

func TestMergeGpx_TwoFiles(t *testing.T) {
	g1 := `<?xml version="1.0" encoding="UTF-8"?>
<gpx version="1.1" creator="test" xmlns="http://www.topografix.com/GPX/1/1">
  <metadata>
    <name>A</name>
    <time>2020-01-02T15:04:05Z</time>
  </metadata>
</gpx>`

	g2 := `<?xml version="1.0" encoding="UTF-8"?>
<gpx version="1.1" creator="test" xmlns="http://www.topografix.com/GPX/1/1">
  <metadata>
    <name>B</name>
    <time>2019-01-01T00:00:00Z</time>
  </metadata>
</gpx>`

	mergedBytes, err := MergeGpx([]byte(g1), []byte(g2))
	if err != nil {
		t.Fatalf("unexpected error from MergeGpx: %v", err)
	}

	parsed, err := gpx.ParseBytes(mergedBytes)
	if err != nil {
		t.Fatalf("failed to parse merged GPX: %v", err)
	}

	if parsed.Name != "A, B" {
		t.Fatalf("unexpected merged name: %q", parsed.Name)
	}

	if parsed.Time == nil {
		t.Fatalf("expected merged time to be set")
	}

	want := time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	if !parsed.Time.Equal(want) {
		t.Fatalf("expected earliest time %v, got %v", want, parsed.Time)
	}
}
