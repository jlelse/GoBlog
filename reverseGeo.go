package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	geojson "github.com/paulmach/go.geojson"
	"github.com/thoas/go-funk"
)

func geoTitle(lat, lon float64, lang string) string {
	ba, err := photonReverse(lat, lon, lang)
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

func photonReverse(lat, lon float64, lang string) ([]byte, error) {
	cacheKey := fmt.Sprintf("photon-%v-%v-%v", lat, lon, lang)
	cache, _ := retrievePersistentCache(cacheKey)
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
	resp, err := appHttpClient.Do(req)
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
	ba, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	_ = cachePersistently(cacheKey, ba)
	return ba, nil
}
