package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"strings"

	gogeouri "git.jlel.se/jlelse/go-geouri"
	"github.com/carlmjohnson/requests"
	geojson "github.com/paulmach/go.geojson"
	"github.com/thoas/go-funk"
	"go.goblog.app/app/pkgs/bufferpool"
)

func (a *goBlog) geoTitle(g *gogeouri.Geo, lang string) string {
	if name, ok := g.Parameters["name"]; ok && len(name) > 0 && name[0] != "" {
		return name[0]
	}
	fc, err := a.photonReverse(g.Latitude, g.Longitude, lang)
	if err != nil || len(fc.Features) < 1 {
		return ""
	}
	f := fc.Features[0]
	return strings.Join(funk.FilterString([]string{
		f.PropertyMustString("name", ""), f.PropertyMustString("city", ""), f.PropertyMustString("state", ""), f.PropertyMustString("country", ""),
	}, func(s string) bool { return s != "" }), ", ")
}

func (a *goBlog) photonReverse(lat, lon float64, lang string) (*geojson.FeatureCollection, error) {
	// Only allow one concurrent request
	a.photonMutex.Lock()
	defer a.photonMutex.Unlock()
	// Create feature collection
	fc := geojson.NewFeatureCollection()
	// Check cache
	cacheKey := fmt.Sprintf("photon-%v-%v-%v", lat, lon, lang)
	if cache, _ := a.db.retrievePersistentCache(cacheKey); cache != nil {
		// Cache hit, unmarshal and return
		if err := json.Unmarshal(cache, fc); err != nil {
			return nil, err
		}
		return fc, nil
	}
	// No cache, fetch from Photon
	buf := bufferpool.Get()
	defer bufferpool.Put(buf)
	// Create request
	rb := requests.URL("https://photon.komoot.io/reverse").Client(a.httpClient).UserAgent(appUserAgent).ToBytesBuffer(buf)
	// Set parameters
	rb.Param("lat", fmt.Sprintf("%v", lat)).Param("lon", fmt.Sprintf("%v", lon))
	rb.Param("lang", funk.ShortIf(lang == "de" || lang == "fr" || lang == "it", lang, "en").(string)) // Photon only supports en, de, fr, it
	// Do request
	if err := rb.Fetch(context.Background()); err != nil {
		return nil, err
	}
	// Cache response
	_ = a.db.cachePersistently(cacheKey, buf.Bytes())
	// Unmarshal response
	if err := json.NewDecoder(buf).Decode(fc); err != nil {
		return nil, err
	}
	return fc, nil
}

func geoOSMLink(g *gogeouri.Geo) string {
	return fmt.Sprintf("https://www.openstreetmap.org/?mlat=%v&mlon=%v", g.Latitude, g.Longitude)
}

//go:embed leaflet/*
var leafletFiles embed.FS
