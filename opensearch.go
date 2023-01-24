package main

import (
	"encoding/xml"
	"io"
	"net/http"

	"go.goblog.app/app/pkgs/contenttype"
)

type openSearchDescription struct {
	XMLName     xml.Name                  `xml:"http://a9.com/-/spec/opensearch/1.1/ OpenSearchDescription"`
	Text        string                    `xml:",chardata"`
	ShortName   string                    `xml:"ShortName"`
	Description string                    `xml:"Description"`
	URL         *openSearchDescriptionUrl `xml:"Url"`
	SearchForm  string                    `xml:"http://www.mozilla.org/2006/browser/search/ SearchForm"`
}

type openSearchDescriptionUrl struct {
	Text     string                         `xml:",chardata"`
	Type     string                         `xml:"type,attr"`
	Method   string                         `xml:"method,attr"`
	Template string                         `xml:"template,attr"`
	Param    *openSearchDescriptionUrlParam `xml:"Param"`
}

type openSearchDescriptionUrlParam struct {
	Text  string `xml:",chardata"`
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

func (a *goBlog) serveOpenSearch(w http.ResponseWriter, r *http.Request) {
	_, b := a.getBlog(r)
	title := a.renderMdTitle(b.Title)
	sURL := a.getFullAddress(b.getRelativePath(defaultIfEmpty(b.Search.Path, defaultSearchPath)))
	openSearch := &openSearchDescription{
		ShortName:   title,
		Description: title,
		URL: &openSearchDescriptionUrl{
			Type:     "text/html",
			Method:   "post",
			Template: sURL,
			Param: &openSearchDescriptionUrlParam{
				Name:  "q",
				Value: "{searchTerms}",
			},
		},
		SearchForm: sURL,
	}
	pr, pw := io.Pipe()
	go func() {
		_, _ = io.WriteString(pw, xml.Header)
		_ = pw.CloseWithError(xml.NewEncoder(pw).Encode(openSearch))
	}()
	w.Header().Set(contentType, "application/opensearchdescription+xml"+contenttype.CharsetUtf8Suffix)
	_ = pr.CloseWithError(a.min.Get().Minify(contenttype.XML, w, pr))
}

func openSearchUrl(b *configBlog) string {
	if b.Search != nil && b.Search.Enabled {
		return b.getRelativePath(defaultIfEmpty(b.Search.Path, defaultSearchPath) + "/opensearch.xml")
	}
	return ""
}
