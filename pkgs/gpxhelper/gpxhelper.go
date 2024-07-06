package gpxhelper

import (
	"fmt"
	"strings"
	"time"

	"github.com/tkrajina/gpxgo/gpx"
)

func MergeGpx(gpxFiles ...[]byte) ([]byte, error) {
	if len(gpxFiles) == 0 {
		return nil, fmt.Errorf("no GPX files provided")
	}

	mergedGpx := gpx.GPX{}
	var names []string
	var earliestTime *time.Time

	for _, gpxFile := range gpxFiles {
		parsedGpx, err := gpx.ParseBytes(gpxFile)
		if err != nil {
			return nil, fmt.Errorf("error parsing GPX file: %w", err)
		}

		mergedGpx.Tracks = append(mergedGpx.Tracks, parsedGpx.Tracks...)
		mergedGpx.Waypoints = append(mergedGpx.Waypoints, parsedGpx.Waypoints...)
		mergedGpx.Routes = append(mergedGpx.Routes, parsedGpx.Routes...)

		if parsedGpx.Name != "" {
			names = append(names, parsedGpx.Name)
		}

		if parsedGpx.Time != nil {
			if earliestTime == nil || parsedGpx.Time.Before(*earliestTime) {
				earliestTime = parsedGpx.Time
			}
		}
	}

	mergedGpx.Name = strings.Join(names, ", ")
	if earliestTime != nil {
		mergedGpx.Time = earliestTime
	}

	return mergedGpx.ToXml(gpx.ToXmlParams{Version: "1.1", Indent: false})
}
