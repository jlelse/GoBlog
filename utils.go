package main

import (
	"bytes"
	"cmp"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"
	"github.com/araddon/dateparse"
	"github.com/c2h5oh/datasize"
	tdl "github.com/mergestat/timediff/locale"
	"github.com/microcosm-cc/bluemonday"
	"github.com/samber/lo"
	"go.goblog.app/app/pkgs/bufferpool"
	"go.goblog.app/app/pkgs/builderpool"
	"go.goblog.app/app/pkgs/contenttype"
	"go.goblog.app/app/pkgs/minify"
	"golang.org/x/net/html"
	"golang.org/x/text/language"
)

type contextKey string

func urlize(str string) string {
	needsMap := false
	for i := range len(str) {
		c := str[i]
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') {
			needsMap = true
			break
		}
	}
	if !needsMap {
		return str
	}
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
		}
		// Drop character
		return -1
	}, str)
}

func sortedStrings(s []string) []string {
	if len(s) <= 1 {
		return s
	}
	type sortItem struct {
		orig  string
		lower string
	}
	var stackBuf [16]sortItem
	var items []sortItem
	if len(s) <= len(stackBuf) {
		items = stackBuf[:len(s)]
	} else {
		items = make([]sortItem, len(s))
	}
	for i, v := range s {
		items[i] = sortItem{orig: v, lower: strings.ToLower(v)}
	}
	slices.SortFunc(items, func(a, b sortItem) int {
		return cmp.Compare(a.lower, b.lower)
	})
	for i, item := range items {
		s[i] = item.orig
	}
	return s
}

var defaultLetters = []rune("abcdefghijklmnopqrstuvwxyz")

func randomString(n int, allowedChars ...rune) string {
	if len(allowedChars) == 0 {
		allowedChars = append(allowedChars, defaultLetters...)
	}
	return lo.RandomString(n, allowedChars)
}

func isAbsoluteURL(s string) bool {
	if u, err := url.Parse(s); err != nil || !u.IsAbs() {
		return false
	}
	return true
}

// isHTTPURL reports whether s is an absolute http(s) URL with a host.
// This prevents dangerous schemes such as javascript: or data: from being
// stored and later rendered into href attributes.
func isHTTPURL(s string) bool {
	u, err := url.Parse(strings.TrimSpace(s))
	return err == nil && u.Host != "" && (u.Scheme == "http" || u.Scheme == "https")
}

// httpURLOrEmpty returns s without surrounding whitespace if it is an absolute
// http(s) URL with a host, otherwise an empty string.
func httpURLOrEmpty(s string) string {
	s = strings.TrimSpace(s)
	if !isHTTPURL(s) {
		return ""
	}
	return s
}

func allLinksFromHTML(r io.Reader, baseURL string) ([]string, error) {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, err
	}
	var links []string
	doc.Find("a[href]").Each(func(_ int, item *goquery.Selection) {
		if href, exists := item.Attr("href"); exists {
			links = append(links, href)
		}
	})
	links, err = resolveURLReferences(baseURL, links...)
	return lo.Uniq(links), err
}

