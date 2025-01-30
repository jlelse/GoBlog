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

func (*goBlog) wrapForPlugins(
	originalWriter io.Writer,
	plugins []any,
	pluginRender func(plugin any, doc *goquery.Document),
	selector string,
) (wrappedHb *htmlbuilder.HtmlBuilder, finish func()) {
	if len(plugins) == 0 {
		// No plugins, nothing to wrap
		if hb, ok := (originalWriter).(*htmlbuilder.HtmlBuilder); ok {
			return hb, func() {}
		}
		return htmlbuilder.NewHtmlBuilder(originalWriter), func() {}
	}
	var wg sync.WaitGroup
	pr, pw := io.Pipe()
	finish = func() {
		_ = pw.Close()
		wg.Wait()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		doc, err := goquery.NewDocumentFromReader(pr)
		_ = pr.CloseWithError(err)
		for _, plugin := range plugins {
			pluginRender(plugin, doc)
		}
		finalSelection := doc.Selection
		if selector != "" {
			finalSelection = finalSelection.Find(selector).Children()
		}
		finalSelection.Each(func(_ int, s *goquery.Selection) {
			_ = goquery.Render(originalWriter, s)
		})
	}()
	return htmlbuilder.NewHtmlBuilder(pw), finish
}
