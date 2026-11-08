// Package socialspec provides a SocialSpec plugin for GoBlog.
package socialspec

import (
	"net/http"
	"strings"
	"unicode/utf8"

	"go.goblog.app/app/pkgs/plugintypes"
	"go.yaml.in/yaml/v4"
)

const (
	followingPath = "/.well-known/following.opml"
	aboutPath     = "/.well-known/about.md"
	contactPath   = "/.well-known/contact.vcf"

	feedXMLAlias  = "/feed.xml"
	feedJSONAlias = "/feed.json"

	aboutContentType   = "text/markdown; charset=utf-8"
	contactContentType = "text/vcard; charset=utf-8"
)

type plugin struct {
	app           plugintypes.App
	blog          string
	aboutPage     string
	summary       string
	endpoints     []endpoint
	cachedHandler http.Handler
}

type endpoint struct {
	path        string
	contentType string
	body        string
	build       func() (string, bool, error)
}

type contactData struct {
	name   string
	emails []string
	urls   []string
	phones []string
	org    string
	title  string
	note   string
}

// GetPlugin returns the socialspec plugin instance.
func GetPlugin() (plugintypes.SetApp, plugintypes.SetConfig, plugintypes.Middleware) {
	p := &plugin{}
	return p, p, p
}

func (p *plugin) SetApp(app plugintypes.App) {
	p.app = app
	p.cachedHandler = app.CORSHandler(app.CacheMiddleware(http.HandlerFunc(p.serve)))
}

func (p *plugin) SetConfig(config map[string]any) {
	p.endpoints = nil
	p.blog = stringValue(config["blog"])

	title := stringValue(config["title"])
	p.summary = stringValue(config["summary"])
	p.aboutPage = firstString(config, "aboutpage", "aboutPage")
	if p.aboutPage != "" {
		p.endpoints = append(p.endpoints, endpoint{
			path:        aboutPath,
			contentType: aboutContentType,
			build:       p.buildAbout,
		})
	}

	if contact, ok := parseContact(config["contact"]); ok {
		if contact.name == "" {
			contact.name = title
		}
		if contact.name != "" {
			p.endpoints = append(p.endpoints, newEndpoint(contactPath, contactContentType, renderVCard(contact)))
		}
	}
}

func (p *plugin) buildAbout() (string, bool, error) {
	if p.app == nil {
		return "", false, nil
	}
	post, err := p.app.GetPost(p.aboutPage)
	if err != nil {
		return "", false, err
	}
	content, err := p.app.RewriteMarkdownURLs(post.GetContent())
	if err != nil {
		return "", false, err
	}
	summary := p.summary
	if summary == "" {
		summary = post.GetFirstParameterValue("summary")
	}
	return markdownWithSummary(content, summary), true, nil
}

func markdownWithSummary(markdown, summary string) string {
	if summary == "" {
		return markdown
	}
	frontMatter, err := yaml.Marshal(map[string]string{"summary": summary})
	if err != nil {
		return markdown
	}
	return "---\n" + string(frontMatter) + "---\n\n" + strings.TrimLeft(markdown, "\n")
}

func (p *plugin) Prio() int {
	return 1000
}

func (p *plugin) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !p.handlesPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		if p.cachedHandler == nil {
			p.serve(w, r)
			return
		}
		p.cachedHandler.ServeHTTP(w, r)
	})
}

func (p *plugin) handlesPath(path string) bool {
	if path == feedXMLAlias || path == feedJSONAlias || path == followingPath {
		return true
	}
	for i := range p.endpoints {
		if p.endpoints[i].path == path {
			return true
		}
	}
	return false
}

func (p *plugin) serve(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case feedXMLAlias, feedJSONAlias:
		p.serveFeedAlias(w, r)
		return
	case followingPath:
		p.serveBlogrollRedirect(w, r)
		return
	}

	var ep *endpoint
	for i := range p.endpoints {
		if p.endpoints[i].path == r.URL.Path {
			ep = &p.endpoints[i]
			break
		}
	}
	if ep == nil {
		w.Header().Set("Cache-Control", "no-store")
		http.NotFound(w, r)
		return
	}

	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	body := ep.body
	if ep.build != nil {
		var ok bool
		var err error
		body, ok, err = ep.build()
		if err != nil {
			w.Header().Set("Cache-Control", "no-store")
			http.Error(w, "failed to load SocialSpec data", http.StatusInternalServerError)
			return
		}
		if !ok {
			w.Header().Set("Cache-Control", "no-store")
			http.NotFound(w, r)
			return
		}
	}

	w.Header().Set("Content-Type", ep.contentType)
	_, _ = w.Write([]byte(body))
}

