package main

import (
	"fmt"
	"html/template"
	"strings"
)

// This file includes some functions that render parts of the HTML

type htmlBuilder struct {
	strings.Builder
}

func (h *htmlBuilder) write(s string) {
	_, _ = h.WriteString(s)
}

func (h *htmlBuilder) writeEscaped(s string) {
	if len(s) == 0 {
		return
	}
	template.HTMLEscape(h, []byte(s))
}

func (h *htmlBuilder) writeAttribute(attr, val string) {
	h.write(` `)
	h.write(attr)
	h.write(`="`)
	h.writeEscaped(val)
	h.write(`"`)
}

func (h *htmlBuilder) writeElementOpen(tag string, attrs ...string) {
	h.write(`<`)
	h.write(tag)
	for i := 0; i < len(attrs); i += 2 {
		h.writeAttribute(attrs[i], attrs[i+1])
	}
	h.write(`>`)
}

func (h *htmlBuilder) writeElementClose(tag string) {
	h.write(`</`)
	h.write(tag)
	h.write(`>`)
}

func (h *htmlBuilder) html() template.HTML {
	return template.HTML(h.String())
}

// Render the HTML to show the list of post taxonomy values (tags, series, etc.)
func (a *goBlog) renderPostTax(p *post, b *configBlog) template.HTML {
	if b == nil || p == nil {
		return ""
	}
	var hb htmlBuilder
	// Iterate over all taxonomies
	for _, tax := range b.Taxonomies {
		// Get all sorted taxonomy values for this post
		if taxValues := sortedStrings(p.Parameters[tax.Name]); len(taxValues) > 0 {
			// Start new paragraph
			hb.writeElementOpen("p")
			// Add taxonomy name
			hb.writeElementOpen("strong")
			hb.writeEscaped(a.renderMdTitle(tax.Title))
			hb.writeElementClose("strong")
			hb.write(": ")
			// Add taxonomy values
			for i, taxValue := range taxValues {
				if i > 0 {
					hb.write(", ")
				}
				hb.writeElementOpen(
					"a",
					"class", "p-category",
					"rel", "tag",
					"href", b.getRelativePath(fmt.Sprintf("/%s/%s", tax.Name, urlize(taxValue))),
				)
				hb.writeEscaped(a.renderMdTitle(taxValue))
				hb.writeElementClose("a")
			}
			// End paragraph
			hb.writeElementClose("p")
		}
	}
	return hb.html()
}

// Render the HTML to show a warning for old posts
func (a *goBlog) renderOldContentWarning(p *post, b *configBlog) template.HTML {
	if b == nil || p == nil || !p.Old() {
		return ""
	}
	var hb htmlBuilder
	hb.writeElementOpen("strong", "class", "p border-top border-bottom")
	hb.writeEscaped(a.ts.GetTemplateStringVariant(b.Lang, "oldcontent"))
	hb.writeElementClose("strong")
	return hb.html()
}

