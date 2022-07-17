package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_geoTrack(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Blogs = map[string]*configBlog{
		"en": {
			Lang: "en",
		},
		"de": {
			Lang: "de",
		},
	}

	_ = app.initConfig(false)

	// First test (just with track)

	gpxBytes, _ := os.ReadFile("testdata/test.gpx")

	p := &post{
		Blog: "en",
		Parameters: map[string][]string{
			"gpx": {
				string(gpxBytes),
			},
		},
	}

	resEn, err := app.getTrack(p, true)
	require.NoError(t, err)

	assert.NotEmpty(t, resEn.Paths)
	assert.Empty(t, resEn.Points)
	assert.Equal(t, "2.70", resEn.Kilometers)
	assert.Equal(t, "0:42:53", resEn.Hours)

	p.Blog = "de"

	resDe, err := app.getTrack(p, true)
	require.NoError(t, err)

	assert.NotEmpty(t, resDe.Paths)
	assert.Empty(t, resDe.Points)
	assert.Equal(t, "2,70", resDe.Kilometers)
	assert.Equal(t, "0:42:53", resDe.Hours)

	// Second file (with track and waypoint)

	gpxBytes, _ = os.ReadFile("testdata/test2.gpx")

	p = &post{
		Blog: "en",
		Parameters: map[string][]string{
			"gpx": {
				string(gpxBytes),
			},
		},
	}

	resEn, err = app.getTrack(p, true)
	require.NoError(t, err)

	assert.NotEmpty(t, resEn.Paths)
	assert.NotEmpty(t, resEn.Points)
	assert.Equal(t, "0.08", resEn.Kilometers)
	assert.Equal(t, "0:01:29", resEn.Hours)

	// Test "privacy" feature to hide track

	p = &post{
		Blog: "en",
		Parameters: map[string][]string{
			"gpx":       {string(gpxBytes)},
			"showroute": {"false"},
		},
	}

	resEn, err = app.getTrack(p, p.showTrackRoute())
	require.NoError(t, err)

	assert.False(t, p.showTrackRoute())
	assert.Empty(t, resEn.Paths)
	assert.Empty(t, resEn.Points)
	assert.Equal(t, "0.08", resEn.Kilometers)
	assert.Equal(t, "0:01:29", resEn.Hours)

	// Third file (just with route)

	gpxBytes, _ = os.ReadFile("testdata/test3.gpx")

	p = &post{
		Blog: "en",
		Parameters: map[string][]string{
			"gpx": {
				string(gpxBytes),
			},
		},
	}

	resEn, err = app.getTrack(p, true)
	require.NoError(t, err)

	assert.NotEmpty(t, resEn.Paths)
	assert.Empty(t, resEn.Points)
	assert.Equal(t, "", resEn.Kilometers)
	assert.Equal(t, "", resEn.Hours)

}
