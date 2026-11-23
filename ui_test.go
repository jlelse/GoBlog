package main

import (
	"bytes"
	"mime/multipart"
	"net/http/httptest"
	"net/url"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.goblog.app/app/pkgs/htmlbuilder"
	"willnorris.com/go/microformats"
)

func TestIndexFeed(t *testing.T) {
	app := &goBlog{cfg: createDefaultTestConfig(t)}
	require.NoError(t, app.initConfig(false))
	require.NoError(t, app.initTemplateStrings())
	blog := app.cfg.Blogs["default"]
	blog.Title = "Field notes"
	blog.Taxonomies = []*configTaxonomy{{Name: "series"}, {Name: "tags"}}
	p := &post{Blog: "default", Path: "/walk", Content: "Along the canal.",
		Parameters: map[string][]string{"tags": {"Walking", "Autumn"}, "series": {"Notebooks"}}}
	base, err := url.Parse("https://example.org/")
	require.NoError(t, err)
	rr := httptest.NewRecorder()
	app.render(rr, httptest.NewRequest("GET", "/", nil), app.renderIndex, &renderData{
		Blog: blog, IsHome: true, Canonical: base.String(), Data: &indexRenderData{posts: []*post{p}},
	})
	for _, feed := range microformats.Parse(rr.Body, base).Items {
		if slices.Contains(feed.Type, "h-feed") {
			require.Equal(t, []any{"Field notes"}, feed.Properties["name"])
			require.Equal(t, []any{base.String()}, feed.Properties["url"])
			require.Len(t, feed.Children, 1)
			entry := feed.Children[0]
			require.Contains(t, entry.Type, "h-entry")
			require.Contains(t, entry.Properties["url"], "https://example.org/walk")
			require.Equal(t, []any{"Autumn", "Notebooks", "Walking"}, entry.Properties["category"])
			return
		}
	}
	t.Fatal("index does not contain an h-feed")
}

func Test_renderOldContentWarning(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}

	_ = app.initConfig(false)
	_ = app.initTemplateStrings()

	p := &post{
		Published: "2018-01-01",
	}

	buf := &bytes.Buffer{}
	hb := htmlbuilder.NewHTMLBuilder(buf)

	app.renderOldContentWarning(hb, p, app.cfg.Blogs["default"])
	res := buf.String()

	_, err := goquery.NewDocumentFromReader(strings.NewReader(res))
	require.NoError(t, err)

	assert.Equal(t, "<strong class=\"p border-top border-bottom\" id=\"oldcontentwarning\">⚠️ This entry is already over one year old. It may no longer be up to date. Opinions may have changed.</strong>", res)
}

func Test_renderInteractions(t *testing.T) {
	var err error

	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Server.PublicAddress = "https://example.com"

	_ = app.initConfig(false)
	_ = app.initTemplateStrings()

	app.d = app.buildRouter()

	err = app.createPost(&post{
		Path: "/testpost1",
	})
	require.NoError(t, err)

	err = app.createPost(&post{
		Path:    "/testpost2",
		Content: "[Test](/testpost1)",
		Parameters: map[string][]string{
			"title": {"Test-Title"},
		},
	})
	require.NoError(t, err)

	err = app.verifyMention(&mention{
		Source: "https://example.com/testpost2",
		Target: "https://example.com/testpost1",
	})
	require.NoError(t, err)
	err = app.db.approveWebmentionID(1)
	require.NoError(t, err)

	err = app.createPost(&post{
		Path:    "/testpost3",
		Content: "[Test](/testpost2)",
		Parameters: map[string][]string{
			"title": {"Test-Title"},
		},
	})
	require.NoError(t, err)

	err = app.verifyMention(&mention{
		Source: "https://example.com/testpost3",
		Target: "https://example.com/testpost2",
	})
	require.NoError(t, err)
	err = app.db.approveWebmentionID(2)
	require.NoError(t, err)

	buf := &bytes.Buffer{}
	hb := htmlbuilder.NewHTMLBuilder(buf)

	app.renderInteractions(hb, &renderData{
		Blog:      app.cfg.Blogs["default"],
		Canonical: "https://example.com/testpost1",
	})
	res := buf.Bytes()

	_, err = goquery.NewDocumentFromReader(bytes.NewReader(res))
	require.NoError(t, err)

	expected, err := os.ReadFile("testdata/interactionstest.html")
	require.NoError(t, err)

	assert.Equal(t, expected, res)
}

