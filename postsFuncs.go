package main

import (
	"html/template"
	"log"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

func (a *goBlog) fullPostURL(p *post) string {
	return a.cfg.Server.PublicAddress + p.Path
}

func (a *goBlog) shortPostURL(p *post) string {
	s, err := a.db.shortenPath(p.Path)
	if err != nil {
		return ""
	}
	if a.cfg.Server.ShortPublicAddress != "" {
		return a.cfg.Server.ShortPublicAddress + s
	}
	return a.cfg.Server.PublicAddress + s
}

func (p *post) firstParameter(parameter string) (result string) {
	if pp := p.Parameters[parameter]; len(pp) > 0 {
		result = pp[0]
	}
	return
}

func (p *post) title() string {
	return p.firstParameter("title")
}

func (a *goBlog) html(p *post) template.HTML {
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

func (a *goBlog) absoluteHTML(p *post) template.HTML {
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

func (a *goBlog) summary(p *post) (summary string) {
	summary = p.firstParameter("summary")
	if summary != "" {
		return
	}
	html := string(a.html(p))
	if splitted := strings.Split(html, summaryDivider); len(splitted) > 1 {
		doc, _ := goquery.NewDocumentFromReader(strings.NewReader(splitted[0]))
		summary = doc.Text()
	} else {
		doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
		summary = doc.Find("p").First().Text()
	}
	return
}

func (a *goBlog) translations(p *post) []*post {
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
