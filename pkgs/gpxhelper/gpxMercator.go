// Package gpxhelper provides GPX file processing utilities.
package gpxhelper

import "math"

const (
	// EarthRadius is the Earth's radius in meters.
	EarthRadius = 6378137.0
	// DegToRad converts degrees to radians.
	DegToRad = math.Pi / 180.0
	// RadToDeg converts radians to degrees.
	RadToDeg = 180.0 / math.Pi
)

// WebMercatorX converts a longitude to a Web Mercator X coordinate.
func WebMercatorX(lon float64) float64 {
	return EarthRadius * lon * DegToRad
}

// WebMercatorY converts a latitude to a Web Mercator Y coordinate.
func WebMercatorY(lat float64) float64 {
	sin := math.Sin(lat * DegToRad)
	y := EarthRadius * math.Log((1+sin)/(1-sin)) / 2
	return y
}
