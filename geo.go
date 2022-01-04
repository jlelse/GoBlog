package main

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"strings"

	gogeouri "git.jlel.se/jlelse/go-geouri"
	"github.com/carlmjohnson/requests"
	geojson "github.com/paulmach/go.geojson"
	"github.com/thoas/go-funk"
)

func (a *goBlog) geoTitle(g *gogeouri.Geo, lang string) string {
	if name, ok := g.Parameters["name"]; ok && len(name) > 0 && name[0] != "" {
		return name[0]
	}
	ba, err := a.photonReverse(g.Latitude, g.Longitude, lang)
	if err != nil {
		return ""
	}
	fc, err := geojson.UnmarshalFeatureCollection(ba)
	if err != nil || len(fc.Features) < 1 {
		return ""
	}
	f := fc.Features[0]
	name := f.PropertyMustString("name", "")
	city := f.PropertyMustString("city", "")
	state := f.PropertyMustString("state", "")
	country := f.PropertyMustString("country", "")
	return strings.Join(funk.FilterString([]string{name, city, state, country}, func(s string) bool { return s != "" }), ", ")
}

func (a *goBlog) photonReverse(lat, lon float64, lang string) ([]byte, error) {
	cacheKey := fmt.Sprintf("photon-%v-%v-%v", lat, lon, lang)
	cache, _ := a.db.retrievePersistentCache(cacheKey)
	if cache != nil {
		return cache, nil
	}
	var buf bytes.Buffer
	rb := requests.URL("https://photon.komoot.io/reverse").Client(a.httpClient).UserAgent(appUserAgent).ToBytesBuffer(&buf)
	rb.Param("lat", fmt.Sprintf("%v", lat)).Param("lon", fmt.Sprintf("%v", lon))
	if lang == "de" || lang == "fr" || lang == "it" {
		rb.Param("lang", lang)
	} else {
		rb.Param("lang", "en")
	}
	if err := rb.Fetch(context.Background()); err != nil {
		return nil, err
	}
	_ = a.db.cachePersistently(cacheKey, buf.Bytes())
	return buf.Bytes(), nil
}

func geoOSMLink(g *gogeouri.Geo) string {
	return fmt.Sprintf("https://www.openstreetmap.org/?mlat=%v&mlon=%v", g.Latitude, g.Longitude)
}

//go:embed leaflet/*
var leafletFiles embed.FS
