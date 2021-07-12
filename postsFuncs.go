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

func postHasParameter(p *post, parameter string) bool {
	return len(p.Parameters[parameter]) > 0
}

func (p *post) firstParameter(parameter string) (result string) {
	if pp := p.Parameters[parameter]; len(pp) > 0 {
		result = pp[0]
	}
	return
}

func firstPostParameter(p *post, parameter string) string {
	return p.firstParameter(parameter)
}

func (a *goBlog) postHtml(p *post) template.HTML {
	if p.rendered != "" {
		return p.rendered
	}
	htmlContent, err := a.renderMarkdown(p.Content, false)
	if err != nil {
		log.Fatal(err)
		return ""
	}
	p.rendered = template.HTML(htmlContent)
	return p.rendered
}

func (a *goBlog) absolutePostHTML(p *post) template.HTML {
	if p.absoluteRendered != "" {
		return p.absoluteRendered
	}
	htmlContent, err := a.renderMarkdown(p.Content, true)
	if err != nil {
		log.Fatal(err)
		return ""
	}
	p.absoluteRendered = template.HTML(htmlContent)
	return p.absoluteRendered
}

const summaryDivider = "<!--more-->"

func (a *goBlog) postSummary(p *post) (summary string) {
	summary = p.firstParameter("summary")
	if summary != "" {
		return
	}
	html := string(a.postHtml(p))
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
	posts, err := a.db.getPosts(&postsRequestConfig{
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
	content := fmt.Sprintf("---\n%s---\n%s", string(pb), p.Content)
	return &microformatItem{
		Type: []string{"h-entry"},
		Properties: &microformatProperties{
			Name:       p.Parameters["title"],
			Published:  []string{p.Published},
			Updated:    []string{p.Updated},
			PostStatus: []string{string(p.Status)},
			Category:   p.Parameters[a.cfg.Micropub.CategoryParam],
			Content:    []string{content},
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

// Public because of rendering

func (p *post) Title() string {
	return p.firstParameter("title")
}

func (p *post) GeoURI() *gogeouri.Geo {
	loc := p.firstParameter("location")
	if loc == "" {
		return nil
	}
	g, _ := gogeouri.Parse(loc)
	return g
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
