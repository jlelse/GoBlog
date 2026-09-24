package main

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/carlmjohnson/requests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_llmsTxt(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}

	_ = app.initConfig(false)

	err := app.createPost(&post{
		Path:      "/testpost",
		Section:   "posts",
		Status:    "published",
		Published: "2020-10-15T10:00:00Z",
		Parameters: map[string][]string{
			"title": {"Test Post"},
			"tags":  {"Test"},
		},
		Content: "Test Content",
	})
	require.NoError(t, err)

	err = app.setBlogLongDescription("default", "My long blog description.")
	require.NoError(t, err)
	app.cfg.Blogs["default"].LongDescription = "My long blog description."
	// Section description with markdown formatting should be rendered
	section := app.cfg.Blogs["default"].Sections["posts"]
	section.Description = "Posts, **partially with markdown** *formatting*."

	app.cfg.Blogs["default"].Menus = map[string]*configMenu{
		"Main": {
			Items: []*configMenuItem{
				{Title: "About", Link: "/about"},
			},
		},
	}

	// Enable some features
	app.cfg.Blogs["default"].Photos = &configPhotos{Enabled: true}
	app.cfg.Blogs["default"].Search = &configSearch{Enabled: true}
	app.cfg.Blogs["default"].Blogroll = &configBlogroll{Enabled: true}
	app.cfg.Blogs["default"].OnThisDay = &configOnThisDay{Enabled: true}
	app.cfg.Blogs["default"].RandomPost = &configRandomPost{Enabled: true}

	// Add a second blog with a subpath, should be listed after the root blog
	app.cfg.Blogs["de"] = &configBlog{
		Path:        "/de",
		Lang:        "de",
		Title:       "Mein Blog",
		Description: "Willkommen.",
	}

	_ = app.initTemplateStrings()

	_ = app.initTemplateStrings()

	app.d = app.buildRouter()

	client := newHandlerClient(app.d)

	var resString string

	err = requests.
		URL("http://localhost:8080/llms.txt").
		CheckStatus(http.StatusOK).
		ToString(&resString).
		Client(client).Fetch(context.Background())
	require.NoError(t, err)

	assert.Contains(t, resString, "# My Blog")
	assert.Contains(t, resString, "> Language: en")
	assert.Contains(t, resString, "> Welcome to my blog.")
	assert.Contains(t, resString, "> My long blog description.")
	assert.Contains(t, resString, "- [My Blog](http://localhost:8080): Welcome to my blog.")
	assert.Contains(t, resString, "- [Posts](http://localhost:8080/posts): Posts, partially with markdown formatting.")
	assert.Contains(t, resString, "> Welcome to my blog.")
	assert.Contains(t, resString, "## Features")
	assert.Contains(t, resString, "- [Photos](http://localhost:8080/photos)")
	assert.Contains(t, resString, "- [Search](http://localhost:8080/search)")
	assert.Contains(t, resString, "- [Blogroll](http://localhost:8080/blogroll)")
	assert.Contains(t, resString, "- [Photos](http://localhost:8080/photos)")
	assert.Contains(t, resString, "- [On this day](http://localhost:8080/onthisday)")
	assert.Contains(t, resString, "- [Random post](http://localhost:8080/random)")
	assert.Contains(t, resString, "- [Tags](http://localhost:8080/tags)")
	assert.Contains(t, resString, "- [Test](http://localhost:8080/tags/test)")
	assert.Contains(t, resString, "## Main")
	assert.Contains(t, resString, "- [About](/about)")
	// The root blog should be featured first, the blog with a subpath second
	assert.Less(t, strings.Index(resString, "# My Blog"), strings.Index(resString, "# Mein Blog"))
	assert.Contains(t, resString, "> Sprache: de")
}
