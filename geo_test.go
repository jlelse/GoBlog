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

	// Test failed response
	fc.setFakeResponse(http.StatusNotFound, "")

	gt := app.geoTitle(gu, "de")

	assert.Equal(t, "52.5163, 13.3774", gt)

	// Test original Nominatim request
	fc.setFakeResponse(http.StatusOK, `{"type":"FeatureCollection","licence":"Data © OpenStreetMap contributors, ODbL 1.0. http://osm.org/copyright","features":[{"type":"Feature","properties":{"place_id":134066932,"osm_type":"relation","osm_id":1402158,"place_rank":21,"category":"boundary","type":"postal_code","importance":0.12008875486381318,"addresstype":"postcode","name":"10117","display_name":"10117, Mitte, Berlin, Deutschland","address":{"postcode":"10117","suburb":"Mitte","borough":"Mitte","city":"Berlin","ISO3166-2-lvl4":"DE-BE","country":"Deutschland","country_code":"de"}},"bbox":[13.3710363,52.5069995,13.4057347,52.5281501],"geometry":{"type": "Point","coordinates": [13.38776860055452, 52.5175184]}}]}`)

	gt = app.geoTitle(gu, "de")

	require.NotNil(t, fc.req)
	assert.Equal(t, "https://nominatim.openstreetmap.org/reverse?accept-language=de%2C+en&format=geojson&lat=52.51627&layer=address&lon=13.37737&zoom=14", fc.req.URL.String())

	assert.Equal(t, "Mitte, Berlin, Deutschland", gt)

	// Test cache
	fc.setFakeResponse(http.StatusOK, "")

	gt = app.geoTitle(gu, "de")

	assert.Nil(t, fc.req)

	assert.Equal(t, "Mitte, Berlin, Deutschland", gt)

}
