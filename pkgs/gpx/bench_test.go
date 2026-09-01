package gpx

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
)

var benchGPX = sync.OnceValue(func() string {
	content, err := os.ReadFile("../../testdata/test.gpx")
	if err != nil {
		return ""
	}
	return string(content)
})

var benchGPXLarge = sync.OnceValue(func() string {
	return makeLargeGPX(200000)
})

// makeLargeGPX generates a synthetic GPX with the given number of track points.
func makeLargeGPX(points int) string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0"?><gpx version="1.1" xmlns="http://www.topografix.com/GPX/1/1"><metadata><name>Large</name></metadata><trk><trkseg>`)
	for i := range points {
		fmt.Fprintf(&sb, `<trkpt lat="%.6f" lon="%.6f"><ele>%.2f</ele><time>2021-01-01T10:00:%02dZ</time></trkpt>`, 52.0+float64(i)*0.0001, 13.0+float64(i)*0.0001, 100.0+float64(i%100), i%60)
	}
	sb.WriteString(`</trkseg></trk></gpx>`)
	return sb.String()
}

// BenchmarkParse measures the lean parser on a regular track.
func BenchmarkParse(b *testing.B) {
	s := benchGPX()
	if s == "" {
		b.Skip("testdata/test.gpx not found")
	}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Parse(s); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkParse_large measures the parser on a large track.
func BenchmarkParse_large(b *testing.B) {
	s := benchGPXLarge()
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Parse(s); err != nil {
			b.Fatal(err)
		}
	}
}
