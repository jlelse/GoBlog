package main

import (
	"crypto/sha256"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/PuerkitoBio/goquery"
	"github.com/araddon/dateparse"
	"github.com/thoas/go-funk"
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

const randomLetters = "abcdefghijklmnopqrstuvwxyz"

func generateRandomString(chars int) string {
	return funk.RandomString(chars, []rune(randomLetters))
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
	if u, err := url.Parse(s); err != nil || !u.IsAbs() {
		return false
	}
	return true
}

func allLinksFromHTMLString(html, baseURL string) ([]string, error) {
	return allLinksFromHTML(strings.NewReader(html), baseURL)
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
	return funk.UniqString(links), err
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

type stringGroup struct {
	Identifier string
	Strings    []string
}

func groupStrings(toGroup []string) []stringGroup {
	stringMap := map[string][]string{}
	for _, s := range toGroup {
		first := strings.ToUpper(string([]rune(s)[0]))
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

func dateFormat(date string, format string) string {
	d, err := dateparse.ParseLocal(date)
	if err != nil {
		return ""
	}
	return d.Local().Format(format)
}

func isoDateFormat(date string) string {
	return dateFormat(date, "2006-01-02")
}

func unixToLocalDateString(unix int64) string {
	return time.Unix(unix, 0).Local().String()
}

func localNowString() string {
	return time.Now().Local().String()
}

type stringPair struct {
	First, Second string
}

func wordCount(s string) int {
	return len(strings.Fields(s))
}

// Count all letters and numbers in string
func charCount(s string) (count int) {
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			count++
		}
	}
	return count
}

func wrapStringAsHTML(s string) template.HTML {
	return template.HTML(s)
}

// Check if url has allowed file extension
func urlHasExt(rawUrl string, allowed ...string) (ext string, has bool) {
	u, err := url.Parse(rawUrl)
	if err != nil {
		return "", false
	}
	ext = strings.ToLower(path.Ext(u.Path))
	if ext == "" {
		return "", false
	}
	ext = ext[1:]
	allowed = funk.Map(allowed, func(str string) string {
		return strings.ToLower(str)
	}).([]string)
	return ext, funk.ContainsString(allowed, strings.ToLower(ext))
}

// Get SHA-256 hash of file
func getSHA256(file io.ReadSeeker) (filename string, err error) {
	if _, err = file.Seek(0, 0); err != nil {
		return "", err
	}
	h := sha256.New()
	if _, err = io.Copy(h, file); err != nil {
		return "", err
	}
	if _, err = file.Seek(0, 0); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
