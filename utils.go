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
	"github.com/araddon/dateparse"
)

type requestContextKey string

func urlize(str string) string {
	var sb strings.Builder
	for _, c := range strings.ToLower(str) {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' {
			_, _ = sb.WriteRune(c)
		} else if c == ' ' {
			_, _ = sb.WriteRune('-')
		}
	}
	return sb.String()
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
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, err
	}
	links := []string{}
	doc.Find("a[href]").Each(func(_ int, item *goquery.Selection) {
		if href, exists := item.Attr("href"); exists {
			links = append(links, href)
		}
	})
	links, err = resolveURLReferences(baseURL, links...)
	return links, err
}

func resolveURLReferences(base string, refs ...string) ([]string, error) {
	b, err := url.Parse(base)
	if err != nil {
		return nil, err
	}
	var urls []string
	for _, r := range refs {
		u, err := url.Parse(r)
		if err != nil {
			continue
		}
		urls = append(urls, b.ResolveReference(u).String())
	}
	return urls, nil
}

func unescapedPath(p string) string {
	if u, err := url.PathUnescape(p); err == nil {
		return u
	}
	return p
}

func slashIfEmpty(s string) string {
	if s == "" {
		return "/"
	}
	return s
}

type stringGroup struct {
	Identifier string
	Strings    []string
}

func groupStrings(toGroup []string) []stringGroup {
	stringMap := map[string][]string{}
	for _, s := range toGroup {
		first := strings.ToUpper(strings.Split(s, "")[0])
		stringMap[first] = append(stringMap[first], s)
	}
	stringGroups := []stringGroup{}
	for key, sa := range stringMap {
		stringGroups = append(stringGroups, stringGroup{
			Identifier: key,
			Strings:    sortedStrings(sa),
		})
	}
	sort.Slice(stringGroups, func(i, j int) bool {
		return strings.ToLower(stringGroups[i].Identifier) < strings.ToLower(stringGroups[j].Identifier)
	})
	return stringGroups
}

func toLocalSafe(s string) string {
	d, _ := toLocal(s)
	return d
}

func toLocal(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	d, err := dateparse.ParseLocal(s)
	if err != nil {
		return "", err
	}
	return d.Local().String(), nil
}