func resolveURLReferences(base string, refs ...string) ([]string, error) {
	b, err := url.Parse(base)
	if err != nil {
		return nil, err
	}
	urls := make([]string, 0)
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

func lowerUnescapedPath(p string) string {
	return strings.ToLower(unescapedPath(p))
}

// rawQueryMayContain reports whether the raw query string of r could contain
// the given string after percent-decoding. It is a cheap pre-check to avoid
// parsing the query string when the string cannot be present.
//
// Percent-decoding only changes a query string if it contains '%', and '+'
// only decodes to a space. For strings that contain no space (such as
// parameter names), the raw query must therefore contain the string literally
// unless it contains '%'.
func rawQueryMayContain(r *http.Request, s string) bool {
	rawQuery := r.URL.RawQuery
	if rawQuery == "" {
		return false
	}
	return strings.Contains(rawQuery, s) || strings.Contains(rawQuery, "%")
}

type stringGroup struct {
	Identifier string
	Strings    []string
}

func groupStrings(toGroup []string) []stringGroup {
	if len(toGroup) == 0 {
		return []stringGroup{}
	}
	grouped := make(map[string][]string)
	for _, s := range toGroup {
		r, _ := utf8.DecodeRuneInString(s)
		key := string(unicode.ToUpper(r))
		grouped[key] = append(grouped[key], s)
	}
	stringGroups := make([]stringGroup, 0, len(grouped))
	for key, strs := range grouped {
		stringGroups = append(stringGroups, stringGroup{
			Identifier: key,
			Strings:    sortedStrings(strs),
		})
	}
	slices.SortFunc(stringGroups, func(a, b stringGroup) int {
		return cmp.Compare(strings.ToLower(a.Identifier), strings.ToLower(b.Identifier))
	})
	return stringGroups
}

// parseTime parses a timestamp, preferring RFC 3339 and falling back to
// the more lenient dateparse.
func parseTime(s string) (time.Time, error) {
	if d, err := time.Parse(time.RFC3339, s); err == nil {
		return d, nil
	}
	return dateparse.ParseLocal(s)
}

func toLocalSafe(s string) string {
	d, _ := toLocal(s)
	return d
}

func toLocal(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	d, err := parseTime(s)
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
	d, err := parseTime(s)
	if err != nil {
		return "", err
	}
	return d.UTC().Format(time.RFC3339), nil
}

func toLocalTime(date string) time.Time {
	if date == "" {
		return time.Time{}
	}
	d, err := parseTime(date)
	if err != nil {
		return time.Time{}
	}
	return d.Local()
}

const isoDateFormat = "2006-01-02"

func utcNowString() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func utcNowNanos() int64 {
	return time.Now().UTC().UnixNano()
}

type stringPair struct {
	First, Second string
}

func wordCount(s string) int {
	count := 0
	for range strings.FieldsSeq(s) {
		count++
	}
	return count
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

func mBytesString(size int64) string {
	return fmt.Sprintf("%.2f MB", datasize.ByteSize(size).MBytes()) //nolint:gosec
}

// Policy for cleaning HTML to text
var textPolicy = bluemonday.StrictPolicy().
	AllowElements("h1", "h2", "h3", "h4", "h5", "h6"). // Headers
	AllowElements("p").                                // Paragraphs
	AllowElements("ol", "ul", "li").                   // Lists
	AllowElements("blockquote")                        // Blockquotes

func htmlText(r io.Reader) (string, error) {
	// Filter HTML
	sanitized := bufferpool.Get()
	defer bufferpool.Put(sanitized)
	if err := textPolicy.SanitizeReaderToWriter(r, sanitized); err != nil {
		return "", err
	}
	raw := sanitized.Bytes()
	if len(raw) == 0 {
		return "", nil
	}
	// Fast path when no HTML tags remain
	if !bytes.ContainsRune(raw, '<') {
		return strings.TrimSpace(html.UnescapeString(string(raw))), nil
	}
	// Fast path for a single <p>...</p> (common for title rendering)
	trimmed := bytes.TrimSpace(raw)
	if bytes.HasPrefix(trimmed, []byte("<p>")) && bytes.HasSuffix(trimmed, []byte("</p>")) {
		inner := trimmed[3 : len(trimmed)-4]
		if !bytes.ContainsRune(inner, '<') {
			return strings.TrimSpace(html.UnescapeString(string(inner))), nil
		}
	}
	// Read into document
	doc, err := html.Parse(bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	// Parse text
	text := builderpool.Get()
	defer builderpool.Put(text)
	body := findBodyNode(doc)
	if body != nil && hasElementChild(body) {
		htmlElementChildrenToText(body, false, text)
	} else {
		// Input was probably just text, so just use the text
		htmlNodeToText(doc, text)
	}
	// Trim whitespace and return
	return strings.TrimSpace(text.String()), nil
}

func findBodyNode(n *html.Node) *html.Node {
	if n.Type == html.ElementNode && n.Data == "body" {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if b := findBodyNode(c); b != nil {
			return b
		}
	}
	return nil
}

func hasElementChild(n *html.Node) bool {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode {
			return true
		}
	}
	return false
}

func isParagraphTag(tag string) bool {
	switch tag {
	case "h1", "h2", "h3", "h4", "h5", "h6", "p", "ol", "ul", "li", "blockquote":
		return true
	}
	return false
}

func htmlElementChildrenToText(parent *html.Node, inOL bool, text *strings.Builder) {
	i := 0
	var numBuf [20]byte
	for c := parent.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode {
			continue
		}
		// Not first child and starts a new paragraph
		if i > 0 && isParagraphTag(c.Data) {
			_, _ = text.WriteString("\n\n")
		}
		// List item in ordered list
		if inOL && c.Data == "li" {
			_, _ = text.Write(strconv.AppendInt(numBuf[:0], int64(i+1), 10))
			_, _ = text.WriteString(". ")
		}
		if hasElementChild(c) {
			htmlElementChildrenToText(c, c.Data == "ol", text)
		} else {
			htmlNodeToText(c, text)
		}
		i++
	}
}

func htmlNodeToText(n *html.Node, text io.StringWriter) {
	if n.Type == html.TextNode {
		_, _ = text.WriteString(n.Data)
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		htmlNodeToText(c, text)
	}
}

func cleanHTMLText(s string) string {
	if s == "" {
		return ""
	}
	if !strings.ContainsAny(s, "<&\r\x00") {
		return strings.TrimSpace(s)
	}
	// htmlText policy is already strict enough
	r, _ := htmlText(strings.NewReader(s))
	return r
}

func containsStrings(s string, subStrings ...string) bool {
	for _, ss := range subStrings {
		if strings.Contains(s, ss) {
			return true
		}
	}
	return false
}

func noError[T any](t T, _ error) T {
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
		// Ensure Host header is set from URL if not already set
		if req.Host == "" && req.URL != nil && req.URL.Host != "" {
			req.Host = req.URL.Host
		}
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
	return newHandlerClient(handler).Do(req) //nolint:gosec
}

var (
	timeDiffSupportedLangs = [...]string{"en", "de", "es", "hi", "pt", "ru", "zh-CN"}
	timeDiffMatcher        = sync.OnceValue(func() language.Matcher {
		tags := make([]language.Tag, len(timeDiffSupportedLangs))
		for i, lang := range timeDiffSupportedLangs {
			tags[i] = language.Make(lang)
		}
		return language.NewMatcher(tags)
	})
	timeDiffLocaleMap   = map[string]tdl.Locale{}
	timeDiffLocaleMutex sync.RWMutex
)

func matchTimeDiffLocale(lang string) tdl.Locale {
	timeDiffLocaleMutex.RLock()
	locale, ok := timeDiffLocaleMap[lang]
	timeDiffLocaleMutex.RUnlock()
	if ok {
		return locale
	}
	timeDiffLocaleMutex.Lock()
	defer timeDiffLocaleMutex.Unlock()
	if locale, ok := timeDiffLocaleMap[lang]; ok {
		return locale
	}
	_, idx, _ := timeDiffMatcher().Match(language.Make(lang))
	locale = tdl.Locale(timeDiffSupportedLangs[idx])
	timeDiffLocaleMap[lang] = locale
	return locale
}

func stringToInt(s string) int {
	i, _ := strconv.Atoi(s)
	return i
}

func loStringNotEmpty(s string, _ int) bool {
	return s != ""
}

func truncateStringWithEllipsis(s string, l int) string {
	if tr := []rune(s); len(tr) > l && l > 3 {
		return string(tr[0:l-3]) + "…"
	}
	return s
}

// writeTracker records whether anything was written to the wrapped
// ResponseWriter, so that an error response can still be served if generation
// fails before the first byte was sent to the client.
type writeTracker struct {
	http.ResponseWriter
	wrote bool
}

func (t *writeTracker) Write(p []byte) (int, error) {
	// Swallow empty writes, as even a zero-length write commits the response
	if len(p) == 0 {
		return 0, nil
	}
	t.wrote = true
	return t.ResponseWriter.Write(p)
}

func (t *writeTracker) WriteHeader(statusCode int) {
	t.wrote = true
	t.ResponseWriter.WriteHeader(statusCode)
}

// respondWithMinified streams the output of write to w, minified according to
// the given media type. Generation and minification are pipelined through an
// io.Pipe. If an error occurs before anything was sent to the client, an error
// response is served instead.
func (a *goBlog) respondWithMinified(w http.ResponseWriter, r *http.Request, minifyType, responseContentType string, write func(io.Writer) error) {
	pr, pw := io.Pipe()
	go func() {
		_ = pw.CloseWithError(write(pw))
	}()
	w.Header().Set(contentType, responseContentType)
	tw := &writeTracker{ResponseWriter: w}
	err := minify.Get().Minify(minifyType, tw, pr)
	// Closing the reader unblocks the writer goroutine
	_ = pr.CloseWithError(err)
	if err == nil {
		return
	}
	a.error("Failed to generate response body", "err", err)
	// The status code can only be changed as long as nothing was sent
	if !tw.wrote {
		w.Header().Del(contentType)
		a.serveError(w, r, "Failed to generate response body", http.StatusInternalServerError)
	}
}

func (a *goBlog) respondWithMinifiedJSON(w http.ResponseWriter, r *http.Request, v any) {
	a.respondWithMinified(w, r, contenttype.JSON, contenttype.JSONUTF8, func(buf io.Writer) error {
		return json.NewEncoder(buf).Encode(v)
	})
}

// generateSecurePassword creates a secure random password of the given length
func generateSecurePassword(length int) (string, error) {
	letters := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%&*()-_=+[]{}|;:,.<>?/"
	runes := []rune(letters)
	runesLen := len(runes)
	sb := builderpool.Get()
	defer builderpool.Put(sb)
	sb.Grow(length)
	randRead := rand.Reader
	for range length {
		r, err := rand.Int(randRead, big.NewInt(int64(runesLen)))
		if err != nil {
			return "", err
		}
		sb.WriteRune(runes[r.Int64()])
	}
	return sb.String(), nil
}

func getHost(address string) string {
	u, err := url.Parse(address)
	if err != nil {
		return ""
	}
	return u.Host
}
