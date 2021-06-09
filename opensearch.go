package main

import (
	"fmt"
	"net/http"
)

func (a *goBlog) serveOpenSearch(w http.ResponseWriter, r *http.Request) {
	blog := r.Context().Value(blogContextKey).(string)
	b := a.cfg.Blogs[blog]
	title := b.Title
	sURL := a.cfg.Server.PublicAddress + b.getRelativePath(b.Search.Path)
	xml := fmt.Sprintf("<?xml version=\"1.0\"?><OpenSearchDescription xmlns=\"http://a9.com/-/spec/opensearch/1.1/\" xmlns:moz=\"http://www.mozilla.org/2006/browser/search/\">"+
		"<ShortName>%s</ShortName><Description>%s</Description>"+
		"<Url type=\"text/html\" method=\"post\" template=\"%s\"><Param name=\"q\" value=\"{searchTerms}\" /></Url>"+
		"<moz:SearchForm>%s</moz:SearchForm>"+
		"</OpenSearchDescription>",
		title, title, sURL, sURL)
	w.Header().Set(contentType, "application/opensearchdescription+xml")
	writeMinified(w, contentTypeXML, []byte(xml))
}

func openSearchUrl(b *configBlog) string {
	if b.Search != nil && b.Search.Enabled {
		return b.getRelativePath(b.Search.Path + "/opensearch.xml")
	}
	return ""
}
