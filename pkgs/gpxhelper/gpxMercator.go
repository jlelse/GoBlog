// Package gpxhelper provides GPX file processing utilities.
package gpxhelper

import "math"

const (
	// EARTH_RADIUS is the Earth's radius in meters.
	EARTH_RADIUS = 6378137.0 // Earth's radius in meters
	DEG_TO_RAD   = math.Pi / 180.0
	RAD_TO_DEG   = 180.0 / math.Pi
)

// WebMercatorX converts a longitude to a Web Mercator X coordinate.
func WebMercatorX(lon float64) float64 {
	return EARTH_RADIUS * lon * DEG_TO_RAD
}

// WebMercatorY converts a latitude to a Web Mercator Y coordinate.
func WebMercatorY(lat float64) float64 {
	sin := math.Sin(lat * DEG_TO_RAD)
	y := EARTH_RADIUS * math.Log((1+sin)/(1-sin)) / 2
	return y
}
