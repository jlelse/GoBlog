package minify

import (
	"io"
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
		m.m.AddFunc(contenttype.HTML, mHtml.Minify)
		m.m.AddFunc(contenttype.CSS, mCss.Minify)
		m.m.AddFunc(contenttype.XML, mXml.Minify)
		m.m.AddFunc(contenttype.JS, mJs.Minify)
		m.m.AddFunc(contenttype.RSS, mXml.Minify)
		m.m.AddFunc(contenttype.ATOM, mXml.Minify)
		m.m.AddFunc(contenttype.JSONFeed, mJson.Minify)
		m.m.AddFunc(contenttype.AS, mJson.Minify)
	})
}

func (m *Minifier) Get() *minify.M {
	m.init()
	return m.m
}

func (m *Minifier) Write(w io.Writer, mediatype string, b []byte) (int, error) {
	mw := m.Get().Writer(mediatype, w)
	defer mw.Close()
	return mw.Write(b)
}

func (m *Minifier) MinifyBytes(mediatype string, b []byte) ([]byte, error) {
	return m.Get().Bytes(mediatype, b)
}

func (m *Minifier) MinifyString(mediatype string, s string) (string, error) {
	return m.Get().String(mediatype, s)
}

func (m *Minifier) Minify(mediatype string, w io.Writer, r io.Reader) error {
	return m.Get().Minify(mediatype, w, r)
}
