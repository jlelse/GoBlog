package main

import (
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

func urlize(str string) string {
	newStr := ""
	for _, c := range strings.Split(strings.ToLower(str), "") {
		if c >= "a" && c <= "z" || c >= "A" && c <= "Z" || c >= "0" && c <= "9" {
			newStr += c
		} else if c == " " {
			newStr += "-"
		}
	}
	return newStr
}

func sortedStrings(s []string) []string {
	sort.Slice(s, func(i, j int) bool {
		return strings.ToLower(s[i]) < strings.ToLower(s[j])
	})
	return s
}

func generateRandomString(chars int) string {
	rand.Seed(time.Now().UnixNano())
	letters := []rune("abcdefghijklmnopqrstuvwxyz")
	b := make([]rune, chars)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func isAllowedHost(r *http.Request, hosts ...string) bool {
	if r.URL == nil {
		return false
	}
	rh := r.URL.Host
	switch r.URL.Scheme {
	case "http":
		rh = strings.TrimSuffix(rh, ":80")
	case "https":
		rh = strings.TrimSuffix(rh, ":443")
	}
	for _, host := range hosts {
		if rh == host {
			return true
		}
	}
	return false
}

func isAbsoluteURL(s string) bool {
	if !strings.HasPrefix(s, "https://") && !strings.HasPrefix(s, "http://") {
		return false
	}
	if _, err := url.Parse(s); err != nil {
		return false
	}
	return true
}

func allLinksFromHTML(r io.Reader, baseURL string) ([]string, error) {
	bu, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, err
	}
	links := []string{}
	doc.Find("a[href]").Each(func(_ int, item *goquery.Selection) {
		if href, exists := item.Attr("href"); exists {
			if ref, err := url.Parse(href); err == nil {
				links = append(links, bu.ResolveReference(ref).String())
			}
		}
	})
	return links, nil
}
