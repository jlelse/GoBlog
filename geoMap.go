package main

import (
	"encoding/json"
	"net/http"
)

const defaultGeoMapPath = "/map"

func (a *goBlog) serveGeoMap(w http.ResponseWriter, r *http.Request) {
	blog := r.Context().Value(blogKey).(string)
	bc := a.cfg.Blogs[blog]

	allPostsWithLocation, err := a.getPosts(&postsRequestConfig{
		blog:               blog,
		status:             statusPublished,
		parameters:         []string{a.cfg.Micropub.LocationParam, gpxParameter},
		withOnlyParameters: []string{a.cfg.Micropub.LocationParam, gpxParameter},
	})
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}

	if len(allPostsWithLocation) == 0 {
		a.render(w, r, templateGeoMap, &renderData{
			BlogString: blog,
			Data: map[string]interface{}{
				"nolocations": true,
			},
		})
		return
	}

	type templateLocation struct {
		Lat  float64
		Lon  float64
		Post string
	}

	type templateTrack struct {
		Paths  [][]*trackPoint
		Points []*trackPoint
		Post   string
	}

	var locations []*templateLocation
	var tracks []*templateTrack
	for _, p := range allPostsWithLocation {
		if g := a.geoURI(p); g != nil {
			locations = append(locations, &templateLocation{
				Lat:  g.Latitude,
				Lon:  g.Longitude,
				Post: p.Path,
			})
		}
		if t, err := a.getTrack(p); err == nil && t != nil {
			tracks = append(tracks, &templateTrack{
				Paths:  t.Paths,
				Points: t.Points,
				Post:   p.Path,
			})
		}
	}

	locationsJson := ""
	if len(locations) > 0 {
		locationsJsonBytes, err := json.Marshal(locations)
		if err != nil {
			a.serveError(w, r, err.Error(), http.StatusInternalServerError)
			return
		}
		locationsJson = string(locationsJsonBytes)
	}

	tracksJson := ""
	if len(tracks) > 0 {
		tracksJsonBytes, err := json.Marshal(tracks)
		if err != nil {
			a.serveError(w, r, err.Error(), http.StatusInternalServerError)
			return
		}
		tracksJson = string(tracksJsonBytes)
	}

	mapPath := bc.getRelativePath(defaultIfEmpty(bc.Map.Path, defaultGeoMapPath))
	a.render(w, r, templateGeoMap, &renderData{
		BlogString: blog,
		Canonical:  a.getFullAddress(mapPath),
		Data: map[string]interface{}{
			"locations": locationsJson,
			"tracks":    tracksJson,
		},
	})
}
