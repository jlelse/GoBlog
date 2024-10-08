package main

import (
	"cmp"
	"net/http"

	"github.com/samber/lo"
)

const defaultGeoMapPath = "/map"

func (a *goBlog) blogsOnMap(blog string, bc *configBlog) []string {
	if bc.Map != nil && bc.Map.AllBlogs {
		return lo.Keys(a.cfg.Blogs)
	}
	return []string{blog}
}

func (a *goBlog) serveGeoMap(w http.ResponseWriter, r *http.Request) {
	blog, bc := a.getBlog(r)

	mapPath := bc.getRelativePath(cmp.Or(bc.Map.Path, defaultGeoMapPath))
	canonical := a.getFullAddress(mapPath)

	allPostsWithLocationRequestConfig := &postsRequestConfig{
		blogs:                 a.blogsOnMap(blog, bc),
		anyParams:             []string{a.cfg.Micropub.LocationParam, gpxParameter},
		fetchWithoutParams:    true,
		excludeParameter:      showRouteParam,
		excludeParameterValue: "false", // Don't show hidden route tracks
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
	blog, bc := a.getBlog(r)

	allPostsWithTracksRequestConfig := &postsRequestConfig{
		blogs:                 a.blogsOnMap(blog, bc),
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
		Paths  [][]trackPoint
		Points []trackPoint
		Post   string
	}

	var tracks []templateTrack
	for _, p := range allPostsWithTracks {
		if t, err := a.getTrack(p); err == nil && t != nil {
			tracks = append(tracks, templateTrack{
				Paths:  t.Paths,
				Points: t.Points,
				Post:   p.Path,
			})
		}
	}

	a.respondWithMinifiedJson(w, &tracks)
}

const geoMapLocationsSubpath = "/locations.json"

func (a *goBlog) serveGeoMapLocations(w http.ResponseWriter, r *http.Request) {
	blog, bc := a.getBlog(r)

	allPostsWithLocationRequestConfig := &postsRequestConfig{
		blogs:       a.blogsOnMap(blog, bc),
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
		Point trackPoint
		Post  string
	}

	trunc := func(num float64) float64 {
		return float64(int64(num*100000)) / 100000
	}

	var locations []templateLocation
	for _, p := range allPostsWithLocations {
		for _, g := range a.geoURIs(p) {
			locations = append(locations, templateLocation{
				Point: trackPoint{trunc(g.Latitude), trunc(g.Longitude)},
				Post:  p.Path,
			})
		}
	}

	a.respondWithMinifiedJson(w, &locations)
}
