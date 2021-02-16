package main

import (
	"io"

	"github.com/tdewolff/minify/v2"
	mCss "github.com/tdewolff/minify/v2/css"
	mHtml "github.com/tdewolff/minify/v2/html"
	mJs "github.com/tdewolff/minify/v2/js"
	mJson "github.com/tdewolff/minify/v2/json"
	mXml "github.com/tdewolff/minify/v2/xml"
)

var minifier *minify.M

func initMinify() {
	minifier = minify.New()
	minifier.AddFunc(contentTypeHTML, mHtml.Minify)
	minifier.AddFunc("text/css", mCss.Minify)
	minifier.AddFunc("text/xml", mXml.Minify)
	minifier.AddFunc("application/javascript", mJs.Minify)
	minifier.AddFunc(contentTypeRSS, mXml.Minify)
	minifier.AddFunc(contentTypeATOM, mXml.Minify)
	minifier.AddFunc(contentTypeJSONFeed, mJson.Minify)
	minifier.AddFunc(contentTypeAS, mJson.Minify)
}

func writeMinified(w io.Writer, mediatype string, b []byte) (int, error) {
	mw := minifier.Writer(mediatype, w)
	defer func() { mw.Close() }()
	return mw.Write(b)
}
