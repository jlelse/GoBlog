package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	geojson "github.com/paulmach/go.geojson"
	"github.com/thoas/go-funk"
)

func geoTitle(lat, lon float64) string {
	ba, err := photonReverse(lat, lon)
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

func photonReverse(lat, lon float64) ([]byte, error) {
	cacheKey := fmt.Sprintf("photon-%v-%v", lat, lon)
	cache, _ := retrievePersistentCache(cacheKey)
	if cache != nil {
		return cache, nil
	}
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://photon.komoot.io/reverse?lat=%v&lon=%v", lat, lon), nil)
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
