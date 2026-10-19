package main

import (
	"bytes"
	"encoding/xml"
	"io"
	"net/http"

	"go.goblog.app/app/pkgs/contenttype"
	"go.goblog.app/app/pkgs/minify"
)

// xmlViewScript is embedded as the first child of an XML document so browsers
// execute it and replace the raw XML with a human-readable HTML view. Feed
// readers and crawlers ignore it.
type xmlViewScript struct {
	XMLName          xml.Name `xml:"http://www.w3.org/1999/xhtml script"`
	Src              string   `xml:"src,attr"`
	Defer            string   `xml:"defer,attr"`
	Integrity        string   `xml:"integrity,attr,omitempty"`
	DataCSS          string   `xml:"data-css,attr"`
	DataCSSIntegrity string   `xml:"data-css-integrity,attr,omitempty"`
	DataWebsite      string   `xml:"data-website,attr,omitempty"`
}

func (a *goBlog) newXMLViewScript(js string) xmlViewScript {
	return xmlViewScript{
		Src:              a.assetFileName(js),
		Defer:            "",
		Integrity:        a.assetFileHash(js),
		DataCSS:          a.assetFileName("css/styles.css"),
		DataCSSIntegrity: a.assetFileHash("css/styles.css"),
	}
}

func (s xmlViewScript) marshal() ([]byte, error) {
	return xml.Marshal(s)
}

// writeXMLWithViewScript writes an XML document with the view script injected
// as its first child.
func (a *goBlog) writeXMLWithViewScript(w http.ResponseWriter, script []byte, write func(io.Writer) error) {
	pr, pw := io.Pipe()
	go func() {
		_, _ = io.WriteString(pw, xml.Header)
		_ = pw.CloseWithError(write(&xmlScriptInjector{w: pw, script: script}))
	}()
	w.Header().Set(contentType, contenttype.XMLUTF8)
	_ = pr.CloseWithError(minify.Get().Minify(contenttype.XML, w, pr))
}

// xmlScriptInjector inserts a script element right after the opening tag of an
// XML document, before any other child element.
type xmlScriptInjector struct {
	w        io.Writer
	script   []byte
	buffer   []byte
	injected bool
}

func (i *xmlScriptInjector) Write(p []byte) (int, error) {
	if i.injected {
		return i.w.Write(p)
	}

	// Buffer everything until the root element's opening tag is complete.
	i.buffer = append(i.buffer, p...)
	end := bytes.IndexByte(i.buffer, '>')
	if end < 0 {
		// Nothing can be written yet, but all bytes were accepted.
		return len(p), nil
	}

	// Write the opening tag, insert the script, then write the rest.
	if _, err := i.w.Write(i.buffer[:end+1]); err != nil {
		return 0, err
	}
	if _, err := i.w.Write(i.script); err != nil {
		return 0, err
	}
	if _, err := i.w.Write(i.buffer[end+1:]); err != nil {
		return 0, err
	}
	i.injected = true
	i.buffer = nil
	return len(p), nil
}
