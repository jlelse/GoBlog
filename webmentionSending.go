package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/thoas/go-funk"
	"github.com/tomnomnom/linkheader"
	"go.goblog.app/app/pkgs/contenttype"
)

func (a *goBlog) sendWebmentions(p *post) error {
	if p.Status != statusPublished && p.Status != statusUnlisted {
		// Not published or unlisted
		return nil
	}
	if wm := a.cfg.Webmention; wm != nil && wm.DisableSending {
		// Just ignore the mentions
		return nil
	}
	links := []string{}
	contentLinks, err := allLinksFromHTML(strings.NewReader(string(a.postHtml(p))), a.fullPostURL(p))
	if err != nil {
		return err
	}
	links = append(links, contentLinks...)
	links = append(links, p.firstParameter("link"), p.firstParameter(a.cfg.Micropub.LikeParam), p.firstParameter(a.cfg.Micropub.ReplyParam), p.firstParameter(a.cfg.Micropub.BookmarkParam))
	for _, link := range funk.UniqString(links) {
		if link == "" {
			continue
		}
		// Internal mention
		if strings.HasPrefix(link, a.cfg.Server.PublicAddress) {
			// Save mention directly
			if err := a.createWebmention(a.fullPostURL(p), link); err != nil {
				log.Println("Failed to create webmention:", err.Error())
			}
			continue
		}
		// External mention
		if pm := a.cfg.PrivateMode; pm != nil && pm.Enabled {
			// Private mode, don't send external mentions
			continue
		}
		// Send webmention
		endpoint := a.discoverEndpoint(link)
		if endpoint == "" {
			continue
		}
		if err = a.sendWebmention(endpoint, a.fullPostURL(p), link); err != nil {
			log.Println("Sending webmention to " + link + " failed")
			continue
		}
		log.Println("Sent webmention to " + link)
	}
	return nil
}

func (a *goBlog) sendWebmention(endpoint, source, target string) error {
	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(url.Values{
		"source": []string{source},
		"target": []string{target},
	}.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set(contentType, contenttype.WWWForm)
	req.Header.Set(userAgent, appUserAgent)
	res, err := a.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, res.Body)
	if code := res.StatusCode; code < 200 || 300 <= code {
		return fmt.Errorf("response error: %v", res.StatusCode)
	}
	return nil
}

func (a *goBlog) discoverEndpoint(urlStr string) string {
	doRequest := func(method, urlStr string) string {
		req, err := http.NewRequest(method, urlStr, nil)
		if err != nil {
			return ""
		}
		req.Header.Set(userAgent, appUserAgent)
		resp, err := a.httpClient.Do(req)
		if err != nil {
			return ""
		}
		defer resp.Body.Close()
		if code := resp.StatusCode; code < 200 || 300 <= code {
			_, _ = io.Copy(io.Discard, resp.Body)
			return ""
		}
		endpoint, err := extractEndpoint(resp)
		if err != nil || endpoint == "" {
			_, _ = io.Copy(io.Discard, resp.Body)
			return ""
		}
		if urls, err := resolveURLReferences(urlStr, endpoint); err == nil && len(urls) > 0 && urls[0] != "" {
			return urls[0]
		}
		return ""
	}
	headEndpoint := doRequest(http.MethodHead, urlStr)
	if headEndpoint != "" {
		return headEndpoint
	}
	getEndpoint := doRequest(http.MethodGet, urlStr)
	if getEndpoint != "" {
		return getEndpoint
	}
	return ""
}

func extractEndpoint(resp *http.Response) (string, error) {
	// first check http link headers
	if endpoint := wmEndpointHTTPLink(resp.Header); endpoint != "" {
		return endpoint, nil
	}
	// then look in the HTML body
	endpoint, err := wmEndpointHTMLLink(resp.Body)
	if err != nil {
		return "", err
	}
	return endpoint, nil
}

func wmEndpointHTTPLink(headers http.Header) string {
	links := linkheader.ParseMultiple(headers[http.CanonicalHeaderKey("Link")]).FilterByRel("webmention")
	for _, link := range links {
		if u := link.URL; u != "" {
			return u
		}
	}
	return ""
}

func wmEndpointHTMLLink(r io.Reader) (string, error) {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return "", err
	}
	href, _ := doc.Find("a[href][rel=webmention],link[href][rel=webmention]").Attr("href")
	return href, nil
}
