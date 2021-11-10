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

func (p *post) HasTrip() bool {
	return p.firstParameter("gpx") != ""
}

type tripRenderResult struct {
	HasPoints  bool
	Paths      [][]*tripPoint
	PathsJSON  string
	Kilometers string
	Hours      string
	Name       string
}

func (a *goBlog) renderTrip(p *post) (result *tripRenderResult, err error) {
	gpxString := p.firstParameter("gpx")
	if gpxString == "" {
		return nil, errors.New("no gpx parameter in post")
	}

	// Parse GPX
	parseResult, err := tripParseGPX(gpxString)
	if err != nil {
		return nil, err
	}

	l, _ := language.Parse(a.cfg.Blogs[p.Blog].Lang)
	lp := message.NewPrinter(l)

	pathsJSON, err := json.Marshal(parseResult.paths)
	if err != nil {
		return nil, err
	}

	result = &tripRenderResult{
		HasPoints:  len(parseResult.paths) > 0 && len(parseResult.paths[0]) > 0,
		Paths:      parseResult.paths,
		PathsJSON:  string(pathsJSON),
		Name:       parseResult.gpxData.Name,
		Kilometers: lp.Sprintf("%.2f", parseResult.md.MovingDistance/1000),
		Hours: lp.Sprintf(
			"%.0f:%2.0f:%2.0f",
			math.Floor(parseResult.md.MovingTime/3600),               // Hours
			math.Floor(math.Mod(parseResult.md.MovingTime, 3600)/60), // Minutes
			math.Floor(math.Mod(parseResult.md.MovingTime, 60)),      // Seconds
		),
	}

	log.Println(string(pathsJSON))

	return result, nil
}

type tripPoint struct {
	Lat, Lon float64
}

type tripParseResult struct {
	paths   [][]*tripPoint
	gpxData *gpx.GPX
	md      *gpx.MovingData
}

func tripParseGPX(gpxString string) (result *tripParseResult, err error) {
	result = &tripParseResult{}

	type tripPath struct {
		gpxMovingData *gpx.MovingData
		points        []*tripPoint
	}

	var paths []*tripPath

	result.gpxData, err = gpx.ParseString(gpxString)
	if err != nil {
		return nil, err
	}
	for _, track := range result.gpxData.Tracks {
		for _, segment := range track.Segments {
			md := segment.MovingData()
			path := &tripPath{
				gpxMovingData: &md,
			}
			for _, point := range segment.Points {
				path.points = append(path.points, &tripPoint{
					Lat: point.GetLatitude(), Lon: point.GetLongitude(),
				})
			}
			paths = append(paths, path)
		}
	}

	result.md = &gpx.MovingData{}
	for _, path := range paths {
		// Combine moving data
		result.md.MaxSpeed = math.Max(result.md.MaxSpeed, path.gpxMovingData.MaxSpeed)
		result.md.MovingDistance = result.md.MovingDistance + path.gpxMovingData.MovingDistance
		result.md.MovingTime = result.md.MovingTime + path.gpxMovingData.MovingTime
		result.md.StoppedDistance = result.md.StoppedDistance + path.gpxMovingData.StoppedDistance
		result.md.StoppedTime = result.md.StoppedTime + path.gpxMovingData.StoppedTime
		// Add points
		result.paths = append(result.paths, path.points)
	}

	return result, nil
}
