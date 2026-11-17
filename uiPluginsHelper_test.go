package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"go.goblog.app/app/pkgs/htmlbuilder"
)

func TestWrapForPlugins(t *testing.T) {
	app := &goBlog{}
	var out bytes.Buffer
	hb, finish := app.wrapForPlugins(&out, "test", []any{struct{}{}}, func(_ any, doc *goquery.Document) {
		doc.Find("article").AppendHtml(`<p class="plugin">added</p>`)
	}, selectorBodyInner)
	hb.WriteUnescaped(`<article><p>original</p></article>`)
	finish()
	finish() // finish must be idempotent

	assert.Equal(t, `<article><p>original</p><p class="plugin">added</p></article>`, out.String())
}

func TestWrapForPluginsWithoutPlugins(t *testing.T) {
	app := &goBlog{}
	var out bytes.Buffer
	orig := htmlbuilder.NewHTMLBuilder(&out)
	hb, finish := app.wrapForPlugins(orig, "test", nil, func(any, *goquery.Document) {}, selectorBodyInner)
	assert.Same(t, orig, hb)
	finish()

	hb.WriteUnescaped("<p>unchanged</p>")
	assert.Equal(t, "<p>unchanged</p>", out.String())
}

func TestWrapForPluginsPluginPanicKeepsHTML(t *testing.T) {
	app := &goBlog{}
	var out bytes.Buffer
	hb, finish := app.wrapForPlugins(&out, "test", []any{struct{}{}}, func(any, *goquery.Document) {
		panic("plugin panic")
	}, selectorBodyInner)
	hb.WriteUnescaped(`<article><p>original</p></article>`)
	finish()

	assert.Equal(t, `<article><p>original</p></article>`, out.String())
}

var benchmarkPluginFragment = strings.Repeat(`<article class="h-entry border-bottom"><h2 class="p-name"><a href="/x">Title</a></h2><p>Some summary text with <a href="/y">a link</a> and more text to make it realistic.</p><div class="e-content"><p>Content</p></div></article>`, 4)

func BenchmarkWrapForPlugins(b *testing.B) {
	app := &goBlog{}
	plugins := []any{struct{}{}}
	pluginRender := func(_ any, doc *goquery.Document) {
		doc.Find("article").AppendHtml("<p>plugin</p>")
	}
	b.SetBytes(int64(len(benchmarkPluginFragment)))
	b.ReportAllocs()
	for b.Loop() {
		var out bytes.Buffer
		hb, finish := app.wrapForPlugins(&out, "summary", plugins, pluginRender, selectorBodyInner)
		hb.WriteUnescaped(benchmarkPluginFragment)
		finish()
	}
}
