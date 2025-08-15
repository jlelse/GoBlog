package main

import (
	"cmp"
	"encoding/xml"
	"io"
	"net/http"

	"github.com/samber/lo"
	"go.goblog.app/app/pkgs/contenttype"
)

type openSearchDescription struct {
	XMLName     xml.Name                    `xml:"http://a9.com/-/spec/opensearch/1.1/ OpenSearchDescription"`
	Text        string                      `xml:",chardata"`
	ShortName   string                      `xml:"ShortName"`
	Description string                      `xml:"Description"`
	URL         []*openSearchDescriptionUrl `xml:"Url"`
	InputEnc    string                      `xml:"InputEncoding,omitempty"`
	OutputEnc   string                      `xml:"OutputEncoding,omitempty"`
	SearchForm  string                      `xml:"SearchForm"`
}

type openSearchDescriptionUrl struct {
	Text     string                         `xml:",chardata"`
	Type     string                         `xml:"type,attr"`
	Method   string                         `xml:"method,attr"`
	Rel      string                         `xml:"rel,attr,omitempty"`
	Template string                         `xml:"template,attr"`
	Param    *openSearchDescriptionUrlParam `xml:"Param,omitempty"`
}

type openSearchDescriptionUrlParam struct {
	Text  string `xml:",chardata"`
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

// serveOpenSearch serves the OpenSearch description XML for the blog.
// Spec: https://github.com/dewitt/opensearch/blob/master/opensearch-1-1-draft-6.md.
func (a *goBlog) serveOpenSearch(w http.ResponseWriter, r *http.Request) {
	_, b := a.getBlog(r)
	blogTitle := a.renderMdTitle(b.Title)
	sURL := a.getFullAddress(b.getRelativePath(cmp.Or(b.Search.Path, defaultSearchPath)))
	openSearch := &openSearchDescription{
		ShortName:   lo.Ellipsis(blogTitle, 16),   // ShortName must be <= 16 chars per spec
		Description: lo.Ellipsis(blogTitle, 1024), // Description must be <= 1024 chars per spec
		URL: []*openSearchDescriptionUrl{
			{
				Type: "text/html", Template: sURL + "?q={searchTerms}",
			},
			{
				Type: "text/html", Method: "post", Template: sURL,
				Param: &openSearchDescriptionUrlParam{
					Name: "q", Value: "{searchTerms}",
				},
			},
			{
				Type: "application/opensearchdescription+xml", Rel: "self",
				Template: a.getFullAddress(openSearchUrl(b)),
			},
		},
		InputEnc: "UTF-8", OutputEnc: "UTF-8",
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
		return b.getRelativePath(cmp.Or(b.Search.Path, defaultSearchPath) + "/opensearch.xml")
	}
	return ""
}
