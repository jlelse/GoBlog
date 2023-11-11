package main

import (
	"net/http"
)

const defaultGeoMapPath = "/map"

func (a *goBlog) serveGeoMap(w http.ResponseWriter, r *http.Request) {
	blog, bc := a.getBlog(r)

	mapPath := bc.getRelativePath(defaultIfEmpty(bc.Map.Path, defaultGeoMapPath))
	canonical := a.getFullAddress(mapPath)

	allPostsWithLocationRequestConfig := &postsRequestConfig{
		blog:        blog,
		anyParams:   []string{a.cfg.Micropub.LocationParam, gpxParameter},
		fetchParams: []string{a.cfg.Micropub.LocationParam, gpxParameter},
	}
	allPostsWithLocationRequestConfig.status, allPostsWithLocationRequestConfig.visibility = a.getDefaultPostStates(r)

	allPostsWithLocation, err := a.db.countPosts(allPostsWithLocationRequestConfig)
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}

	if allPostsWithLocation == 0 {
		a.render(w, r, a.renderGeoMap, &renderData{
			Canonical: canonical,
			Data: &geoMapRenderData{
				noLocations: true,
			},
		})
		return
	}

	a.render(w, r, a.renderGeoMap, &renderData{
		Canonical: canonical,
		Data: &geoMapRenderData{
			locations:   "url:" + canonical + geoMapLocationsSubpath,
			tracks:      "url:" + canonical + geoMapTracksSubpath,
			attribution: a.getMapAttribution(),
			minZoom:     a.getMinZoom(),
			maxZoom:     a.getMaxZoom(),
		},
	})
}

const geoMapTracksSubpath = "/tracks.json"

func (a *goBlog) serveGeoMapTracks(w http.ResponseWriter, r *http.Request) {
	blog, _ := a.getBlog(r)

	allPostsWithTracksRequestConfig := &postsRequestConfig{
		blog:                  blog,
		anyParams:             []string{gpxParameter},
		fetchParams:           []string{gpxParameter},
		excludeParameter:      showRouteParam,
		excludeParameterValue: "false", // Don't show hidden route tracks
	}
	allPostsWithTracksRequestConfig.status, allPostsWithTracksRequestConfig.visibility = a.getDefaultPostStates(r)

	allPostsWithTracks, err := a.getPosts(allPostsWithTracksRequestConfig)
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}

	type templateTrack struct {
		Paths  [][]*trackPoint
		Points []*trackPoint
		Post   string
	}

	var tracks []*templateTrack
	for _, p := range allPostsWithTracks {
		if t, err := a.getTrack(p, true); err == nil && t != nil {
			tracks = append(tracks, &templateTrack{
				Paths:  t.Paths,
				Points: t.Points,
				Post:   p.Path,
			})
		}
	}

	a.respondWithMinifiedJson(w, tracks)
}

const geoMapLocationsSubpath = "/locations.json"

func (a *goBlog) serveGeoMapLocations(w http.ResponseWriter, r *http.Request) {
	blog, _ := a.getBlog(r)

	allPostsWithLocationRequestConfig := &postsRequestConfig{
		blog:        blog,
		anyParams:   []string{a.cfg.Micropub.LocationParam},
		fetchParams: []string{a.cfg.Micropub.LocationParam},
	}
	allPostsWithLocationRequestConfig.status, allPostsWithLocationRequestConfig.visibility = a.getDefaultPostStates(r)

	allPostsWithLocations, err := a.getPosts(allPostsWithLocationRequestConfig)
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}

	type templateLocation struct {
		Lat  float64
		Lon  float64
		Post string
	}

	var locations []*templateLocation
	for _, p := range allPostsWithLocations {
		for _, g := range a.geoURIs(p) {
			locations = append(locations, &templateLocation{
				Lat:  g.Latitude,
				Lon:  g.Longitude,
				Post: p.Path,
			})
		}
	}

	a.respondWithMinifiedJson(w, locations)
}
