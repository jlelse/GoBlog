package main

import (
	"errors"
	"math"

	"go.goblog.app/app/pkgs/gpx"
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
	Paths      [][]gpx.Point
	Points     []gpx.Point
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
	parseResult, err := gpx.Parse(gpxString)
	if err != nil {
		// Failed to parse, but just log error
		a.error("failed to parse GPX", "err", err)
		return nil, nil
	}

	l, _ := language.Parse(a.getBlogFromPost(p).Lang)
	lp := message.NewPrinter(l)

	result = &trackResult{
		Name: parseResult.Name,
	}

	// Add Paths
	result.Paths = parseResult.Paths
	// Add Points
	result.Points = parseResult.Points
	// Calculate statistics
	if parseResult.MovingData != nil {
		result.Kilometers = lp.Sprintf("%.2f", (parseResult.MovingData.MovingDistance+parseResult.MovingData.StoppedDistance)/1000)
		result.Hours = lp.Sprintf(
			"%.0f:%02.0f:%02.0f",
			math.Floor(parseResult.MovingData.MovingTime/3600),               // Hours
			math.Floor(math.Mod(parseResult.MovingData.MovingTime, 3600)/60), // Minutes
			math.Floor(math.Mod(parseResult.MovingData.MovingTime, 60)),      // Seconds
		)
	}
	if parseResult.UphillDownhill != nil {
		result.Uphill = lp.Sprintf("%.0f", parseResult.UphillDownhill.Uphill)
		result.Downhill = lp.Sprintf("%.0f", parseResult.UphillDownhill.Downhill)
	}

	return result, nil
}
