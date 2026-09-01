// Package minify provides HTML, CSS, and JS minification.
package minify

import (
	"sync"

	"github.com/tdewolff/minify/v2"
	mCss "github.com/tdewolff/minify/v2/css"
	mHtml "github.com/tdewolff/minify/v2/html"
	mJs "github.com/tdewolff/minify/v2/js"
	mJson "github.com/tdewolff/minify/v2/json"
	mXml "github.com/tdewolff/minify/v2/xml"
	"go.goblog.app/app/pkgs/contenttype"
)

var getMinifier = sync.OnceValue(func() *minify.M {
	m := minify.New()
	// HTML
	m.AddFunc(contenttype.HTML, (&mHtml.Minifier{
		KeepDocumentTags: true,
	}).Minify)
	// CSS
	m.AddFunc(contenttype.CSS, mCss.Minify)
	// JS
	m.AddFunc(contenttype.JS, mJs.Minify)
	// XML
	m.AddFunc(contenttype.XML, mXml.Minify)
	m.AddFunc(contenttype.RSS, mXml.Minify)
	m.AddFunc(contenttype.ATOM, mXml.Minify)
	// JSON
	m.AddFunc(contenttype.JSON, mJson.Minify)
	m.AddFunc(contenttype.JSONFeed, mJson.Minify)
	m.AddFunc(contenttype.AS, mJson.Minify)
	return m
})

// Get returns the minifier instance.
func Get() *minify.M {
	return getMinifier()
}