func (p *plugin) serveFeedAlias(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if p.app == nil {
		w.Header().Set("Cache-Control", "no-store")
		http.Error(w, "failed to resolve feed", http.StatusInternalServerError)
		return
	}
	feed := "rss"
	if r.URL.Path == feedJSONAlias {
		feed = "json"
	}
	target, err := p.app.GetBlogFeedPath(p.blog, feed)
	if err != nil {
		w.Header().Set("Cache-Control", "no-store")
		http.Error(w, "failed to resolve feed", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, target, http.StatusMovedPermanently)
}

func (p *plugin) serveBlogrollRedirect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if p.app == nil {
		w.Header().Set("Cache-Control", "no-store")
		http.Error(w, "failed to resolve blogroll", http.StatusInternalServerError)
		return
	}
	target, enabled, err := p.app.GetBlogrollExportPath(p.blog)
	if err != nil {
		w.Header().Set("Cache-Control", "no-store")
		http.Error(w, "failed to resolve blogroll", http.StatusInternalServerError)
		return
	}
	if !enabled {
		w.Header().Set("Cache-Control", "no-store")
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, target, http.StatusFound)
}

func newEndpoint(path, contentType, body string) endpoint {
	return endpoint{
		path:        path,
		contentType: contentType,
		body:        body,
	}
}

func parseContact(value any) (contactData, bool) {
	var contact contactData
	m, ok := value.(map[string]any)
	if !ok {
		return contact, false
	}
	contact.name = firstString(m, "name", "fn")
	contact.emails = stringList(m["email"])
	if len(contact.emails) == 0 {
		contact.emails = stringList(m["emails"])
	}
	contact.urls = stringList(m["url"])
	if len(contact.urls) == 0 {
		contact.urls = stringList(m["urls"])
	}
	if len(contact.urls) == 0 {
		contact.urls = stringList(m["website"])
	}
	contact.phones = stringList(m["phone"])
	if len(contact.phones) == 0 {
		contact.phones = stringList(m["phones"])
	}
	contact.org = firstString(m, "org", "organization")
	contact.title = firstString(m, "title", "role")
	contact.note = stringValue(m["note"])
	return contact, contact.name != "" || len(contact.emails) > 0 || len(contact.urls) > 0
}

func renderVCard(contact contactData) string {
	lines := []string{
		"BEGIN:VCARD",
		"VERSION:4.0",
		"FN:" + escapeVCard(contact.name),
	}
	for i, email := range contact.emails {
		if i == 0 {
			lines = append(lines, "EMAIL;PREF=1:"+escapeVCard(email))
		} else {
			lines = append(lines, "EMAIL:"+escapeVCard(email))
		}
	}
	for _, u := range contact.urls {
		lines = append(lines, "URL:"+u)
	}
	for _, phone := range contact.phones {
		lines = append(lines, "TEL:"+escapeVCard(phone))
	}
	if contact.org != "" {
		lines = append(lines, "ORG:"+escapeVCard(contact.org))
	}
	if contact.title != "" {
		lines = append(lines, "TITLE:"+escapeVCard(contact.title))
	}
	if contact.note != "" {
		lines = append(lines, "NOTE:"+escapeVCard(contact.note))
	}
	lines = append(lines, "END:VCARD")

	var b strings.Builder
	for _, line := range lines {
		folded := foldVCardLine(line)
		for i, part := range folded {
			if i > 0 {
				b.WriteString(" ")
			}
			b.WriteString(part)
			b.WriteString("\r\n")
		}
	}
	return b.String()
}

func foldVCardLine(line string) []string {
	const maxLength = 75
	var parts []string
	for len(line) > 0 {
		limit := maxLength
		if len(parts) > 0 {
			limit = maxLength - 1
		}
		if len(line) <= limit {
			parts = append(parts, line)
			break
		}
		cut := limit
		for cut > 0 && !utf8.RuneStart(line[cut]) {
			cut--
		}
		parts = append(parts, line[:cut])
		line = line[cut:]
	}
	return parts
}

func escapeVCard(value string) string {
	return strings.NewReplacer(
		"\\", "\\\\",
		";", "\\;",
		",", "\\,",
		"\r\n", "\\n",
		"\n", "\\n",
		"\r", "\\n",
	).Replace(value)
}

func firstString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := stringValue(m[key]); value != "" {
			return value
		}
	}
	return ""
}

func stringValue(value any) string {
	s, _ := value.(string)
	return s
}

func stringList(value any) []string {
	switch v := value.(type) {
	case string:
		if v == "" {
			return nil
		}
		return []string{v}
	case []string:
		return v
	case []any:
		var result []string
		for _, item := range v {
			if s := stringValue(item); s != "" {
				result = append(result, s)
			}
		}
		return result
	default:
		return nil
	}
}
