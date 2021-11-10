package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_renderTrip(t *testing.T) {
	app := &goBlog{
		cfg: &config{
			Db: &configDb{
				File: filepath.Join(t.TempDir(), "test.db"),
			},
			Server: &configServer{},
			Blogs: map[string]*configBlog{
				"en": {
					Lang: "en",
				},
				"de": {
					Lang: "de",
				},
			},
		},
	}

	_ = app.initDatabase(false)
	app.initComponents(false)

	gpxBytes, _ := os.ReadFile("testdata/test.gpx")

	post := &post{
		Blog: "en",
		Parameters: map[string][]string{
			"gpx": {
				string(gpxBytes),
			},
		},
	}

	resEn, err := app.renderTrip(post)
	require.NoError(t, err)

	assert.True(t, resEn.HasPoints)
	assert.Equal(t, "2.70", resEn.Kilometers)
	assert.Equal(t, "0:42:53", resEn.Hours)

	post.Blog = "de"

	resDe, err := app.renderTrip(post)
	require.NoError(t, err)

	assert.True(t, resDe.HasPoints)
	assert.Equal(t, "2,70", resDe.Kilometers)
	assert.Equal(t, "0:42:53", resDe.Hours)
}
