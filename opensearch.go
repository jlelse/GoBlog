package main

import (
	"bytes"
	"fmt"
	"net/http"

	"go.goblog.app/app/pkgs/contenttype"
)

func (a *goBlog) serveOpenSearch(w http.ResponseWriter, r *http.Request) {
	_, b := a.getBlog(r)
	title := a.renderMdTitle(b.Title)
	sURL := a.getFullAddress(b.getRelativePath(defaultIfEmpty(b.Search.Path, defaultSearchPath)))
	var buf bytes.Buffer
	_, _ = fmt.Fprintf(&buf, "<?xml version=\"1.0\"?><OpenSearchDescription xmlns=\"http://a9.com/-/spec/opensearch/1.1/\" xmlns:moz=\"http://www.mozilla.org/2006/browser/search/\">"+
		"<ShortName>%s</ShortName><Description>%s</Description>"+
		"<Url type=\"text/html\" method=\"post\" template=\"%s\"><Param name=\"q\" value=\"{searchTerms}\" /></Url>"+
		"<moz:SearchForm>%s</moz:SearchForm>"+
		"</OpenSearchDescription>",
		title, title, sURL, sURL)
	w.Header().Set(contentType, "application/opensearchdescription+xml")
	_ = a.min.Minify(contenttype.XML, w, &buf)
}

func openSearchUrl(b *configBlog) string {
	if b.Search != nil && b.Search.Enabled {
		return b.getRelativePath(defaultIfEmpty(b.Search.Path, defaultSearchPath) + "/opensearch.xml")
	}
	return ""
}
