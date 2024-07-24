package gpxhelper

import "math"

const (
	EARTH_RADIUS = 6378137.0 // Earth's radius in meters
	DEG_TO_RAD   = math.Pi / 180.0
	RAD_TO_DEG   = 180.0 / math.Pi
)

func WebMercatorX(lon float64) float64 {
	return EARTH_RADIUS * lon * DEG_TO_RAD
}

func WebMercatorY(lat float64) float64 {
	sin := math.Sin(lat * DEG_TO_RAD)
	y := EARTH_RADIUS * math.Log((1+sin)/(1-sin)) / 2
	return y
}
