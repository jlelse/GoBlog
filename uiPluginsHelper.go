package main

import (
	"bytes"
	"io"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"go.goblog.app/app/pkgs/bufferpool"
	"go.goblog.app/app/pkgs/htmlbuilder"
)

const (
	selectorBodyInner = "body"
)

func (a *goBlog) wrapForPlugins(
	originalWriter io.Writer,
	pluginType string,
	plugins []any,
	pluginRender func(plugin any, doc *goquery.Document),
	selector string,
) (wrappedHb *htmlbuilder.HTMLBuilder, finish func()) {
	if len(plugins) == 0 {
		// No plugins, nothing to wrap
		if hb, ok := (originalWriter).(*htmlbuilder.HTMLBuilder); ok {
			return hb, func() {}
		}
		return htmlbuilder.NewHTMLBuilder(originalWriter), func() {}
	}
	buf := bufferpool.Get()
	var once sync.Once
	finish = func() {
		once.Do(func() {
			a.renderPluginDocument(originalWriter, pluginType, plugins, pluginRender, selector, buf)
			bufferpool.Put(buf)
		})
	}
	return htmlbuilder.NewHTMLBuilder(buf), finish
}

// renderPluginDocument parses the buffered HTML, lets the plugins modify the
// document, and writes the selected part back to the original writer. Parsing
// a buffer instead of streaming through a pipe avoids a goroutine and a pipe
// per plugin type and fragment.
func (a *goBlog) renderPluginDocument(
	originalWriter io.Writer,
	pluginType string,
	plugins []any,
	pluginRender func(plugin any, doc *goquery.Document),
	selector string,
	buf *bytes.Buffer,
) {
	raw := buf.Bytes()
	wrote := false
	defer func() {
		if r := recover(); r != nil {
			a.error("Plugin HTML rendering panicked", "type", pluginType, "err", r)
			if !wrote {
				_, _ = originalWriter.Write(raw)
			}
		}
	}()
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(raw))
	if err != nil {
		_, _ = originalWriter.Write(raw)
		return
	}
	for i, plugin := range plugins {
		a.callPlugin(pluginType, i, func() {
			pluginRender(plugin, doc)
		})
	}
	finalSelection := doc.Selection
	if selector != "" {
		finalSelection = finalSelection.Find(selector).Children()
	}
	wrote = true
	finalSelection.Each(func(_ int, s *goquery.Selection) {
		_ = goquery.Render(originalWriter, s)
	})
}
