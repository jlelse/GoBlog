package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.goblog.app/app/pkgs/bufferpool"
)

var _ io.Writer = &htmlBuilder{}
var _ io.StringWriter = &htmlBuilder{}

func Test_renderPostTax(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}

	t.Cleanup(app.cleanup)

	_ = app.initConfig(false)
	app.initComponents(false)

	p := &post{
		Parameters: map[string][]string{
			"tags": {"Foo", "Bar"},
		},
	}

	buf := bufferpool.Get()
	defer bufferpool.Put(buf)

	hb := newHtmlBuilder(buf)

	app.renderPostTax(hb, p, app.cfg.Blogs["default"])

	_, err := goquery.NewDocumentFromReader(strings.NewReader(buf.String()))
	require.NoError(t, err)

	assert.Equal(t, "<p><strong>Tags</strong>: <a class=\"p-category\" rel=\"tag\" href=\"/tags/bar\">Bar</a>, <a class=\"p-category\" rel=\"tag\" href=\"/tags/foo\">Foo</a></p>", buf.String())
}

func Test_renderOldContentWarning(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}

	t.Cleanup(app.cleanup)

	_ = app.initConfig(false)
	app.initComponents(false)

	p := &post{
		Published: "2018-01-01",
	}

	buf := &bytes.Buffer{}
	hb := newHtmlBuilder(buf)

	app.renderOldContentWarning(hb, p, app.cfg.Blogs["default"])
	res := buf.String()

	_, err := goquery.NewDocumentFromReader(strings.NewReader(res))
	require.NoError(t, err)

	assert.Equal(t, "<strong class=\"p border-top border-bottom\">⚠️ This entry is already over one year old. It may no longer be up to date. Opinions may have changed.</strong>", res)
}

func Test_renderInteractions(t *testing.T) {
	var err error

	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Server.PublicAddress = "https://example.com"

	t.Cleanup(app.cleanup)

	_ = app.initConfig(false)
	app.initComponents(false)

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
	hb := newHtmlBuilder(buf)

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

func Test_renderAuthor(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.User.Picture = "https://example.com/picture.jpg"
	app.cfg.User.Name = "John Doe"

	t.Cleanup(app.cleanup)

	_ = app.initConfig(false)
	app.initComponents(false)

	buf := &bytes.Buffer{}
	hb := newHtmlBuilder(buf)

	app.renderAuthor(hb)
	res := buf.String()

	_, err := goquery.NewDocumentFromReader(strings.NewReader(res))
	require.NoError(t, err)

	assert.Equal(t, "<div class=\"p-author h-card hide\"><data class=\"u-photo\" value=\"https://example.com/picture.jpg\"></data><a class=\"p-name u-url\" rel=\"me\" href=\"/\">John Doe</a></div>", res)
}
