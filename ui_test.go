package main

import (
	"bytes"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.goblog.app/app/pkgs/bufferpool"
	"go.goblog.app/app/pkgs/htmlbuilder"
)

func Test_renderPostTax(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}

	_ = app.initConfig(false)

	p := &post{
		Parameters: map[string][]string{
			"tags": {"Foo", "Bar"},
		},
	}

	buf := bufferpool.Get()
	defer bufferpool.Put(buf)

	hb := htmlbuilder.NewHtmlBuilder(buf)

	app.renderPostTax(hb, p, app.cfg.Blogs["default"])

	_, err := goquery.NewDocumentFromReader(strings.NewReader(buf.String()))
	require.NoError(t, err)

	assert.Equal(t, "<p><strong>Tags</strong>: <a class=\"p-category\" rel=\"tag\" href=\"/tags/bar\">Bar</a>, <a class=\"p-category\" rel=\"tag\" href=\"/tags/foo\">Foo</a></p>", buf.String())
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
	hb := htmlbuilder.NewHtmlBuilder(buf)

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
	err = app.db.approveWebmentionId(1)
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
	err = app.db.approveWebmentionId(2)
	require.NoError(t, err)

	buf := &bytes.Buffer{}
	hb := htmlbuilder.NewHtmlBuilder(buf)

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

func Test_renderAuthorWithoutProfileImage(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.User.Name = "John Doe"

	_ = app.initConfig(false)

	buf := &bytes.Buffer{}
	hb := htmlbuilder.NewHtmlBuilder(buf)

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
	assert.Equal(t, []string{"/profile.jpg?q=100&v=e3da5a2d765ff693e7eb54cff717ae0f79ec79c06ed3e3adf6054a46c2824f32"}, rr.Header()["Location"])

	// Check rendering
	buf := &bytes.Buffer{}
	hb := htmlbuilder.NewHtmlBuilder(buf)

	app.renderAuthor(hb)
	res := buf.String()

	_, err = goquery.NewDocumentFromReader(strings.NewReader(res))
	require.NoError(t, err)

	assert.Equal(t, "<div class=\"p-author h-card hide\"><data class=\"u-photo\" value=\"http://localhost:8080/profile.jpg?v=e3da5a2d765ff693e7eb54cff717ae0f79ec79c06ed3e3adf6054a46c2824f32\"></data><a class=\"p-name u-url\" rel=\"me\" href=\"/\">John Doe</a></div>", res)
}