// Render the HTML to show interactions
func (a *goBlog) renderInteractions(b *configBlog, canonical string) template.HTML {
	if b == nil || canonical == "" {
		return ""
	}
	var hb htmlBuilder
	// Start accordion
	hb.writeElementOpen("details", "class", "p", "id", "interactions")
	hb.writeElementOpen("summary")
	hb.writeElementOpen("strong")
	hb.writeEscaped(a.ts.GetTemplateStringVariant(b.Lang, "interactions"))
	hb.writeElementClose("strong")
	hb.writeElementClose("summary")
	// Render mentions
	var renderMentions func(m []*mention)
	renderMentions = func(m []*mention) {
		if len(m) == 0 {
			return
		}
		hb.writeElementOpen("ul")
		for _, mention := range m {
			hb.writeElementOpen("li")
			hb.writeElementOpen("a", "href", mention.Url, "target", "_blank", "rel", "nofollow noopener noreferrer ugc")
			hb.writeEscaped(defaultIfEmpty(mention.Author, mention.Url))
			hb.writeElementClose("a")
			if mention.Title != "" {
				hb.write(" ")
				hb.writeElementOpen("strong")
				hb.writeEscaped(mention.Title)
				hb.writeElementClose("strong")
			}
			if mention.Content != "" {
				hb.write(" ")
				hb.writeElementOpen("i")
				hb.writeEscaped(mention.Content)
				hb.writeElementClose("i")
			}
			if len(mention.Submentions) > 0 {
				renderMentions(mention.Submentions)
			}
			hb.writeElementClose("li")
		}
		hb.writeElementClose("ul")
	}
	renderMentions(a.db.getWebmentionsByAddress(canonical))
	// Show form to send a webmention
	hb.writeElementOpen("form", "class", "fw p", "method", "post", "action", "/webmention")
	hb.writeElementOpen("label", "for", "wm-source", "class", "p")
	hb.writeEscaped(a.ts.GetTemplateStringVariant(b.Lang, "interactionslabel"))
	hb.writeElementClose("label")
	hb.writeElementOpen("input", "id", "wm-source", "type", "url", "name", "source", "placeholder", "URL", "required", "")
	hb.writeElementOpen("input", "type", "hidden", "name", "target", "value", canonical)
	hb.writeElementOpen("input", "type", "submit", "value", a.ts.GetTemplateStringVariant(b.Lang, "send"))
	hb.writeElementClose("form")
	// Show form to create a new comment
	hb.writeElementOpen("form", "class", "fw p", "method", "post", "action", "/comment")
	hb.writeElementOpen("input", "type", "hidden", "name", "target", "value", canonical)
	hb.writeElementOpen("input", "type", "text", "name", "name", "placeholder", a.ts.GetTemplateStringVariant(b.Lang, "nameopt"))
	hb.writeElementOpen("input", "type", "url", "name", "website", "placeholder", a.ts.GetTemplateStringVariant(b.Lang, "websiteopt"))
	hb.writeElementOpen("textarea", "name", "comment", "required", "", "placeholder", a.ts.GetTemplateStringVariant(b.Lang, "comment"))
	hb.writeElementClose("textarea")
	hb.writeElementOpen("input", "type", "submit", "value", a.ts.GetTemplateStringVariant(b.Lang, "docomment"))
	hb.writeElementClose("form")
	// Finish accordion
	hb.writeElementClose("details")
	return hb.html()
}

// Render HTML for author h-card
func (a *goBlog) renderAuthor() template.HTML {
	user := a.cfg.User
	if user == nil {
		return ""
	}
	var hb htmlBuilder
	hb.writeElementOpen("div", "class", "p-author h-card hide")
	if user.Picture != "" {
		hb.writeElementOpen("data", "class", "u-photo", "value", user.Picture)
		hb.writeElementClose("data")
	}
	if user.Name != "" {
		hb.writeElementOpen("a", "class", "p-name u-url", "rel", "me", "href", defaultIfEmpty(user.Link, "/"))
		hb.writeEscaped(user.Name)
		hb.writeElementClose("a")
	}
	hb.writeElementClose("div")
	return hb.html()
}

// Render HTML for TOR notice in the footer
func (a *goBlog) renderTorNotice(b *configBlog, torUsed bool, torAddress string) template.HTML {
	if !a.cfg.Server.Tor || b == nil || !torUsed && torAddress == "" {
		return ""
	}
	var hb htmlBuilder
	if torUsed {
		hb.writeElementOpen("p", "id", "tor")
		hb.writeEscaped("🔐 ")
		hb.writeEscaped(a.ts.GetTemplateStringVariant(b.Lang, "connectedviator"))
		hb.writeElementClose("p")
	} else if torAddress != "" {
		hb.writeElementOpen("p", "id", "tor")
		hb.writeEscaped("🔓 ")
		hb.writeElementOpen("a", "href", torAddress)
		hb.writeEscaped(a.ts.GetTemplateStringVariant(b.Lang, "connectviator"))
		hb.writeElementClose("a")
		hb.writeEscaped(" ")
		hb.writeElementOpen("a", "href", "https://www.torproject.org/", "target", "_blank", "rel", "nofollow noopener noreferrer")
		hb.writeEscaped(a.ts.GetTemplateStringVariant(b.Lang, "whatistor"))
		hb.writeElementClose("a")
		hb.writeElementClose("p")
	}
	return hb.html()
}
