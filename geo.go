package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	gogeouri "git.jlel.se/jlelse/go-geouri"
	"github.com/carlmjohnson/requests"
	geojson "github.com/paulmach/go.geojson"
	"go.goblog.app/app/pkgs/bufferpool"
)

func (a *goBlog) geoTitle(g *gogeouri.Geo, lang string) string {
	if name, ok := g.Parameters["name"]; ok && len(name) > 0 && name[0] != "" {
		return name[0]
	}
	fallbackTitle := fmt.Sprintf("%.4f, %.4f", g.Latitude, g.Longitude)
	fc, err := a.nominatimReverse(g.Latitude, g.Longitude, lang)
	if err != nil || len(fc.Features) == 0 {
		return fallbackTitle
	}
	feature := fc.Features[0]
	address, ok := feature.Properties["address"].(map[string]any)
	if !ok {
		return fallbackTitle
	}
	getFirstNonEmpty := func(keys ...string) string {
		for _, key := range keys {
			if value, ok := address[key].(string); ok && value != "" {
				return value
			}
		}
		return ""
	}
	titleParts := []string{
		getFirstNonEmpty("suburb", "neighborhood", "hamlet"),
		getFirstNonEmpty("village", "town", "city"),
		getFirstNonEmpty("state", "county"),
		getFirstNonEmpty("country"),
	}
	var nonEmptyParts []string
	for _, part := range titleParts {
		if part != "" {
			nonEmptyParts = append(nonEmptyParts, part)
		}
	}
	if title := strings.Join(nonEmptyParts, ", "); title != "" {
		return title
	}
	return fallbackTitle
}

func (a *goBlog) nominatimReverse(lat, lon float64, lang string) (*geojson.FeatureCollection, error) {
	key := fmt.Sprintf("nominatim-%v-%v-%v", lat, lon, lang)
	fc, err, _ := a.nominatimGroup.Do(key, func() (*geojson.FeatureCollection, error) {
		// Check cache
		var cached []byte
		if cache, _ := a.db.retrievePersistentCache(key); cache != nil {
			cached = cache
		} else {
			// No cache, fetch from Nominatim
			buf := bufferpool.Get()
			defer bufferpool.Put(buf)
			// Create request
			rb := requests.URL("https://nominatim.openstreetmap.org/reverse").Client(a.httpClient).ToBytesBuffer(buf)
			// Set parameters
			rb.Param("format", "geojson")
			rb.Param("lat", fmt.Sprintf("%v", lat))
			rb.Param("lon", fmt.Sprintf("%v", lon))
			rb.Param("accept-language", lang+", en")
			rb.Param("zoom", "14")
			rb.Param("layer", "address")
			// Do request
			ctx, cancelFunc := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancelFunc()
			if err := rb.Fetch(ctx); err != nil {
				return nil, err
			}
			cached = buf.Bytes()
			// Cache response
			_ = a.db.cachePersistently(key, cached)
		}
		// Unmarshal response
		fc := geojson.NewFeatureCollection()
		if err := json.Unmarshal(cached, fc); err != nil {
			return nil, err
		}
		return fc, nil
	})
	return fc, err
}

func geoOSMLink(g *gogeouri.Geo) string {
	return fmt.Sprintf("https://www.openstreetmap.org/?mlat=%v&mlon=%v", g.Latitude, g.Longitude)
}

//go:embed leaflet/*
var leafletFiles embed.FS
