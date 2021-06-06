package main

import (
	"io"
	"sync"

	"github.com/tdewolff/minify/v2"
	mCss "github.com/tdewolff/minify/v2/css"
	mHtml "github.com/tdewolff/minify/v2/html"
	mJs "github.com/tdewolff/minify/v2/js"
	mJson "github.com/tdewolff/minify/v2/json"
	mXml "github.com/tdewolff/minify/v2/xml"
)

var (
	initMinify sync.Once
	minifier   *minify.M
)

func getMinifier() *minify.M {
	initMinify.Do(func() {
		minifier = minify.New()
		minifier.AddFunc(contentTypeHTML, mHtml.Minify)
		minifier.AddFunc("text/css", mCss.Minify)
		minifier.AddFunc(contentTypeXML, mXml.Minify)
		minifier.AddFunc("application/javascript", mJs.Minify)
		minifier.AddFunc(contentTypeRSS, mXml.Minify)
		minifier.AddFunc(contentTypeATOM, mXml.Minify)
		minifier.AddFunc(contentTypeJSONFeed, mJson.Minify)
		minifier.AddFunc(contentTypeAS, mJson.Minify)
	})
	return minifier
}

func writeMinified(w io.Writer, mediatype string, b []byte) (int, error) {
	mw := getMinifier().Writer(mediatype, w)
	defer func() { _ = mw.Close() }()
	return mw.Write(b)
}
