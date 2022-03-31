package main

import (
	"encoding/json"
	"errors"
	"log"
	"math"

	"github.com/tkrajina/gpxgo/gpx"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

const gpxParameter = "gpx"

func (p *post) HasTrack() bool {
	return p.firstParameter(gpxParameter) != ""
}

type trackResult struct {
	HasPoints        bool
	Paths            [][]*trackPoint
	PathsJSON        string
	Points           []*trackPoint
	PointsJSON       string
	Kilometers       string
	Hours            string
	Name             string
	MapAttribution   string
	MinZoom, MaxZoom int
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
		log.Printf("failed to parse GPX: %v", err)
		return nil, nil
	}

	l, _ := language.Parse(a.cfg.Blogs[p.Blog].Lang)
	lp := message.NewPrinter(l)

	pathsJSON, err := json.Marshal(parseResult.paths)
	if err != nil {
		return nil, err
	}

	pointsJSON, err := json.Marshal(parseResult.points)
	if err != nil {
		return nil, err
	}

	result = &trackResult{
		HasPoints:      len(parseResult.paths) > 0 && len(parseResult.paths[0]) > 0,
		Paths:          parseResult.paths,
		PathsJSON:      string(pathsJSON),
		Points:         parseResult.points,
		PointsJSON:     string(pointsJSON),
		Name:           parseResult.gpxData.Name,
		MapAttribution: a.getMapAttribution(),
		MinZoom:        a.getMinZoom(),
		MaxZoom:        a.getMaxZoom(),
	}

	if parseResult.md != nil {
		result.Kilometers = lp.Sprintf("%.2f", parseResult.md.MovingDistance/1000)
		result.Hours = lp.Sprintf(
			"%.0f:%02.0f:%02.0f",
			math.Floor(parseResult.md.MovingTime/3600),               // Hours
			math.Floor(math.Mod(parseResult.md.MovingTime, 3600)/60), // Minutes
			math.Floor(math.Mod(parseResult.md.MovingTime, 60)),      // Seconds
		)
	}

	return result, nil
}

type trackPoint struct {
	Lat, Lon float64
}

type trackParseResult struct {
	paths   [][]*trackPoint
	points  []*trackPoint
	gpxData *gpx.GPX
	md      *gpx.MovingData
}

func trackParseGPX(gpxString string) (result *trackParseResult, err error) {
	result = &trackParseResult{}

	type trackPath struct {
		gpxMovingData *gpx.MovingData
		points        []*trackPoint
	}

	result.gpxData, err = gpx.ParseString(gpxString)
	if err != nil {
		return nil, err
	}

	paths := make([]*trackPath, 0)
	for _, track := range result.gpxData.Tracks {
		for _, segment := range track.Segments {
			md := segment.MovingData()
			path := &trackPath{
				gpxMovingData: &md,
			}
			for _, point := range segment.Points {
				path.points = append(path.points, &trackPoint{
					Lat: point.GetLatitude(), Lon: point.GetLongitude(),
				})
			}
			paths = append(paths, path)
		}
	}
	for _, route := range result.gpxData.Routes {
		path := &trackPath{}
		for _, point := range route.Points {
			path.points = append(path.points, &trackPoint{
				Lat: point.GetLatitude(), Lon: point.GetLongitude(),
			})
		}
		paths = append(paths, path)
	}
	result.paths = make([][]*trackPoint, len(paths))
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
	}

	result.points = []*trackPoint{}
	for _, point := range result.gpxData.Waypoints {
		result.points = append(result.points, &trackPoint{
			Lat: point.GetLatitude(), Lon: point.GetLongitude(),
		})
	}

	return result, nil
}
