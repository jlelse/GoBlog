package main

import (
	"encoding/json"
	"io"
	"net/http"

	"go.goblog.app/app/pkgs/bufferpool"
	"go.goblog.app/app/pkgs/contenttype"
)

const defaultGeoMapPath = "/map"

func (a *goBlog) serveGeoMap(w http.ResponseWriter, r *http.Request) {
	blog, bc := a.getBlog(r)

	mapPath := bc.getRelativePath(defaultIfEmpty(bc.Map.Path, defaultGeoMapPath))
	canonical := a.getFullAddress(mapPath)

	allPostsWithLocation, err := a.db.countPosts(&postsRequestConfig{
		blog:               blog,
		statusse:           a.getDefaultPostStatusse(r),
		parameters:         []string{a.cfg.Micropub.LocationParam, gpxParameter},
		withOnlyParameters: []string{a.cfg.Micropub.LocationParam, gpxParameter},
	})
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

	allPostsWithTracks, err := a.getPosts(&postsRequestConfig{
		blog:                  blog,
		statusse:              a.getDefaultPostStatusse(r),
		parameters:            []string{gpxParameter},
		withOnlyParameters:    []string{gpxParameter},
		excludeParameter:      showRouteParam,
		excludeParameterValue: "false", // Don't show hidden route tracks
	})
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

	buf := bufferpool.Get()
	defer bufferpool.Put(buf)
	err = json.NewEncoder(buf).Encode(tracks)
	if err != nil {
		a.serveError(w, r, "", http.StatusInternalServerError)
		return
	}
	w.Header().Set(contentType, contenttype.JSONUTF8)
	_, _ = io.Copy(w, buf)
}

const geoMapLocationsSubpath = "/locations.json"

func (a *goBlog) serveGeoMapLocations(w http.ResponseWriter, r *http.Request) {
	blog, _ := a.getBlog(r)

	allPostsWithLocations, err := a.getPosts(&postsRequestConfig{
		blog:               blog,
		statusse:           a.getDefaultPostStatusse(r),
		parameters:         []string{a.cfg.Micropub.LocationParam},
		withOnlyParameters: []string{a.cfg.Micropub.LocationParam},
	})
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

	buf := bufferpool.Get()
	defer bufferpool.Put(buf)
	err = json.NewEncoder(buf).Encode(locations)
	if err != nil {
		a.serveError(w, r, "", http.StatusInternalServerError)
		return
	}
	w.Header().Set(contentType, contenttype.JSONUTF8)
	_, _ = io.Copy(w, buf)
}
