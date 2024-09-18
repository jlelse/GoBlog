package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	gogeouri "git.jlel.se/jlelse/go-geouri"
	"github.com/carlmjohnson/requests"
	geojson "github.com/paulmach/go.geojson"
	"github.com/samber/lo"
	"go.goblog.app/app/pkgs/bufferpool"
)

func (a *goBlog) geoTitle(g *gogeouri.Geo, lang string) string {
	if name, ok := g.Parameters["name"]; ok && len(name) > 0 && name[0] != "" {
		return name[0]
	}
	fc, err := a.photonReverse(g.Latitude, g.Longitude, lang)
	if err != nil || len(fc.Features) < 1 {
		return fmt.Sprintf("%.4f, %.4f", g.Latitude, g.Longitude)
	}
	f := fc.Features[0]
	return strings.Join(lo.Filter([]string{
		f.PropertyMustString("city", ""), f.PropertyMustString("state", ""), f.PropertyMustString("country", ""),
	}, func(s string, _ int) bool { return s != "" }), ", ")
}

func (a *goBlog) photonReverse(lat, lon float64, lang string) (*geojson.FeatureCollection, error) {
	key := fmt.Sprintf("photon-%v-%v-%v", lat, lon, lang)
	fc, err, _ := a.photonGroup.Do(key, func() (*geojson.FeatureCollection, error) {
		// Create feature collection
		fc := geojson.NewFeatureCollection()
		// Check cache
		if cache, _ := a.db.retrievePersistentCache(key); cache != nil {
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
		rb := requests.URL("https://photon.komoot.io/reverse").Client(a.httpClient).ToBytesBuffer(buf)
		// Set parameters
		rb.Param("lat", fmt.Sprintf("%v", lat)).Param("lon", fmt.Sprintf("%v", lon))
		rb.Param("lang", lo.If(slices.Contains([]string{"de", "fr", "it"}, lang), lang).Else("en"))
		// Do request
		ctx, cancelFunc := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancelFunc()
		if err := rb.Fetch(ctx); err != nil {
			return nil, err
		}
		// Unmarshal response
		if err := json.Unmarshal(buf.Bytes(), fc); err != nil {
			return nil, err
		}
		// Cache response
		_ = a.db.cachePersistently(key, buf.Bytes())
		return fc, nil
	})
	return fc, err
}

func geoOSMLink(g *gogeouri.Geo) string {
	return fmt.Sprintf("https://www.openstreetmap.org/?mlat=%v&mlon=%v", g.Latitude, g.Longitude)
}

//go:embed leaflet/*
var leafletFiles embed.FS
