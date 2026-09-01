// Package gpx provides a lean GPX parser and helpers. It extracts only the
// data needed by GoBlog from GPX files: track name, paths, waypoints and
// movement statistics. It also provides merging of multiple GPX files into
// one and Web Mercator projection helpers.
package gpx

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/HBTGmbH/gosaxml"
)

// Track is the parsed representation of a GPX document.
type Track struct {
	Name           string
	Paths          [][]Point
	Points         []Point
	MovingData     *MovingData
	UphillDownhill *UphillDownhill
}

// MovingData holds the movement statistics of a track.
type MovingData struct {
	MovingTime      float64
	StoppedTime     float64
	MovingDistance  float64
	StoppedDistance float64
	MaxSpeed        float64
}

// UphillDownhill holds the accumulated elevation gain and loss of a track.
type UphillDownhill struct {
	Uphill   float64
	Downhill float64
}

// Point is a coordinate pair: Lat, Lon.
type Point [2]float64

// Lat returns the latitude of the point.
func (p *Point) Lat() float64 {
	return p[0]
}

// Lon returns the longitude of the point.
func (p *Point) Lon() float64 {
	return p[1]
}

// Trunc truncates a coordinate to 5 decimal places.
func Trunc(num float64) float64 {
	return float64(int64(num*100000)) / 100000
}

var decoderPool = sync.Pool{
	New: func() any {
		return gosaxml.NewDecoder(nil)
	},
}

// Parse parses a GPX document into a Track. Malformed input is returned as
// an error; panics of the underlying XML decoder on malformed input are
// recovered and returned as errors as well.
func Parse(gpxString string) (track *Track, err error) {
	defer func() {
		if r := recover(); r != nil {
			track = nil
			err = fmt.Errorf("panic while parsing GPX: %v", r)
		}
	}()
	s, err := unescapeCDATA(gpxString)
	if err != nil {
		return nil, err
	}
	return parseGPX(s)
}

const (
	cdataOpen  = "<![CDATA["
	cdataClose = "]]>"
)

// cdataEscaper escapes the text content of CDATA sections so that it
// survives as plain character data.
var cdataEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

// unescapeCDATA replaces CDATA sections with their escaped text content,
// because the XML decoder does not support CDATA sections yet
// (https://github.com/HBTGmbH/gosaxml/pull/36). Returns the input unchanged
// when it contains no CDATA section.
func unescapeCDATA(s string) (string, error) {
	open := strings.Index(s, cdataOpen)
	if open < 0 {
		return s, nil
	}
	var sb strings.Builder
	sb.Grow(len(s))
	for {
		sb.WriteString(s[:open])
		rest := s[open+len(cdataOpen):]
		before, after, ok := strings.Cut(rest, cdataClose)
		if !ok {
			return "", errors.New("unterminated CDATA section")
		}
		sb.WriteString(cdataEscaper.Replace(before))
		s = after
		if open = strings.Index(s, cdataOpen); open < 0 {
			sb.WriteString(s)
			return sb.String(), nil
		}
	}
}
