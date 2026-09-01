package gpx

import (
	"bytes"
	"encoding/xml"
	"strings"
	"testing"
	"time"
)

func mergeGpxBytes(gpxFiles ...[]byte) ([]byte, error) {
	var buf bytes.Buffer
	err := MergeGpx(&buf, gpxFiles...)
	return buf.Bytes(), err
}

type mergedGpx struct {
	Metadata struct {
		Name string `xml:"name"`
		Time string `xml:"time"`
	} `xml:"metadata"`
	Tracks    []xml.Name `xml:"trk"`
	Routes    []xml.Name `xml:"rte"`
	Waypoints []xml.Name `xml:"wpt"`
}

func TestMergeGpx_NoArgs(t *testing.T) {
	if _, err := mergeGpxBytes(); err == nil {
		t.Fatal("expected error when no GPX files provided")
	}
}

func TestMergeGpx_NotAGpxFile(t *testing.T) {
	if _, err := mergeGpxBytes([]byte("<html></html>")); err == nil {
		t.Fatal("expected error for non-GPX input")
	}
	if _, err := mergeGpxBytes([]byte("not xml at all")); err == nil {
		t.Fatal("expected error for invalid XML input")
	}
}

func TestMergeGpx_TwoFiles(t *testing.T) {
	g1 := `<?xml version="1.0" encoding="UTF-8"?>
<gpx version="1.1" creator="test" xmlns="http://www.topografix.com/GPX/1/1">
  <metadata>
    <name>A</name>
    <time>2020-01-02T15:04:05Z</time>
  </metadata>
  <trk><name>t1</name><trkseg>
    <trkpt lat="52.0" lon="13.0"><ele>100</ele><time>2020-01-02T15:04:05Z</time></trkpt>
  </trkseg></trk>
</gpx>`

	g2 := `<?xml version="1.0" encoding="UTF-8"?>
<gpx version="1.1" creator="test" xmlns="http://www.topografix.com/GPX/1/1">
  <metadata>
    <name>B</name>
    <time>2019-01-01T00:00:00Z</time>
  </metadata>
  <trk><name>t2</name><trkseg>
    <trkpt lat="52.1" lon="13.1"><ele>110</ele><time>2019-01-01T00:00:00Z</time></trkpt>
  </trkseg></trk>
  <wpt lat="52.2" lon="13.2"><name>WP</name></wpt>
</gpx>`

	mergedBytes, err := mergeGpxBytes([]byte(g1), []byte(g2))
	if err != nil {
		t.Fatalf("unexpected error from MergeGpx: %v", err)
	}

	var parsed mergedGpx
	if err := xml.Unmarshal(mergedBytes, &parsed); err != nil {
		t.Fatalf("failed to parse merged GPX: %v", err)
	}

	if parsed.Metadata.Name != "A, B" {
		t.Fatalf("unexpected merged name: %q", parsed.Metadata.Name)
	}

	if len(parsed.Tracks) != 2 {
		t.Fatalf("expected 2 tracks, got %d", len(parsed.Tracks))
	}
	if len(parsed.Waypoints) != 1 {
		t.Fatalf("expected 1 waypoint, got %d", len(parsed.Waypoints))
	}

	want := time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	got, err := time.Parse(time.RFC3339, parsed.Metadata.Time)
	if err != nil {
		t.Fatalf("invalid merged time: %v", err)
	}
	if !got.Equal(want) {
		t.Fatalf("expected earliest time %v, got %v", want, got)
	}

	if !strings.HasPrefix(string(mergedBytes), `<?xml`) || !strings.Contains(string(mergedBytes), `</gpx>`) {
		t.Fatalf("unexpected merged output: %s", mergedBytes)
	}
}

func TestMergeGpx_PrefixedNamespace(t *testing.T) {
	g1 := `<?xml version="1.0"?>
<g:gpx xmlns:g="http://www.topografix.com/GPX/1/1" version="1.1">
  <g:metadata><g:name>Prefixed</g:name></g:metadata>
  <g:trk><g:trkseg>
    <g:trkpt lat="51.5" lon="7.5"><g:ele>50</g:ele></g:trkpt>
  </g:trkseg></g:trk>
</g:gpx>`

	mergedBytes, err := mergeGpxBytes([]byte(g1))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed mergedGpx
	if err := xml.Unmarshal(mergedBytes, &parsed); err != nil {
		t.Fatalf("failed to parse merged GPX: %v", err)
	}
	if parsed.Metadata.Name != "Prefixed" {
		t.Fatalf("unexpected merged name: %q", parsed.Metadata.Name)
	}
	if len(parsed.Tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(parsed.Tracks))
	}
}

func TestMergeGpx_Gpx10(t *testing.T) {
	g1 := `<?xml version="1.0"?>
<gpx version="1.0" creator="test" xmlns="http://www.topografix.com/GPX/1/0">
  <name>Tour 10</name>
  <time>2018-06-01T08:00:00Z</time>
  <trk><trkseg>
    <trkpt lat="48.0" lon="11.0"><ele>100</ele></trkpt>
  </trkseg></trk>
</gpx>`

	mergedBytes, err := mergeGpxBytes([]byte(g1))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed mergedGpx
	if err := xml.Unmarshal(mergedBytes, &parsed); err != nil {
		t.Fatalf("failed to parse merged GPX: %v", err)
	}
	if parsed.Metadata.Name != "Tour 10" {
		t.Fatalf("unexpected merged name: %q", parsed.Metadata.Name)
	}
	if len(parsed.Tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(parsed.Tracks))
	}
	want := time.Date(2018, 6, 1, 8, 0, 0, 0, time.UTC)
	got, err := time.Parse(time.RFC3339, parsed.Metadata.Time)
	if err != nil {
		t.Fatalf("invalid merged time: %v", err)
	}
	if !got.Equal(want) {
		t.Fatalf("expected time %v, got %v", want, got)
	}
}

func TestMergeGpx_reparse(t *testing.T) {
	g1 := `<gpx version="1.1" xmlns="http://www.topografix.com/GPX/1/1"><metadata><name>A</name></metadata><trk><trkseg>
<trkpt lat="52.0" lon="13.0"><ele>100</ele><time>2021-01-01T10:00:00Z</time></trkpt>
<trkpt lat="52.01" lon="13.01"><ele>110</ele><time>2021-01-01T10:01:00Z</time></trkpt>
</trkseg></trk></gpx>`
	g2 := `<gpx version="1.1" xmlns="http://www.topografix.com/GPX/1/1"><metadata><name>B</name></metadata><rte><rtept lat="53.0" lon="14.0"></rtept><rtept lat="53.01" lon="14.01"></rtept></rte></gpx>`

	merged, err := mergeGpxBytes([]byte(g1), []byte(g2))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("merged: %s", merged)

	res, err := Parse(string(merged))
	if err != nil {
		t.Fatalf("lean parser failed on merged GPX: %v", err)
	}
	if res.Name != "A, B" {
		t.Fatalf("name = %q", res.Name)
	}
	if len(res.Paths) != 2 || len(res.Points) != 0 {
		t.Fatalf("paths=%d points=%d", len(res.Paths), len(res.Points))
	}
	if len(res.Paths[0]) != 2 {
		t.Fatalf("path len = %d", len(res.Paths[0]))
	}
	if res.MovingData == nil || res.UphillDownhill == nil {
		t.Fatalf("missing stats")
	}
}
