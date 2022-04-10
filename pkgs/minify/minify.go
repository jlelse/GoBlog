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

type Minifier struct {
	i sync.Once
	m *minify.M
}

func (m *Minifier) init() {
	m.i.Do(func() {
		m.m = minify.New()
		// HTML
		m.m.AddFunc(contenttype.HTML, mHtml.Minify)
		// CSS
		m.m.AddFunc(contenttype.CSS, mCss.Minify)
		// JS
		m.m.AddFunc(contenttype.JS, mJs.Minify)
		// XML
		m.m.AddFunc(contenttype.XML, mXml.Minify)
		m.m.AddFunc(contenttype.RSS, mXml.Minify)
		m.m.AddFunc(contenttype.ATOM, mXml.Minify)
		// JSON
		m.m.AddFunc(contenttype.JSON, mJson.Minify)
		m.m.AddFunc(contenttype.JSONFeed, mJson.Minify)
		m.m.AddFunc(contenttype.AS, mJson.Minify)
	})
}

func (m *Minifier) Get() *minify.M {
	m.init()
	return m.m
}
