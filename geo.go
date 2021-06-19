package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	gogeouri "git.jlel.se/jlelse/go-geouri"
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
	uv := url.Values{}
	uv.Set("lat", fmt.Sprintf("%v", lat))
	uv.Set("lon", fmt.Sprintf("%v", lon))
	if lang == "de" || lang == "fr" || lang == "it" {
		uv.Set("lang", lang)
	} else {
		uv.Set("lang", "en")
	}
	req, err := http.NewRequest(http.MethodGet, "https://photon.komoot.io/reverse?"+uv.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set(userAgent, appUserAgent)
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("response status code: %v", resp.StatusCode)
	}
	ba, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	_ = a.db.cachePersistently(cacheKey, ba)
	return ba, nil
}

func geoOSMLink(g *gogeouri.Geo) string {
	return fmt.Sprintf("https://www.openstreetmap.org/?mlat=%v&mlon=%v", g.Latitude, g.Longitude)
}