func Test_renderInteractionsCommentFormLoggedIn(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Server.PublicAddress = "https://example.com"
	app.cfg.User.Name = "John Doe"
	app.cfg.User.Link = "https://example.com/johndoe"

	_ = app.initConfig(false)
	_ = app.initTemplateStrings()
	app.cfg.Blogs["default"].Comments = &configComments{Enabled: true}

	render := func(loggedIn bool) string {
		req := httptest.NewRequest("GET", "/test", nil)
		setLoggedIn(req, loggedIn)
		rr := httptest.NewRecorder()
		app.render(rr, req, func(hb *htmlbuilder.HTMLBuilder, rd *renderData) {
			app.renderInteractions(hb, rd)
		}, &renderData{
			Blog:      app.cfg.Blogs["default"],
			Canonical: "https://example.com/test",
		})
		return rr.Body.String()
	}

	// Logged in: name and website prefilled with user link
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(render(true)))
	require.NoError(t, err)
	assert.Equal(t, "John Doe", doc.Find("form[action=\"/comment\"] input[name=\"name\"]").AttrOr("value", ""))
	assert.Equal(t, "https://example.com/johndoe", doc.Find("form[action=\"/comment\"] input[name=\"website\"]").AttrOr("value", ""))

	// Logged in without user link: website falls back to main URL
	app.cfg.User.Link = ""
	doc, err = goquery.NewDocumentFromReader(strings.NewReader(render(true)))
	require.NoError(t, err)
	assert.Equal(t, "https://example.com", doc.Find("form[action=\"/comment\"] input[name=\"website\"]").AttrOr("value", ""))

	// Not logged in: no prefill
	doc, err = goquery.NewDocumentFromReader(strings.NewReader(render(false)))
	require.NoError(t, err)
	_, nameValue := doc.Find("form[action=\"/comment\"] input[name=\"name\"]").Attr("value")
	_, websiteValue := doc.Find("form[action=\"/comment\"] input[name=\"website\"]").Attr("value")
	assert.False(t, nameValue)
	assert.False(t, websiteValue)
}

func Test_renderAuthorWithoutProfileImage(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.User.Name = "John Doe"

	_ = app.initConfig(false)

	buf := &bytes.Buffer{}
	hb := htmlbuilder.NewHTMLBuilder(buf)

	app.renderAuthor(hb)
	res := buf.String()

	_, err := goquery.NewDocumentFromReader(strings.NewReader(res))
	require.NoError(t, err)

	assert.Equal(t, "<div class=\"p-author h-card hide\"><a class=\"p-name u-url\" rel=\"me\" href=\"/\">John Doe</a></div>", res)
}

func Test_renderAuthorWithProfileImage(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.User.Name = "John Doe"

	_ = app.initConfig(false)

	// Update profile image
	requestBody := &bytes.Buffer{}
	writer := multipart.NewWriter(requestBody)
	fileWriter, err := writer.CreateFormFile("file", "newprofile.jpg")
	require.NoError(t, err)
	_, err = fileWriter.Write(defaultLogo)
	require.NoError(t, err)
	writer.Close()
	req := httptest.NewRequest("POST", "/update", requestBody)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()

	app.serveUpdateProfileImage(rr, req)

	require.Equal(t, 302, rr.Code)
	assert.Equal(t, []string{"/profile.jpg?v=e3da5a2d765ff693e7eb54cff717ae0f79ec79c06ed3e3adf6054a46c2824f32"}, rr.Header()["Location"])

	// Check rendering
	buf := &bytes.Buffer{}
	hb := htmlbuilder.NewHTMLBuilder(buf)

	app.renderAuthor(hb)
	res := buf.String()

	_, err = goquery.NewDocumentFromReader(strings.NewReader(res))
	require.NoError(t, err)

	assert.Equal(t, "<div class=\"p-author h-card hide\"><data class=\"u-photo\" value=\"http://localhost:8080/profile.jpg?v=e3da5a2d765ff693e7eb54cff717ae0f79ec79c06ed3e3adf6054a46c2824f32\"></data><a class=\"p-name u-url\" rel=\"me\" href=\"/\">John Doe</a></div>", res)
}

