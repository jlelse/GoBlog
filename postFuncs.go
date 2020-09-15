package main

import (
	"github.com/PuerkitoBio/goquery"
	"strings"
)

func (p *Post) firstParameter(parameter string) (result string) {
	if pp := p.Parameters[parameter]; len(pp) > 0 {
		result = pp[0]
	}
	return
}

func (p *Post) title() string {
	return p.firstParameter("title")
}

func (p *Post) summary() (summary string) {
	summary = p.firstParameter("summary")
	if summary != "" {
		return
	}
	summaryDivider := "<!--more-->"
	rendered, _ := renderMarkdown(p.Content)
	if strings.Contains(string(rendered), summaryDivider) {
		doc, _ := goquery.NewDocumentFromReader(strings.NewReader(strings.Split(string(rendered), summaryDivider)[0]))
		summary = doc.Text()
		return
	}
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(string(rendered)))
	summary = firstSentences(doc.Text(), 3)
	return
}
