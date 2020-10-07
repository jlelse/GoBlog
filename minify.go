package main

import (
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
	minifier.AddFunc("text/html", mHtml.Minify)
	minifier.AddFunc("text/css", mCss.Minify)
	minifier.AddFunc("application/javascript", mJs.Minify)
	minifier.AddFunc("application/rss+xml", mXml.Minify)
	minifier.AddFunc("application/atom+xml", mXml.Minify)
	minifier.AddFunc("application/feed+json", mJson.Minify)
}
