package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_blogroll(t *testing.T) {

	fc := newFakeHttpClient()

	app := &goBlog{
		httpClient: fc.Client,
		cfg:        createDefaultTestConfig(t),
	}

	app.cfg.Cache.Enable = false
	app.cfg.DefaultBlog = "en"
	app.cfg.Blogs = map[string]*configBlog{
		"en": {
			Lang: "en",
			Blogroll: &configBlogroll{
				Enabled:    true,
				Path:       "/br",
				AuthHeader: "Authheader",
				AuthValue:  "Authtoken",
				Opml:       "https://example.com/opml",
				Categories: []string{"A", "B"},
			},
		},
	}

	_ = app.initConfig(false)

	fc.setFakeResponse(http.StatusOK, `
	<opml version="2.0">
		<head>
			<dateCreated>Tue, 30 Nov 2021 19:34:38 UTC</dateCreated>
		</head>
		<body>
		<outline text="B">
			<outline text="A text" xmlUrl="https://a.example.com/feed.xml" htmlUrl="https://a.example.com" title="A title"/>
			<outline text="B text" xmlUrl="https://b.example.com/feed.xml" htmlUrl="https://b.example.com" title="B title"/>
		</outline>
		<outline text="A">
			<outline text="C text" xmlUrl="https://c.example.com/feed.xml" htmlUrl="https://c.example.com" title="C title"/>
			<outline text="D text" xmlUrl="https://d.example.com/feed.xml" htmlUrl="https://d.example.com" title="D title"/>
		</outline>
		<outline text="C">
		</outline>
		</body>
	</opml>
	`)

	// Test getting the blogroll
	// Tests sorting and filtering

	outlines, err := app.getBlogrollOutlines("en")
	require.NoError(t, err)
	require.NotNil(t, outlines)

	if assert.Len(t, outlines, 2) {
		assert.Equal(t, "A", outlines[0].Text)
		assert.Equal(t, "B", outlines[1].Text)
		if assert.Len(t, outlines[0].Outlines, 2) {
			assert.Equal(t, "C text", outlines[0].Outlines[0].Text)
			assert.Equal(t, "C title", outlines[0].Outlines[0].Title)
		}
	}

	// Test getting the OPML

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/br.opml", nil)
	req = req.WithContext(context.WithValue(req.Context(), blogKey, "en"))

	app.serveBlogrollExport(rec, req)

	assert.Equal(t, 200, rec.Code)

}