func Test_renderSettings(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}

	_ = app.initConfig(false)
	_ = app.initTemplateStrings()

	req := httptest.NewRequest("GET", "/settings", nil)
	rr := httptest.NewRecorder()

	app.render(rr, req, func(hb *htmlbuilder.HTMLBuilder, rd *renderData) {
		app.renderSettings(hb, rd)
	}, &renderData{
		Data: &settingsRenderData{
			blog:                  "default",
			hideOldContentWarning: true,
			hideShareButton:       false,
			hideSpeakButton:       false,
			hideTranslateButton:   true,
			addReplyTitle:         false,
			addReplyContext:       true,
			addLikeTitle:          false,
			addLikeContext:        true,
			userNick:              "testnick",
			userName:              "Test User",
			webmentionBlocklist: []*webmentionBlocklistEntry{
				{Host: "example.com", Incoming: true, Outgoing: false},
			},
		},
	})

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(rr.Body.String()))
	require.NoError(t, err)

	// Check that boolean settings are rendered
	assert.Equal(t, 1, doc.Find("form[action=\"/settings/oldcontentwarning\"]").Length())
	assert.Equal(t, 1, doc.Find("form[action=\"/settings/sharebutton\"]").Length())
	assert.Equal(t, 1, doc.Find("form[action=\"/settings/translatebutton\"]").Length())
	assert.Equal(t, 1, doc.Find("form[action=\"/settings/speakbutton\"]").Length())
	assert.Equal(t, 1, doc.Find("form[action=\"/settings/replytitle\"]").Length())
	assert.Equal(t, 1, doc.Find("form[action=\"/settings/replycontext\"]").Length())
	assert.Equal(t, 1, doc.Find("form[action=\"/settings/liketitle\"]").Length())
	assert.Equal(t, 1, doc.Find("form[action=\"/settings/likecontext\"]").Length())

	// Check that checked state is correct
	assert.Equal(t, 1, doc.Find("input[name=\"hideoldcontentwarning\"][checked]").Length())
	assert.Equal(t, 0, doc.Find("input[name=\"hidesharebutton\"][checked]").Length())
	assert.Equal(t, 1, doc.Find("input[name=\"hidetranslatebutton\"][checked]").Length())
	assert.Equal(t, 0, doc.Find("input[name=\"hidespeakbutton\"][checked]").Length())
	assert.Equal(t, 0, doc.Find("input[name=\"addreplytitle\"][checked]").Length())
	assert.Equal(t, 1, doc.Find("input[name=\"addreplycontext\"][checked]").Length())
	assert.Equal(t, 0, doc.Find("input[name=\"addliketitle\"][checked]").Length())
	assert.Equal(t, 1, doc.Find("input[name=\"addlikecontext\"][checked]").Length())

	// Webmention block list is rendered as collapsible details by default
	assert.Equal(t, 1, doc.Find("details.settings-webmention-blocklist").Length())
	assert.Equal(t, 0, doc.Find("details.settings-webmention-blocklist[open]").Length())
	assert.Equal(t, 1, doc.Find("details.settings-webmention-blocklist table.settings-table").Length())
}

func Test_renderGeoMapAndTrackMap(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	require.NoError(t, app.initConfig(false))
	require.NoError(t, app.initTemplateStrings())
	require.NoError(t, app.initTemplateAssets())
	app.cfg.Blogs["default"].Map = &configGeoMap{Enabled: true}

	t.Run("RasterMap", func(t *testing.T) {
		app.cfg.MapTiles = nil
		req := httptest.NewRequest("GET", "/map", nil)
		rr := httptest.NewRecorder()
		app.render(rr, req, app.renderGeoMap, &renderData{
			Blog: app.cfg.Blogs["default"],
			Data: &geoMapRenderData{
				locations: "url:/map/locations.json",
				tracks:    "url:/map/tracks.json",
			},
		})
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(rr.Body.String()))
		require.NoError(t, err)
		mapEl := doc.Find(".map-container #map")
		assert.Equal(t, 1, mapEl.Length())
		assert.Equal(t, "false", mapEl.AttrOr("data-vector", ""))
		assert.Equal(t, "url:/map/locations.json", mapEl.AttrOr("data-locations", ""))
		assert.Equal(t, "url:/map/tracks.json", mapEl.AttrOr("data-tracks", ""))
		assert.Equal(t, 0, doc.Find(".map-attribution").Length())
	})

	t.Run("VectorMapAndTrackMap", func(t *testing.T) {
		app.cfg.MapTiles = &configMapTiles{Vector: true, MinZoom: 2, MaxZoom: 18}
		req := httptest.NewRequest("GET", "/map", nil)
		rr := httptest.NewRecorder()
		app.render(rr, req, app.renderGeoMap, &renderData{
			Blog: app.cfg.Blogs["default"],
			Data: &geoMapRenderData{
				locations: "url:/map/locations.json",
				tracks:    "url:/map/tracks.json",
			},
		})
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(rr.Body.String()))
		require.NoError(t, err)
		mapEl := doc.Find(".map-container #map")
		assert.Equal(t, 1, mapEl.Length())
		assert.Equal(t, "true", mapEl.AttrOr("data-vector", ""))
		assert.Equal(t, "2", mapEl.AttrOr("data-minzoom", ""))
		assert.Equal(t, "18", mapEl.AttrOr("data-maxzoom", ""))
		assert.Contains(t, doc.Find(".map-attribution").Text(), "OpenFreeMap")

		buf := &bytes.Buffer{}
		hb := htmlbuilder.NewHTMLBuilder(buf)
		app.renderPostTrackMap(hb, &trackResult{})
		trackDoc, err := goquery.NewDocumentFromReader(strings.NewReader(buf.String()))
		require.NoError(t, err)
		trackMapEl := trackDoc.Find(".map-container #map")
		assert.Equal(t, 1, trackMapEl.Length())
		assert.Equal(t, "true", trackMapEl.AttrOr("data-vector", ""))
		assert.Contains(t, trackDoc.Find(".map-attribution").Text(), "OpenFreeMap")
	})
}
