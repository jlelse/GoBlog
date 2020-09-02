package main

import (
	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/css"
	"github.com/tdewolff/minify/v2/html"
	"github.com/tdewolff/minify/v2/js"
	"github.com/tdewolff/minify/v2/json"
	"github.com/tdewolff/minify/v2/xml"
)

var minifier *minify.M

func initMinify() {
	minifier = minify.New()
	minifier.AddFunc("text/html", html.Minify)
	minifier.AddFunc("text/css", css.Minify)
	minifier.AddFunc("application/javascript", js.Minify)
	minifier.AddFunc("application/rss+xml", xml.Minify)
	minifier.AddFunc("application/atom+xml", xml.Minify)
	minifier.AddFunc("application/feed+json", json.Minify)
}
