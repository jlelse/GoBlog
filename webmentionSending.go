package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/carlmjohnson/requests"
	"github.com/samber/lo"
	"github.com/tomnomnom/linkheader"
)

const postParamWebmention = "webmention"

func (a *goBlog) sendWebmentions(p *post) error {
	if p.Status != statusPublished && p.Visibility != visibilityPublic && p.Visibility != visibilityUnlisted {
		// Not published or unlisted
		return nil
	}
	if wm := a.cfg.Webmention; wm != nil && wm.DisableSending {
		// Just ignore the mentions
		return nil
	}
	if pp, ok := p.Parameters[postParamWebmention]; ok && len(pp) > 0 && pp[0] == "false" {
		// Ignore this post
		return nil
	}
	pr, pw := io.Pipe()
	go func() {
		a.postHtmlToWriter(pw, &postHtmlOptions{p: p})
		_ = pw.Close()
	}()
	links, err := allLinksFromHTML(pr, a.fullPostURL(p))
	_ = pr.CloseWithError(err)
	if err != nil {
		return err
	}
	for _, link := range lo.Uniq(links) {
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
		if a.isPrivate() {
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
	// TODO: Pass all tests from https://webmention.rocks/
	return requests.URL(endpoint).Client(a.httpClient).Method(http.MethodPost).
		BodyForm(url.Values{
			"source": []string{source},
			"target": []string{target},
		}).
		AddValidator(func(r *http.Response) error {
			if r.StatusCode < 200 || 300 <= r.StatusCode {
				return fmt.Errorf("HTTP %d", r.StatusCode)
			}
			return nil
		}).
		Fetch(context.Background())
}

func (a *goBlog) discoverEndpoint(urlStr string) string {
	doRequest := func(method, urlStr string) string {
		endpoint := ""
		if err := requests.URL(urlStr).Client(a.httpClient).Method(method).
			AddValidator(func(r *http.Response) error {
				if r.StatusCode < 200 || 300 <= r.StatusCode {
					return fmt.Errorf("HTTP %d", r.StatusCode)
				}
				return nil
			}).
			Handle(func(r *http.Response) error {
				end, err := extractEndpoint(r)
				if err != nil || end == "" {
					return errors.New("no webmention endpoint found")
				}
				endpoint = end
				return nil
			}).
			Fetch(context.Background()); err != nil {
			return ""
		}
		if urls, err := resolveURLReferences(urlStr, endpoint); err == nil && len(urls) > 0 && urls[0] != "" {
			return urls[0]
		}
		return ""
	}
	if headEndpoint := doRequest(http.MethodHead, urlStr); headEndpoint != "" {
		return headEndpoint
	}
	if getEndpoint := doRequest(http.MethodGet, urlStr); getEndpoint != "" {
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
