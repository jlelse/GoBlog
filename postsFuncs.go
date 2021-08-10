package main

import (
	"fmt"
	"html/template"
	"log"
	"strings"
	"time"

	gogeouri "git.jlel.se/jlelse/go-geouri"
	"github.com/PuerkitoBio/goquery"
	"github.com/araddon/dateparse"
	"gopkg.in/yaml.v3"
)

func (a *goBlog) fullPostURL(p *post) string {
	return a.getFullAddress(p.Path)
}

func (a *goBlog) shortPostURL(p *post) string {
	s, err := a.db.shortenPath(p.Path)
	if err != nil {
		return ""
	}
	if a.cfg.Server.ShortPublicAddress != "" {
		return a.cfg.Server.ShortPublicAddress + s
	}
	return a.getFullAddress(s)
}

func postParameter(p *post, parameter string) []string {
	return p.Parameters[parameter]
}

func (p *post) firstParameter(parameter string) (result string) {
	if pp := p.Parameters[parameter]; len(pp) > 0 {
		result = pp[0]
	}
	return
}

func (a *goBlog) postHtml(p *post, absolute bool) template.HTML {
	p.renderMutex.RLock()
	// Check cache
	if r, ok := p.renderCache[absolute]; ok && r != "" {
		p.renderMutex.RUnlock()
		return r
	}
	p.renderMutex.RUnlock()
	// No cache, build it...
	p.renderMutex.Lock()
	defer p.renderMutex.Unlock()
	// Build HTML
	var htmlBuilder strings.Builder
	// Add audio to the top
	if audio, ok := p.Parameters[a.cfg.Micropub.AudioParam]; ok && len(audio) > 0 {
		for _, a := range audio {
			htmlBuilder.WriteString(`<audio controls preload=none><source src="`)
			htmlBuilder.WriteString(a)
			htmlBuilder.WriteString(`"/></audio>`)
		}
	}
	// Render markdown
	htmlContent, err := a.renderMarkdown(p.Content, absolute)
	if err != nil {
		log.Fatal(err)
		return ""
	}
	htmlBuilder.Write(htmlContent)
	// Add bookmark links to the bottom
	if link, ok := p.Parameters[a.cfg.Micropub.BookmarkParam]; ok && len(link) > 0 {
		for _, l := range link {
			htmlBuilder.WriteString(`<p><a class=u-bookmark-of href="`)
			htmlBuilder.WriteString(l)
			htmlBuilder.WriteString(`" target=_blank rel=noopener>`)
			htmlBuilder.WriteString(l)
			htmlBuilder.WriteString(`</a></p>`)
		}
	}
	// Cache
	html := template.HTML(htmlBuilder.String())
	if p.renderCache == nil {
		p.renderCache = map[bool]template.HTML{}
	}
	p.renderCache[absolute] = html
	return html
}

const summaryDivider = "<!--more-->"

func (a *goBlog) postSummary(p *post) (summary string) {
	summary = p.firstParameter("summary")
	if summary != "" {
		return
	}
	html := string(a.postHtml(p, false))
	if splitted := strings.Split(html, summaryDivider); len(splitted) > 1 {
		doc, _ := goquery.NewDocumentFromReader(strings.NewReader(splitted[0]))
		summary = doc.Text()
	} else {
		doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
		summary = doc.Find("p").First().Text()
	}
	return
}

func (a *goBlog) postTranslations(p *post) []*post {
	translationkey := p.firstParameter("translationkey")
	if translationkey == "" {
		return nil
	}
	posts, err := a.getPosts(&postsRequestConfig{
		parameter:      "translationkey",
		parameterValue: translationkey,
	})
	if err != nil || len(posts) == 0 {
		return nil
	}
	translations := []*post{}
	for _, t := range posts {
		if p.Path != t.Path {
			translations = append(translations, t)
		}
	}
	if len(translations) == 0 {
		return nil
	}
	return translations
}

func (p *post) isPublishedSectionPost() bool {
	return p.Published != "" && p.Section != "" && p.Status == statusPublished
}

func (a *goBlog) postToMfItem(p *post) *microformatItem {
	var mfStatus, mfVisibility string
	switch p.Status {
	case statusDraft:
		mfStatus = "draft"
	case statusPublished:
		mfStatus = "published"
		mfVisibility = "public"
	case statusUnlisted:
		mfStatus = "published"
		mfVisibility = "unlisted"
	case statusPrivate:
		mfStatus = "published"
		mfVisibility = "private"
	}
	return &microformatItem{
		Type: []string{"h-entry"},
		Properties: &microformatProperties{
			Name:       p.Parameters["title"],
			Published:  []string{p.Published},
			Updated:    []string{p.Updated},
			PostStatus: []string{mfStatus},
			Visibility: []string{mfVisibility},
			Category:   p.Parameters[a.cfg.Micropub.CategoryParam],
			Content:    []string{p.contentWithParams()},
			URL:        []string{a.fullPostURL(p)},
			InReplyTo:  p.Parameters[a.cfg.Micropub.ReplyParam],
			LikeOf:     p.Parameters[a.cfg.Micropub.LikeParam],
			BookmarkOf: p.Parameters[a.cfg.Micropub.BookmarkParam],
			MpSlug:     []string{p.Slug},
			Audio:      p.Parameters[a.cfg.Micropub.AudioParam],
			// TODO: Photos
		},
	}
}

func (a *goBlog) showFull(p *post) bool {
	if p.Section == "" {
		return false
	}
	sec, ok := a.cfg.Blogs[p.Blog].Sections[p.Section]
	return ok && sec != nil && sec.ShowFull
}

func (a *goBlog) geoURI(p *post) *gogeouri.Geo {
	loc := p.firstParameter(a.cfg.Micropub.LocationParam)
	if loc == "" {
		return nil
	}
	g, _ := gogeouri.Parse(loc)
	return g
}

func (a *goBlog) replyLink(p *post) string {
	return p.firstParameter(a.cfg.Micropub.ReplyParam)
}

func (a *goBlog) replyTitle(p *post) string {
	return p.firstParameter(a.cfg.Micropub.ReplyTitleParam)
}

func (a *goBlog) likeLink(p *post) string {
	return p.firstParameter(a.cfg.Micropub.LikeParam)
}

func (a *goBlog) likeTitle(p *post) string {
	return p.firstParameter(a.cfg.Micropub.LikeTitleParam)
}

func (a *goBlog) photoLinks(p *post) []string {
	return p.Parameters[a.cfg.Micropub.PhotoParam]
}

func (p *post) contentWithParams() string {
	params := map[string]interface{}{}
	for k, v := range p.Parameters {
		if l := len(v); l == 1 {
			params[k] = v[0]
		} else if l > 1 {
			params[k] = v
		}
	}
	params["path"] = p.Path
	params["section"] = p.Section
	params["blog"] = p.Blog
	params["published"] = p.Published
	params["updated"] = p.Updated
	params["status"] = string(p.Status)
	params["priority"] = p.Priority
	pb, _ := yaml.Marshal(params)
	return fmt.Sprintf("---\n%s---\n%s", string(pb), p.Content)
}

// Public because of rendering

func (p *post) Title() string {
	return p.firstParameter("title")
}

func (p *post) Old() bool {
	pub := p.Published
	if pub == "" {
		return false
	}
	pubDate, err := dateparse.ParseLocal(pub)
	if err != nil {
		return false
	}
	return pubDate.AddDate(1, 0, 0).Before(time.Now())
}
