package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.goblog.app/app/pkgs/htmlbuilder"
	"go.goblog.app/app/pkgs/plugintypes"
)

var benchmarkPageContent = strings.Repeat(`<article class="h-entry border-bottom"><h2 class="p-name"><a href="/x">Title</a></h2><p>Some summary text with <a href="/y">a link</a> and more text to make it realistic.</p><div class="e-content"><p>Content</p></div></article>`, 24)

func BenchmarkRenderNoPlugins(b *testing.B) {
	app := &goBlog{cfg: createDefaultTestConfig(b)}
	require.NoError(b, app.initConfig(false))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	b.SetBytes(int64(len(benchmarkPageContent)))
	b.ReportAllocs()
	for b.Loop() {
		rr := httptest.NewRecorder()
		app.render(rr, req, func(hb *htmlbuilder.HTMLBuilder, _ *renderData) {
			hb.WriteUnescaped(benchmarkPageContent)
		}, &renderData{BlogString: app.cfg.DefaultBlog})
	}
}

type benchmarkUIPlugin struct{}

func (benchmarkUIPlugin) Render(_ plugintypes.RenderContext, rendered io.Reader, modified io.Writer) {
	_, _ = io.Copy(modified, rendered)
	_, _ = io.WriteString(modified, "<p>plugin</p>")
}

func BenchmarkChainUIPlugins(b *testing.B) {
	app := &goBlog{}
	plugins := []any{benchmarkUIPlugin{}, benchmarkUIPlugin{}}
	b.SetBytes(int64(len(benchmarkPageContent)))
	b.ReportAllocs()
	for b.Loop() {
		var out bytes.Buffer
		app.chainUIPlugins(plugins, &pluginRenderContext{}, strings.NewReader(benchmarkPageContent), &out)
	}
}
