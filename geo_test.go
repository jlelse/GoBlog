package main

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_geo(t *testing.T) {

	fc := newFakeHttpClient()

	app := &goBlog{
		httpClient: fc.Client,
		cfg:        createDefaultTestConfig(t),
	}

	_ = app.initConfig(false)

	p := &post{
		Blog: "en",
		Parameters: map[string][]string{
			"location": {"geo:52.51627,13.37737"},
		},
	}

	gus := app.geoURIs(p)
	require.NotEmpty(t, gus)
	gu := gus[0]
	assert.Equal(t, 52.51627, gu.Latitude)
	assert.Equal(t, 13.37737, gu.Longitude)

	osmLink := geoOSMLink(gu)
	assert.Equal(t, "https://www.openstreetmap.org/?mlat=52.51627&mlon=13.37737", osmLink)

	// Test original Photon request

	fc.setFakeResponse(http.StatusOK, `{"features":[{"geometry":{"coordinates":[13.3774202,52.5162623],"type":"Point"},"type":"Feature","properties":{"osm_id":38345682,"osm_type":"W","extent":[13.3772052,52.5162623,13.3774202,52.5162476],"country":"Deutschland","osm_key":"highway","city":"Berlin","countrycode":"DE","district":"Mitte","osm_value":"service","postcode":"10117","name":"Platz des 18. März","type":"street"}}],"type":"FeatureCollection"}`)

	gt := app.geoTitle(gu, "de")

	require.NotNil(t, fc.req)
	assert.Equal(t, "https://photon.komoot.io/reverse?lang=de&lat=52.51627&lon=13.37737", fc.req.URL.String())

	assert.Equal(t, "Platz des 18. März, Berlin, Deutschland", gt)

	// Test cache

	fc.setFakeResponse(http.StatusOK, "")

	gt = app.geoTitle(gu, "de")

	assert.Nil(t, fc.req)

	assert.Equal(t, "Platz des 18. März, Berlin, Deutschland", gt)

}
