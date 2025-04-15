package main

import (
	"errors"
	"math"

	"github.com/tkrajina/gpxgo/gpx"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

const gpxParameter = "gpx"
const showRouteParam = "showroute"

func (p *post) hasTrack() bool {
	return p.firstParameter(gpxParameter) != ""
}

func (p *post) showTrackRoute() bool {
	if param := p.firstParameter(showRouteParam); param == "false" {
		return false
	}
	return true
}

type trackResult struct {
	Paths      [][]trackPoint
	Points     []trackPoint
	Kilometers string
	Hours      string
	Uphill     string
	Downhill   string
	Name       string
}

func (t *trackResult) hasPath() bool {
	return t.Paths != nil
}

func (t *trackResult) hasMapFeatures() bool {
	return t.Points != nil || t.hasPath()
}

func (a *goBlog) getTrack(p *post) (result *trackResult, err error) {
	gpxString := p.firstParameter(gpxParameter)
	if gpxString == "" {
		return nil, errors.New("no gpx parameter in post")
	}

	// Parse GPX
	parseResult, err := trackParseGPX(gpxString)
	if err != nil {
		// Failed to parse, but just log error
		a.error("failed to parse GPX", "err", err)
		return nil, nil
	}

	l, _ := language.Parse(a.getBlogFromPost(p).Lang)
	lp := message.NewPrinter(l)

	result = &trackResult{
		Name: parseResult.gpxData.Name,
	}

	// Add Paths
	result.Paths = parseResult.paths
	// Add Points
	result.Points = parseResult.points
	// Calculate statistics
	if parseResult.md != nil {
		result.Kilometers = lp.Sprintf("%.2f", (parseResult.md.MovingDistance+parseResult.md.StoppedDistance)/1000)
		result.Hours = lp.Sprintf(
			"%.0f:%02.0f:%02.0f",
			math.Floor(parseResult.md.MovingTime/3600),               // Hours
			math.Floor(math.Mod(parseResult.md.MovingTime, 3600)/60), // Minutes
			math.Floor(math.Mod(parseResult.md.MovingTime, 60)),      // Seconds
		)
	}
	if parseResult.ud != nil {
		result.Uphill = lp.Sprintf("%.0f", parseResult.ud.Uphill)
		result.Downhill = lp.Sprintf("%.0f", parseResult.ud.Downhill)
	}

	return result, nil
}

type trackPoint [2]float64 // Lat, Lon

func (p *trackPoint) Lat() float64 {
	return p[0]
}

func (p *trackPoint) Lon() float64 {
	return p[1]
}

type trackParseResult struct {
	paths   [][]trackPoint
	points  []trackPoint
	gpxData *gpx.GPX
	md      *gpx.MovingData
	ud      *gpx.UphillDownhill
}

func trackParseGPX(gpxString string) (result *trackParseResult, err error) {
	trunc := func(num float64) float64 {
		return float64(int64(num*100000)) / 100000
	}

	result = &trackParseResult{}

	type trackPath struct {
		gpxMovingData     *gpx.MovingData
		gpxUphillDownhill *gpx.UphillDownhill
		points            []trackPoint
	}

	result.gpxData, err = gpx.ParseString(gpxString)
	if err != nil {
		return nil, err
	}

	paths := make([]trackPath, 0)
	for _, track := range result.gpxData.Tracks {
		for _, segment := range track.Segments {
			md := segment.MovingData()
			ud := segment.UphillDownhill()
			path := trackPath{
				gpxMovingData:     &md,
				gpxUphillDownhill: &ud,
			}
			for _, point := range segment.Points {
				path.points = append(path.points, trackPoint{
					trunc(point.Latitude), trunc(point.Longitude),
				})
			}
			paths = append(paths, path)
		}
	}
	for _, route := range result.gpxData.Routes {
		path := trackPath{}
		for _, point := range route.Points {
			path.points = append(path.points, trackPoint{
				trunc(point.Latitude), trunc(point.Longitude),
			})
		}
		paths = append(paths, path)
	}
	result.paths = make([][]trackPoint, len(paths))
	for i, path := range paths {
		// Add points
		result.paths[i] = path.points
		// Combine moving data
		if path.gpxMovingData != nil {
			if result.md == nil {
				result.md = &gpx.MovingData{}
			}
			result.md.MaxSpeed = math.Max(result.md.MaxSpeed, path.gpxMovingData.MaxSpeed)
			result.md.MovingDistance = result.md.MovingDistance + path.gpxMovingData.MovingDistance
			result.md.MovingTime = result.md.MovingTime + path.gpxMovingData.MovingTime
			result.md.StoppedDistance = result.md.StoppedDistance + path.gpxMovingData.StoppedDistance
			result.md.StoppedTime = result.md.StoppedTime + path.gpxMovingData.StoppedTime
		}
		// Combine uphill/downhill
		if path.gpxUphillDownhill != nil {
			if result.ud == nil {
				result.ud = &gpx.UphillDownhill{}
			}
			result.ud.Uphill = result.ud.Uphill + path.gpxUphillDownhill.Uphill
			result.ud.Downhill = result.ud.Downhill + path.gpxUphillDownhill.Downhill
		}
	}

	result.points = []trackPoint{}
	for _, point := range result.gpxData.Waypoints {
		result.points = append(result.points, trackPoint{
			trunc(point.Latitude), trunc(point.Longitude),
		})
	}

	return result, nil
}
