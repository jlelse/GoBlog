package main

import (
	"html/template"
	"os"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_renderPostTax(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	_ = app.initConfig()
	_ = app.initDatabase(false)
	app.initComponents(false)

	p := &post{
		Parameters: map[string][]string{
			"tags": {"Foo", "Bar"},
		},
	}

	res := app.renderPostTax(p, app.cfg.Blogs["default"])
	_, err := goquery.NewDocumentFromReader(strings.NewReader(string(res)))
	require.NoError(t, err)

	assert.Equal(t, template.HTML("<p><strong>Tags</strong>: <a class=\"p-category\" rel=\"tag\" href=\"/tags/bar\">Bar</a>, <a class=\"p-category\" rel=\"tag\" href=\"/tags/foo\">Foo</a></p>"), res)
}

func Test_renderOldContentWarning(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	_ = app.initConfig()
	_ = app.initDatabase(false)
	app.initComponents(false)

	p := &post{
		Published: "2018-01-01",
	}

	res := app.renderOldContentWarning(p, app.cfg.Blogs["default"])
	_, err := goquery.NewDocumentFromReader(strings.NewReader(string(res)))
	require.NoError(t, err)

	assert.Equal(t, template.HTML("<strong class=\"p border-top border-bottom\">⚠️ This entry is already over one year old. It may no longer be up to date. Opinions may have changed.</strong>"), res)
}

func Test_renderInteractions(t *testing.T) {
	var err error

	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Server.PublicAddress = "https://example.com"
	_ = app.initConfig()
	_ = app.initDatabase(false)
	app.initComponents(false)
	app.d, err = app.buildRouter()
	require.NoError(t, err)

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

	res := app.renderInteractions(app.cfg.Blogs["default"], "https://example.com/testpost1")
	_, err = goquery.NewDocumentFromReader(strings.NewReader(string(res)))
	require.NoError(t, err)

	expected, err := os.ReadFile("testdata/interactionstest.html")
	require.NoError(t, err)

	assert.Equal(t, template.HTML(expected), res)
}

func Test_renderAuthor(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.User.Picture = "https://example.com/picture.jpg"
	app.cfg.User.Name = "John Doe"
	_ = app.initConfig()
	_ = app.initDatabase(false)
	app.initComponents(false)

	res := app.renderAuthor()
	_, err := goquery.NewDocumentFromReader(strings.NewReader(string(res)))
	require.NoError(t, err)

	assert.Equal(t, template.HTML("<div class=\"p-author h-card hide\"><data class=\"u-photo\" value=\"https://example.com/picture.jpg\"></data><a class=\"p-name u-url\" rel=\"me\" href=\"/\">John Doe</a></div>"), res)
}

func Test_renderTorNotice(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	_ = app.initConfig()
	_ = app.initDatabase(false)
	app.initComponents(false)

	app.cfg.Server.Tor = true

	res := app.renderTorNotice(app.cfg.Blogs["default"], true, "http://abc.onion:80/test")
	_, err := goquery.NewDocumentFromReader(strings.NewReader(string(res)))
	require.NoError(t, err)

	assert.Equal(t, template.HTML("<p id=\"tor\">🔐 Connected via Tor.</p>"), res)

	res = app.renderTorNotice(app.cfg.Blogs["default"], false, "http://abc.onion:80/test")
	_, err = goquery.NewDocumentFromReader(strings.NewReader(string(res)))
	require.NoError(t, err)

	assert.Equal(t, template.HTML("<p id=\"tor\">🔓 <a href=\"http://abc.onion:80/test\">Connect via Tor.</a> <a href=\"https://www.torproject.org/\" target=\"_blank\" rel=\"nofollow noopener noreferrer\">What is Tor?</a></p>"), res)
}
