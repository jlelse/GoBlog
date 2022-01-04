package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/PuerkitoBio/goquery"
	"github.com/araddon/dateparse"
	"github.com/c2h5oh/datasize"
	"github.com/microcosm-cc/bluemonday"
	"github.com/thoas/go-funk"
)

type contextKey string

func urlize(str string) string {
	return strings.Map(func(c rune) rune {
		if c >= 'a' && c <= 'z' || c >= '0' && c <= '9' {
			// Is lower case ASCII or number, return unmodified
			return c
		} else if c >= 'A' && c <= 'Z' {
			// Is upper case ASCII, make lower case
			return c + 'a' - 'A'
		} else if c == ' ' {
			// Space, replace with '-'
			return '-'
		} else {
			// Drop character
			return -1
		}
	}, str)
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
	return d.Local().Format(time.RFC3339), nil
}

func toUTCSafe(s string) string {
	d, _ := toUTC(s)
	return d
}

func toUTC(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	d, err := dateparse.ParseLocal(s)
	if err != nil {
		return "", err
	}
	return d.UTC().Format(time.RFC3339), nil
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
	return time.Unix(unix, 0).Local().Format(time.RFC3339)
}

func localNowString() string {
	return time.Now().Local().Format(time.RFC3339)
}

func utcNowString() string {
	return time.Now().UTC().Format(time.RFC3339)
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

// Get SHA-256 hash
func getSHA256(file io.ReadSeeker) (hash string, err error) {
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

func mBytesString(size int64) string {
	return fmt.Sprintf("%.2f MB", datasize.ByteSize(size).MBytes())
}

func htmlText(s string) string {
	// Build policy to only allow a subset of HTML tags
	textPolicy := bluemonday.StrictPolicy()
	textPolicy.AllowElements("h1", "h2", "h3", "h4", "h5", "h6") // Headers
	textPolicy.AllowElements("p")                                // Paragraphs
	textPolicy.AllowElements("ol", "ul", "li")                   // Lists
	textPolicy.AllowElements("blockquote")                       // Blockquotes
	// Filter HTML tags
	htmlBuf := textPolicy.SanitizeReader(strings.NewReader(s))
	// Read HTML into document
	doc, _ := goquery.NewDocumentFromReader(htmlBuf)
	var text strings.Builder
	if bodyChild := doc.Find("body").Children(); bodyChild.Length() > 0 {
		// Input was real HTML, so build the text from the body
		// Declare recursive function to print childs
		var printChilds func(childs *goquery.Selection)
		printChilds = func(childs *goquery.Selection) {
			childs.Each(func(i int, sel *goquery.Selection) {
				if i > 0 && // Not first child
					sel.Is("h1, h2, h3, h4, h5, h6, p, ol, ul, li, blockquote") { // All elements that start a new paragraph
					text.WriteString("\n\n")
				}
				if sel.Is("ol > li") { // List item in ordered list
					fmt.Fprintf(&text, "%d. ", i+1) // Add list item number
				}
				if sel.Children().Length() > 0 { // Has children
					printChilds(sel.Children()) // Recursive call to print childs
				} else {
					text.WriteString(sel.Text()) // Print text
				}
			})
		}
		printChilds(bodyChild)
	} else {
		// Input was probably just text, so just use the text
		text.WriteString(doc.Text())
	}
	// Trim whitespace and return
	return strings.TrimSpace(text.String())
}

func cleanHTMLText(s string) string {
	// Clean HTML with UGC policy and return text
	return htmlText(bluemonday.UGCPolicy().Sanitize(s))
}

func defaultIfEmpty(s, d string) string {
	return funk.ShortIf(s != "", s, d).(string)
}

func containsStrings(s string, subStrings ...string) bool {
	for _, ss := range subStrings {
		if strings.Contains(s, ss) {
			return true
		}
	}
	return false
}

func timeNoErr(t time.Time, _ error) time.Time {
	return t
}

type handlerRoundTripper struct {
	http.RoundTripper
	handler http.Handler
}

func (rt *handlerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if rt.handler != nil {
		// Fake request with handler
		rec := httptest.NewRecorder()
		rt.handler.ServeHTTP(rec, req)
		resp := rec.Result()
		// Copy request to response
		resp.Request = req
		return resp, nil
	}
	return nil, errors.New("no handler")
}

func newHandlerClient(handler http.Handler) *http.Client {
	return &http.Client{Transport: &handlerRoundTripper{handler: handler}}
}

func doHandlerRequest(req *http.Request, handler http.Handler) (*http.Response, error) {
	if req.URL.Path == "" {
		req.URL.Path = "/"
	}
	return newHandlerClient(handler).Do(req)
}

func saveToFile(reader io.Reader, fileName string) error {
	// Create folder path if not exists
	if err := os.MkdirAll(filepath.Dir(fileName), os.ModePerm); err != nil {
		return err
	}
	// Create file
	out, err := os.Create(fileName)
	if err != nil {
		return err
	}
	// Copy response to file
	defer out.Close()
	_, err = io.Copy(out, reader)
	return err
}

type valueOnlyContext struct {
	context.Context
}

func (valueOnlyContext) Deadline() (deadline time.Time, ok bool) {
	return
}

func (valueOnlyContext) Done() <-chan struct{} {
	return nil
}

func (valueOnlyContext) Err() error {
	return nil
}
