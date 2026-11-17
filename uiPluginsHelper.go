package main

import (
	"io"
	"sync"

	"github.com/PuerkitoBio/goquery"
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
	var wg sync.WaitGroup
	pr, pw := io.Pipe()
	finish = func() {
		_ = pw.Close()
		wg.Wait()
	}
	wg.Go(func() {
		defer func() {
			if r := recover(); r != nil {
				a.error("Plugin HTML rendering panicked", "type", pluginType, "err", r)
			}
			// Drain unread input so the renderer does not block on the pipe.
			_, _ = io.Copy(io.Discard, pr)
			_ = pr.Close()
		}()
		doc, err := goquery.NewDocumentFromReader(pr)
		_ = pr.CloseWithError(err)
		for i, plugin := range plugins {
			a.callPlugin(pluginType, i, func() {
				pluginRender(plugin, doc)
			})
		}
		finalSelection := doc.Selection
		if selector != "" {
			finalSelection = finalSelection.Find(selector).Children()
		}
		finalSelection.Each(func(_ int, s *goquery.Selection) {
			_ = goquery.Render(originalWriter, s)
		})
	})
	return htmlbuilder.NewHTMLBuilder(pw), finish
}
