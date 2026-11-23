package main

import (
	"cmp"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/samber/lo"
	"go.goblog.app/app/pkgs/contenttype"
	"go.goblog.app/app/pkgs/gpx"
	"go.goblog.app/app/pkgs/htmlbuilder"
	"go.goblog.app/app/pkgs/plugintypes"
)

type summaryTyp string

const (
	defaultSummary summaryTyp = "summary"
	photoSummary   summaryTyp = "photosummary"
)

// post summary on index pages
func (a *goBlog) renderSummary(origHb *htmlbuilder.HTMLBuilder, rd *renderData, bc *configBlog, p *post, typ summaryTyp) {
	if bc == nil || p == nil {
		return
	}
	if typ == "" {
		typ = defaultSummary
	}
	// Plugin handling
	hb, finish := a.wrapForPlugins(origHb, pluginUISummaryType, a.getPlugins(pluginUISummaryType), func(plugin any, doc *goquery.Document) {
		plugin.(plugintypes.UISummary).RenderSummaryForPost(rd.prc, p, doc)
	}, selectorBodyInner)
	defer finish()
	// Determine accessible name for article (used by screen readers)
	articleLabel := a.titleOrFallback(p)
	// Start article
	if p.RenderedTitle == "" && articleLabel != "" {
		hb.WriteElementOpen("article", "class", "h-entry border-bottom", "aria-label", articleLabel)
	} else {
		hb.WriteElementOpen("article", "class", "h-entry border-bottom")
	}
	if p.Priority > 0 {
		// Is pinned post
		hb.WriteTextElement("p", "📌 "+a.ts.GetTemplateStringVariant(bc.Lang, "pinned"))
	}
	if p.RenderedTitle != "" {
		// Has title
		hb.WriteElementOpen("h2", "class", "p-name")
		hb.WriteTextElement("a", p.RenderedTitle, "class", "u-url", "href", p.Path)
		hb.WriteElementClose("h2")
	}
	// Show photos in photo summary
	photos := a.photoLinks(p)
	if typ == photoSummary && len(photos) > 0 {
		for _, photo := range photos {
			hb.WriteElementOpen("p")
			a.writePictureElement(hb, photo, "", "", "u-photo", p.Path, false)
			hb.WriteElementClose("p")
		}
	}
	// Post meta
	a.renderPostMeta(hb, p, bc, "summary")
	if typ != photoSummary && a.showFull(p) {
		// Show full content
		a.postHTMLToWriter(hb, &postHTMLOptions{p: p})
	} else {
		// Show IndieWeb context
		a.renderPostReplyContext(hb, p)
		a.renderPostLikeContext(hb, p)
		// Show summary
		hb.WriteTextElement("p", a.postSummary(p), "class", "p-summary")
	}
	// Show link to full post
	hb.WriteElementOpen("p")
	written := 0
	if len(photos) > 0 {
		// Contains photos
		hb.WriteEscaped("🖼️")
		written++
	}
	if p.hasTrack() {
		// Has GPX track
		hb.WriteEscaped("🗺️")
		written++
	}
	if written > 0 {
		hb.WriteUnescaped("&nbsp;")
	}
	viewLabel := a.ts.GetTemplateStringVariant(bc.Lang, "view")
	if articleLabel != "" {
		hb.WriteTextElement("a", viewLabel, "class", "u-url", "href", p.Path, "aria-label", viewLabel+": "+articleLabel)
	} else {
		hb.WriteTextElement("a", viewLabel, "class", "u-url", "href", p.Path)
	}
	hb.WriteElementClose("p")
	// Finish article
	hb.WriteElementClose("article")
}

func (a *goBlog) renderPostMetaRow(hb *htmlbuilder.HTMLBuilder, p *post, b *configBlog) {
	type category struct{ name, href string }
	var categories []category
	for _, tax := range b.Taxonomies {
		for _, value := range p.Parameters[tax.Name] {
			if value != "" {
				categories = append(categories, category{a.renderMdTitle(value), b.getRelativePath("/" + tax.Name + "/" + urlize(value))})
			}
		}
	}
	slices.SortStableFunc(categories, func(a, b category) int {
		return strings.Compare(strings.ToLower(a.name), strings.ToLower(b.name))
	})
	published := toLocalTime(p.Published)
	section := b.Sections[p.Section]
	if published.IsZero() && section == nil && len(categories) == 0 {
		return
	}
	hb.WriteElementOpen("div", "class", "post-meta-row")
	needsSeparator := false
	separate := func() {
		if needsSeparator {
			hb.WriteUnescaped(" · ")
		}
		needsSeparator = true
	}
	if !published.IsZero() {
		separate()
		hb.WriteTextElement("time", published.Format(isoDateFormat), "class", "dt-published", "datetime", published.Format(time.RFC3339))
	}
	if section != nil {
		separate()
		hb.WriteTextElement("a", a.renderMdTitle(section.Title), "href", b.getRelativePath(section.Name))
	}
	for _, tag := range categories {
		separate()
		hb.WriteTextElement("a", tag.name, "class", "p-category", "rel", "tag", "href", tag.href)
	}
	hb.WriteElementClose("div")
}

// post meta information.
// typ can be "summary", "post" or "preview".
func (a *goBlog) renderPostMeta(hb *htmlbuilder.HTMLBuilder, p *post, b *configBlog, typ string) {
	if b == nil || p == nil || typ != "summary" && typ != "post" && typ != "preview" {
		return
	}
	hb.WriteElementOpen("div", "class", "p post-meta")
	a.renderPostMetaRow(hb, p, b)
	// Updated time
	if updated := toLocalTime(p.Updated); !updated.IsZero() {
		if typ == "summary" {
			hb.WriteElementOpen("time", "class", "dt-updated", "datetime", updated.Format(time.RFC3339), "hidden", "")
			hb.WriteElementClose("time")
		} else {
			hb.WriteElementOpen("div")
			hb.WriteEscaped(a.ts.GetTemplateStringVariant(b.Lang, "updatedon"))
			hb.WriteUnescaped(" ")
			hb.WriteTextElement("time", updated.Format(isoDateFormat), "class", "dt-updated", "datetime", updated.Format(time.RFC3339))
			hb.WriteElementClose("div")
		}
	}
	// Geo
	if geoURIs := a.geoURIs(p); len(geoURIs) != 0 {
		hb.WriteElementOpen("div")
		hb.WriteEscaped("📍 ")
		for i, geoURI := range geoURIs {
			if i > 0 {
				hb.WriteEscaped(", ")
			}
			hb.WriteElementOpen("a", "class", "p-location h-geo", "target", "_blank", "rel", "nofollow noopener noreferrer", "href", geoOSMLink(geoURI))
			hb.WriteTextElement("span", a.geoTitle(geoURI, b.Lang), "class", "p-name")
			hb.WriteElementOpen("data", "class", "p-longitude", "value", strconv.FormatFloat(geoURI.Longitude, 'f', 6, 64))
			hb.WriteElementClose("data")
			hb.WriteElementOpen("data", "class", "p-latitude", "value", strconv.FormatFloat(geoURI.Latitude, 'f', 6, 64))
			hb.WriteElementClose("data")
			hb.WriteElementClose("a")
		}
		hb.WriteElementClose("div")
	}
	// Post specific elements
	if typ == "post" {
		// Translations
		if translations := a.postTranslations(p); len(translations) > 0 {
			hb.WriteElementOpen("div")
			hb.WriteEscaped(a.ts.GetTemplateStringVariant(b.Lang, "translations"))
			hb.WriteEscaped(": ")
			for i, translation := range translations {
				if i > 0 {
					hb.WriteEscaped(", ")
				}
				hb.WriteTextElement("a", a.titleOrFallback(translation), "translate", "no", "href", translation.Path)
			}
			hb.WriteElementClose("div")
		}
		// Short link
		if shortLink := a.shortPostURL(p); shortLink != "" {
			hb.WriteElementOpen("div")
			hb.WriteEscaped(a.ts.GetTemplateStringVariant(b.Lang, "shorturl"))
			hb.WriteEscaped(" ")
			hb.WriteTextElement("a", shortLink, "rel", "shortlink", "href", shortLink)
			hb.WriteElementClose("div")
		}
		// Status
		if p.Status != statusPublished {
			hb.WriteTextElement("div", a.ts.GetTemplateStringVariant(b.Lang, "status")+": "+string(p.Status))
		}
		// Visibility
		if p.Visibility != visibilityPublic {
			hb.WriteTextElement("div", a.ts.GetTemplateStringVariant(b.Lang, "visibility")+": "+string(p.Visibility))
		}
	}
	hb.WriteElementClose("div")
}

// Reply ("u-in-reply-to")
func (a *goBlog) renderPostReplyContext(hb *htmlbuilder.HTMLBuilder, p *post) {
	a.renderPostLikeReplyContext(hb, "u-in-reply-to", a.ts.GetTemplateStringVariant(a.getBlogFromPost(p).Lang, "replyto"), a.replyLink(p), a.replyTitle(p), a.replyContext(p))
}

// Like ("u-like-of")
func (a *goBlog) renderPostLikeContext(hb *htmlbuilder.HTMLBuilder, p *post) {
	a.renderPostLikeReplyContext(hb, "u-like-of", a.ts.GetTemplateStringVariant(a.getBlogFromPost(p).Lang, "likeof"), a.likeLink(p), a.likeTitle(p), a.likeContext(p))
}

func (a *goBlog) renderPostLikeReplyContext(hb *htmlbuilder.HTMLBuilder, class, pretext, link, title, content string) {
	if link == "" {
		return
	}

	hb.WriteElementOpen("div", "class", "h-cite "+class)

	hb.WriteElementOpen("p")
	hb.WriteElementOpen("strong")
	hb.WriteEscaped(pretext)
	hb.WriteEscaped(": ")
	hb.WriteTextElement("a", lo.If(title != "", title).Else(link), "class", "u-url", "rel", "noopener", "target", "_blank", "href", link)
	hb.WriteElementClose("strong")
	hb.WriteElementClose("p")

	if content != "" {
		hb.WriteElementOpen("blockquote")
		hb.WriteTextElement("p", content, "class", "e-content")
		hb.WriteElementClose("blockquote")
	}

	hb.WriteElementClose("div")
}

// warning for old posts
func (a *goBlog) renderOldContentWarning(hb *htmlbuilder.HTMLBuilder, p *post, b *configBlog) {
	if b == nil || b.hideOldContentWarning || p == nil || !p.Old() {
		return
	}
	hb.WriteTextElement("strong", a.ts.GetTemplateStringVariant(b.Lang, "oldcontent"), "class", "p border-top border-bottom", "id", "oldcontentwarning")
}

func (a *goBlog) renderShareButton(hb *htmlbuilder.HTMLBuilder, p *post, b *configBlog) {
	if b == nil || b.hideShareButton || p == nil {
		return
	}

	// Share button
	hb.WriteTextElement(
		"button", a.ts.GetTemplateStringVariant(b.Lang, "share"),
		"type", "button",
		"id", "shareBtn",
		"class", "button",
	)

	// Share modal
	hb.WriteElementOpen("dialog", "id", "shareModal", "tabindex", "-1")
	hb.WriteElementOpen("div", "id", "shareModalHeader")
	hb.WriteTextElement("strong", a.ts.GetTemplateStringVariant(b.Lang, "sharemodalheading"))
	hb.WriteTextElement("button", "×", "type", "button", "class", "share-modal-close")
	hb.WriteElementClose("div")
	hb.WriteTextElement("button", a.ts.GetTemplateStringVariant(b.Lang, "sharenativeshare"), "id", "shareModalNative", "type", "button", "class", "button fw hide")
	hb.WriteTextElement("button", a.ts.GetTemplateStringVariant(b.Lang, "sharecopy"), "id", "shareModalCopy", "type", "button", "class", "button fw", "data-share-copy-feedback", a.ts.GetTemplateStringVariant(b.Lang, "sharecopyfeedback"))
	hb.WriteElement("div", "id", "shareModalServices")
	hb.WriteElementClose("dialog")
	hb.WriteElementOpen("script", "type", contenttype.JSON, "id", "shareData")
	_ = json.NewEncoder(hb).Encode(newShareData(a.titleOrFallback(p), a.fullPostURL(p)))
	hb.WriteElementClose("script")
	hb.WriteElement("script", "defer", "", "src", a.assetFileName("js/share.js"), "integrity", a.assetFileHash("js/share.js"))
}

func (a *goBlog) renderTranslateButton(hb *htmlbuilder.HTMLBuilder, p *post, b *configBlog) {
	if b == nil || b.hideTranslateButton {
		return
	}
	hb.WriteTextElement(
		"a", "A ⇄ 文", "id", "translateBtn",
		"class", "button",
		"href", fmt.Sprintf("https://translate.google.com/translate?u=%s", a.getFullAddress(p.Path)),
		"target", "_blank", "rel", "nofollow noopener noreferrer",
		"title", a.ts.GetTemplateStringVariant(b.Lang, "translate"),
		"translate", "no",
	)
	hb.WriteElementOpen("script", "defer", "", "src", a.assetFileName("js/translate.js"), "integrity", a.assetFileHash("js/translate.js"))
	hb.WriteElementClose("script")
}

func (a *goBlog) renderInteractions(hb *htmlbuilder.HTMLBuilder, rd *renderData) {
	// Start accordion
	hb.WriteElementOpen("details", "class", "p", "id", "interactions")
	hb.WriteElementOpen("summary")
	hb.WriteTextElement("strong", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "interactions"))
	hb.WriteElementClose("summary")
	// Render mentions
	var renderMentions func(m []*mention)
	renderMentions = func(m []*mention) {
		if len(m) == 0 {
			return
		}
		hb.WriteElementOpen("ul")
		for _, mention := range m {
			hb.WriteElementOpen("li")
			hb.WriteTextElement("a", cmp.Or(mention.Author, mention.URL), "href", mention.URL, "target", "_blank", "rel", "nofollow noopener noreferrer ugc")
			if mention.Title != "" {
				hb.WriteUnescaped(" ")
				hb.WriteTextElement("strong", mention.Title)
			}
			if mention.Content != "" {
				hb.WriteUnescaped(" ")
				hb.WriteTextElement("i", mention.Content)
			}
			if len(mention.Replies) > 0 {
				hb.WriteElementOpen("ul")
				for _, reply := range mention.Replies {
					hb.WriteElementOpen("li")
					hb.WriteTextElement("a", cmp.Or(a.cfg.User.Name, a.getFullAddress(reply.Path)), "href", reply.Path, "target", "_blank")
					if reply.RenderedTitle != "" {
						hb.WriteUnescaped(" ")
						hb.WriteTextElement("strong", reply.RenderedTitle)
					}
					if summary := a.postSummary(reply); summary != "" {
						hb.WriteUnescaped(" ")
						hb.WriteTextElement("i", summary)
					}
					hb.WriteElementClose("li")
				}
				hb.WriteElementClose("ul")
			}
			if len(mention.Submentions) > 0 {
				renderMentions(mention.Submentions)
			}
			hb.WriteElementClose("li")
		}
		hb.WriteElementClose("ul")
	}
	renderMentions(a.getWebmentionsByAddress(rd.Canonical))
	// Show form to send a webmention
	hb.WriteElementOpen("form", "class", "fw p", "method", "post", "action", "/webmention")
	hb.WriteTextElement("label", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "interactionslabel"), "for", "wm-source", "class", "p")
	hb.WriteElementOpen("input", "id", "wm-source", "type", "url", "name", "source", "placeholder", "URL", "required", "")
	hb.WriteElementOpen("input", "type", "hidden", "name", "target", "value", rd.Canonical)
	hb.WriteElementOpen("input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "send"))
	hb.WriteElementClose("form")
	// Show form to create a new comment
	hb.WriteElementOpen("form", "class", "fw p", "method", "post", "action", rd.Blog.getRelativePath(commentPath))
	hb.WriteElementOpen("input", "type", "hidden", "name", "target", "value", rd.Canonical)
	nameArgs := []any{"type", "text", "name", "name", "placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "nameopt")}
	websiteArgs := []any{"type", "url", "name", "website", "placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "websiteopt")}
	if rd.LoggedIn() && a.cfg.User != nil {
		if a.cfg.User.Name != "" {
			nameArgs = append(nameArgs, "value", a.cfg.User.Name)
		}
		if link := cmp.Or(a.cfg.User.Link, a.getFullAddress("/")); link != "" {
			websiteArgs = append(websiteArgs, "value", link)
		}
	}
	hb.WriteElementOpen("input", nameArgs...)
	hb.WriteElementOpen("input", websiteArgs...)
	hb.WriteElementOpen("textarea", "name", "comment", "required", "", "placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "comment"))
	hb.WriteElementClose("textarea")
	hb.WriteElementOpen("input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "docomment"))
	hb.WriteElementClose("form")
	// Finish accordion
	hb.WriteElementClose("details")
}

// author h-card
func (a *goBlog) renderAuthor(hb *htmlbuilder.HTMLBuilder) {
	user := a.cfg.User
	if user == nil {
		return
	}
	hb.WriteElementOpen("div", "class", "p-author h-card hide")
	if a.hasProfileImage() {
		hb.WriteElementOpen("data", "class", "u-photo", "value", a.getFullAddress(a.profileImagePath(profileImageFormatJPEG, 0)))
		hb.WriteElementClose("data")
	}
	if user.Name != "" {
		hb.WriteTextElement("a", user.Name, "class", "p-name u-url", "rel", "me", "href", cmp.Or(user.Link, "/"))
	}
	hb.WriteElementClose("div")
}

// head meta tags for a post
func (a *goBlog) renderPostHeadMeta(hb *htmlbuilder.HTMLBuilder, p *post) {
	if p == nil {
		return
	}
	if a.apEnabled() {
		if userHandle, ok := a.apUserHandle[p.Blog]; ok {
			hb.WriteElementOpen("meta", "name", "fediverse:creator", "property", "fediverse:creator", "content", userHandle)
		}
	}
}

func (a *goBlog) renderPostOpenGraphMeta(hb *htmlbuilder.HTMLBuilder, rd *renderData, p *post, article bool) {
	if p == nil {
		return
	}
	if article {
		hb.WriteElementOpen("meta", "property", "og:title", "content", a.titleOrFallback(p))
		if rd.Description != "" {
			hb.WriteElementOpen("meta", "property", "og:description", "content", rd.Description)
		}
	}
	if images := a.photoLinks(p); len(images) != 0 {
		hb.WriteElementOpen("meta", "property", "og:image", "content", a.absoluteMediaURL(a.mediaFallbackURL(images[0])))
	}
	if article {
		if published := toLocalTime(p.Published); !published.IsZero() {
			hb.WriteElementOpen("meta", "property", "article:published_time", "content", published.Format(time.RFC3339))
		}
		if updated := toLocalTime(p.Updated); !updated.IsZero() {
			hb.WriteElementOpen("meta", "property", "article:modified_time", "content", updated.Format(time.RFC3339))
		}
	}
}

func (a *goBlog) renderPostJSONLD(hb *htmlbuilder.HTMLBuilder, rd *renderData, p *post) {
	if p == nil {
		return
	}
	ld := postJSONLD{
		Context:    "https://schema.org",
		InLanguage: rd.Blog.Lang,
	}
	if rd.IsHome {
		ld.Type = "WebSite"
		ld.Name = a.renderMdTitle(rd.Blog.Title)
		ld.URL = a.getFullAddress(rd.Blog.getRelativePath(""))
	} else {
		ld.Type = "BlogPosting"
		ld.Headline = a.titleOrFallback(p)
		ld.Description = rd.Description
		if published := toLocalTime(p.Published); !published.IsZero() {
			ld.DatePublished = published.Format(time.RFC3339)
		}
		if updated := toLocalTime(p.Updated); !updated.IsZero() {
			ld.DateModified = updated.Format(time.RFC3339)
		}
		if rd.Canonical != "" {
			ld.MainEntityOfPage = rd.Canonical
		}
		if a.cfg.User != nil && a.cfg.User.Name != "" {
			ld.Author = &personJSONLD{
				Type: "Person",
				Name: a.cfg.User.Name,
				URL:  a.getFullAddress(a.getBlogFromPost(p).getRelativePath("")),
			}
		}
	}
	if images := a.photoLinks(p); len(images) != 0 {
		ld.Image = a.absoluteMediaURL(a.mediaFallbackURL(images[0]))
	}
	hb.WriteElementOpen("script", "type", contenttype.LDJSON)
	data, err := json.Marshal(&ld)
	if err == nil {
		hb.WriteUnescaped(string(data))
	}
	hb.WriteElementClose("script")
}

type postJSONLD struct {
	Context          string        `json:"@context"`
	Type             string        `json:"@type"`
	Name             string        `json:"name,omitempty"`
	URL              string        `json:"url,omitempty"`
	Headline         string        `json:"headline,omitempty"`
	Description      string        `json:"description,omitempty"`
	DatePublished    string        `json:"datePublished,omitempty"`
	DateModified     string        `json:"dateModified,omitempty"`
	MainEntityOfPage string        `json:"mainEntityOfPage,omitempty"`
	Author           *personJSONLD `json:"author,omitempty"`
	InLanguage       string        `json:"inLanguage,omitempty"`
	Image            string        `json:"image,omitempty"`
}

type personJSONLD struct {
	Type string `json:"@type"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

func (a *goBlog) renderPostHreflang(hb *htmlbuilder.HTMLBuilder, p *post) {
	if p == nil {
		return
	}
	translations := a.postTranslations(p)
	if len(translations) == 0 {
		return
	}
	writeHreflang := func(t *post) {
		hb.WriteElementOpen("link", "rel", "alternate", "hreflang", a.getBlogFromPost(t).Lang, "href", a.fullPostURL(t))
		if t.Blog == a.cfg.DefaultBlog {
			hb.WriteElementOpen("link", "rel", "alternate", "hreflang", "x-default", "href", a.fullPostURL(t))
		}
	}
	writeHreflang(p)
	for _, t := range translations {
		writeHreflang(t)
	}
}

// TOR notice in the footer
func (a *goBlog) renderTorNotice(hb *htmlbuilder.HTMLBuilder, rd *renderData) {
	if !a.cfg.Server.Tor || (!rd.TorUsed && rd.TorAddress == "") {
		return
	}
	if rd.TorUsed {
		hb.WriteTextElement("p", "🔐 "+a.ts.GetTemplateStringVariant(rd.Blog.Lang, "connectedviator"), "id", "tor")
	} else if rd.TorAddress != "" {
		hb.WriteElementOpen("p", "id", "tor")
		hb.WriteEscaped("🔓 ")
		hb.WriteTextElement("a", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "connectviator"), "href", rd.TorAddress)
		hb.WriteEscaped(" ")
		hb.WriteTextElement("a", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "whatistor"), "href", "https://www.torproject.org/", "target", "_blank", "rel", "nofollow noopener noreferrer")
		hb.WriteElementClose("p")
	}
}

func (a *goBlog) renderTitleTag(hb *htmlbuilder.HTMLBuilder, blog *configBlog, optionalTitle string) {
	if optionalTitle != "" {
		optionalTitle += " - "
	}
	hb.WriteTextElement("title", optionalTitle+a.renderMdTitle(blog.Title))
}

func (a *goBlog) renderPagination(hb *htmlbuilder.HTMLBuilder, blog *configBlog, p pagination) {
	if p.total < 2 {
		return
	}
	hb.WriteElementOpen("nav", "class", "pagination", "aria-label", a.ts.GetTemplateStringVariant(blog.Lang, "pagination"))
	if p.page > 1 {
		hb.WriteTextElement("a", "← "+a.ts.GetTemplateStringVariant(blog.Lang, "prev"), "href", p.url(p.page-1), "rel", "prev")
	}
	hb.WriteTextElement("span", fmt.Sprintf(a.ts.GetTemplateStringVariant(blog.Lang, "pageof"), p.page, p.total), "aria-current", "page")
	if p.page < p.total {
		hb.WriteTextElement("a", a.ts.GetTemplateStringVariant(blog.Lang, "next")+" →", "href", p.url(p.page+1), "rel", "next")
	}
	hb.WriteElementClose("nav")
}

func (a *goBlog) renderPostTitle(hb *htmlbuilder.HTMLBuilder, p *post) {
	if p == nil {
		return
	}
	if p.RenderedTitle != "" {
		hb.WriteTextElement("h1", p.RenderedTitle, "class", "p-name")
		return
	}
	// Untitled posts: use a fallback derived from the post for the heading.
	// The heading is visually hidden but remains in the accessibility tree.
	if fallback := a.fallbackTitle(p); fallback != "" {
		hb.WriteTextElement("h1", fallback, "class", "p-name visually-hidden", "aria-hidden", "true")
	}
}

func (a *goBlog) renderPostTrack(hb *htmlbuilder.HTMLBuilder, p *post, b *configBlog, disableInteractiveMap bool) {
	if p == nil || !p.hasTrack() {
		return
	}
	track, err := a.getTrack(p)
	if err != nil || track == nil {
		return
	}
	a.renderPostTrackStatistics(hb, track, b)
	if !disableInteractiveMap && p.showTrackRoute() && track.hasMapFeatures() {
		a.renderPostTrackMap(hb, track)
	} else if track.hasPath() {
		a.renderPostTrackSVG(hb, track)
	}
}

func (a *goBlog) renderPostTrackStatistics(hb *htmlbuilder.HTMLBuilder, track *trackResult, b *configBlog) {
	hb.WriteElementOpen("p")
	if track.Name != "" {
		hb.WriteTextElement("strong", track.Name)
		hb.WriteUnescaped(" ")
	}
	if track.Kilometers != "" {
		hb.WriteUnescaped("🏁 ")
		hb.WriteEscaped(track.Kilometers)
		hb.WriteUnescaped(" ")
		hb.WriteEscaped(a.ts.GetTemplateStringVariant(b.Lang, "kilometers"))
		hb.WriteUnescaped(" ")
	}
	if track.Hours != "" {
		hb.WriteUnescaped("⏱ ")
		hb.WriteEscaped(track.Hours)
		hb.WriteUnescaped(" ")
	}
	if track.Uphill != "" {
		hb.WriteUnescaped("⬆️ ")
		hb.WriteEscaped(track.Uphill)
		hb.WriteUnescaped(" ")
		hb.WriteEscaped(a.ts.GetTemplateStringVariant(b.Lang, "meters"))
		hb.WriteUnescaped(" ")
	}
	if track.Downhill != "" {
		hb.WriteUnescaped("⬇️ ")
		hb.WriteEscaped(track.Downhill)
		hb.WriteUnescaped(" ")
		hb.WriteEscaped(a.ts.GetTemplateStringVariant(b.Lang, "meters"))
	}
	hb.WriteElementClose("p")
}

func (a *goBlog) renderMap(hb *htmlbuilder.HTMLBuilder, dataAttrs ...any) {
	attribution := a.getMapAttribution()
	vector := a.vectorMapEnabled()
	attrs := make([]any, 0, 12+len(dataAttrs))
	attrs = append(attrs, "id", "map")
	attrs = append(attrs, dataAttrs...)
	attrs = append(attrs,
		"data-minzoom", a.getMinZoom(),
		"data-maxzoom", a.getMaxZoom(),
		"data-attribution", attribution,
		"data-vector", vector,
	)
	hb.WriteElementOpen("div", "class", "map-container p")
	hb.WriteElementOpen("div", attrs...)
	hb.WriteElementClose("div")
	if vector {
		hb.WriteElementOpen("p", "class", "map-attribution")
		hb.WriteUnescaped(attribution)
		hb.WriteElementClose("p")
	}
	hb.WriteElementClose("div")
	hb.WriteElementOpen("script", "defer", "", "src", a.assetFileName("js/geomap.js"), "integrity", a.assetFileHash("js/geomap.js"))
	hb.WriteElementClose("script")
}

func (a *goBlog) renderPostTrackMap(hb *htmlbuilder.HTMLBuilder, track *trackResult) {
	pathsJSON, _ := json.Marshal(track.Paths)
	pointsJSON, _ := json.Marshal(track.Points)
	a.renderMap(hb, "data-paths", string(pathsJSON), "data-points", string(pointsJSON))
}

func (a *goBlog) renderPostTrackSVG(hb *htmlbuilder.HTMLBuilder, track *trackResult) {
	const width, height = 700.0, 400.0
	// Calculate min/max values in Web Mercator projection
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, path := range track.Paths {
		for _, point := range path {
			x, y := gpx.WebMercatorX(point.Lon()), gpx.WebMercatorY(point.Lat())
			minX = math.Min(minX, x)
			maxX = math.Max(maxX, x)
			minY = math.Min(minY, y)
			maxY = math.Max(maxY, y)
		}
	}
	// Calculate scaling and offsets
	dataAspectRatio := (maxX - minX) / (maxY - minY)
	svgAspectRatio := width / height
	var scale, xOffset, yOffset float64
	if dataAspectRatio > svgAspectRatio {
		scale = width / (maxX - minX)
		yOffset = (height - (maxY-minY)*scale) / 2
	} else {
		scale = height / (maxY - minY)
		xOffset = (width - (maxX-minX)*scale) / 2
	}
	// Generate SVG
	hb.WriteElementOpen("svg", "width", "100%", "viewbox", fmt.Sprintf("0 0 %.0f %.0f", width, height))
	var ptBuf [64]byte
	for _, path := range track.Paths {
		_, _ = hb.WriteString(`<polyline points="`)
		for _, pt := range path {
			x := xOffset + (gpx.WebMercatorX(pt.Lon())-minX)*scale
			y := height - (yOffset + (gpx.WebMercatorY(pt.Lat())-minY)*scale)
			b := strconv.AppendFloat(ptBuf[:0], x, 'f', 2, 64)
			b = append(b, ',')
			b = strconv.AppendFloat(b, y, 'f', 2, 64)
			b = append(b, ' ')
			_, _ = hb.Write(b)
		}
		_, _ = hb.WriteString(`" fill="none" stroke="currentColor" stroke-width="3" />`)
	}
	hb.WriteElementClose("svg")
}

func (a *goBlog) renderPostReactions(hb *htmlbuilder.HTMLBuilder, p *post) {
	if !a.reactionsEnabledForPost(p) {
		return
	}
	hb.WriteElementOpen("div", "id", "reactions", "class", "actions", "data-path", p.Path, "data-allowed", strings.Join(a.getAllowedReactions(p.Blog), ","))
	hb.WriteElementClose("div")
	hb.WriteElementOpen("script", "defer", "", "src", a.assetFileName("js/reactions.js"), "integrity", a.assetFileHash("js/reactions.js"))
	hb.WriteElementClose("script")
}

func (a *goBlog) renderPostVideo(hb *htmlbuilder.HTMLBuilder, p *post) {
	if !p.hasVideoPlaylist() {
		return
	}
	hb.WriteElementOpen("div", "id", "video", "data-url", p.firstParameter(videoPlaylistParam))
	hb.WriteElementClose("div")
	hb.WriteElementOpen("script", "defer", "", "src", a.assetFileName("js/video.js"), "integrity", a.assetFileHash("js/video.js"))
	hb.WriteElementClose("script")
}

func (a *goBlog) renderReactionSettings(hb *htmlbuilder.HTMLBuilder, rd *renderData, srd *settingsRenderData) {
	hb.WriteTextElement("h2", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "reactions"))

	// Toggle
	a.renderBooleanSetting(hb, rd, rd.Blog.getRelativePath(settingsPath+settingsUpdateReactionsEnabledPath), a.ts.GetTemplateStringVariant(rd.Blog.Lang, "reactionsenableddesc"), "reactionsenabled", a.reactionsEnabled(srd.blog), false)

	if a.reactionsEnabled(srd.blog) {
		hb.WriteElementOpen("form", "class", "fw p", "method", "post")
		hb.WriteTextElement("p", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "reactionsdesc"))
		hb.WriteElementOpen("input", "type", "text", "name", "reactions", "value", strings.Join(a.getAllowedReactions(srd.blog), ","))
		hb.WriteElementOpen(
			"input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "update"),
			"formaction", rd.Blog.getRelativePath(settingsPath+settingsUpdateReactionsPath),
		)
		hb.WriteElementClose("form")
	}
}

func (a *goBlog) renderWebmentionSettings(hb *htmlbuilder.HTMLBuilder, rd *renderData, srd *settingsRenderData) {
	hb.WriteTextElement("h2", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "webmentions"))

	// Disable sending (global)
	a.renderBooleanSetting(hb, rd, rd.Blog.getRelativePath(settingsPath+settingsWebmentionDisableSendingPath), a.ts.GetTemplateStringVariant(rd.Blog.Lang, "disablesendingdesc"), webmentionDisableSendingSetting, srd.disableSendingWebmentions, true)

	// Disable receiving (global)
	a.renderBooleanSetting(hb, rd, rd.Blog.getRelativePath(settingsPath+settingsWebmentionDisableReceivingPath), a.ts.GetTemplateStringVariant(rd.Blog.Lang, "disablereceivingdesc"), webmentionDisableReceivingSetting, srd.disableReceivingWebmentions, true)

	// Disable inter-GoBlog mentions (global)
	a.renderBooleanSetting(hb, rd, rd.Blog.getRelativePath(settingsPath+settingsWebmentionDisableInterGoblogPath), a.ts.GetTemplateStringVariant(rd.Blog.Lang, "disableintergoblogmentionsdesc"), webmentionDisableInterGoblogSetting, srd.disableInterGoblogMentions, true)

	// Block list (global)
	hb.WriteElementOpen("details", "class", "settings-webmention-blocklist")
	hb.WriteElementOpen("summary")
	hb.WriteTextElement("h3", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "webmentionblocklist"))
	hb.WriteElementClose("summary")

	// Existing entries table
	if len(srd.webmentionBlocklist) > 0 {
		hb.WriteElementOpen("table", "class", "settings-table")
		hb.WriteElementOpen("tr")
		hb.WriteTextElement("th", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "blocklisthost"), "class", "expand")
		hb.WriteTextElement("th", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "blocklistincoming"), "class", "ct")
		hb.WriteTextElement("th", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "blocklistoutgoing"), "class", "ct")
		hb.WriteTextElement("th", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "delete"))
		hb.WriteElementClose("tr")
		for _, entry := range srd.webmentionBlocklist {
			hb.WriteElementOpen("tr")
			// Host
			hb.WriteTextElement("td", entry.Host, "class", "expand")
			// Incoming
			hb.WriteTextElement("td", lo.If(entry.Incoming, "✓").Else(""), "class", "fixed ct")
			// Outgoing
			hb.WriteTextElement("td", lo.If(entry.Outgoing, "✓").Else(""), "class", "fixed ct")
			// Delete action
			hb.WriteElementOpen("td", "class", "fixed")
			hb.WriteElementOpen("form", "method", "post")
			hb.WriteElementOpen("input", "type", "hidden", "name", "blocklisthost", "value", entry.Host)
			hb.WriteTextElement("button", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "delete"), "type", "submit", "formaction", rd.Blog.getRelativePath(settingsPath+settingsWebmentionBlocklistRemovePath), "class", "confirm", "data-confirmmessage", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "confirmdelete"))
			hb.WriteElementClose("form")
			hb.WriteElementClose("td")
			hb.WriteElementClose("tr")
		}
		hb.WriteElementClose("table")
	}

	// Add new entry form
	hb.WriteElementOpen("form", "class", "fw p", "method", "post")
	hb.WriteElementOpen("input", "type", "text", "name", "blocklisthost", "placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "blocklisthost"))
	hb.WriteElementOpen("input", "type", "checkbox", "name", "blocklistincoming", "id", "blocklistincoming")
	hb.WriteTextElement("label", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "blocklistincoming"), "for", "blocklistincoming")
	hb.WriteElementOpen("input", "type", "checkbox", "name", "blocklistoutgoing", "id", "blocklistoutgoing")
	hb.WriteTextElement("label", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "blocklistoutgoing"), "for", "blocklistoutgoing")
	hb.WriteElementOpen("input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "blocklistadd"), "formaction", rd.Blog.getRelativePath(settingsPath+settingsWebmentionBlocklistAddPath))
	hb.WriteElementClose("form")
	hb.WriteElementClose("details")
}

func (a *goBlog) renderPostSectionSettings(hb *htmlbuilder.HTMLBuilder, rd *renderData, srd *settingsRenderData) {
	hb.WriteTextElement("h2", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "postsections"))

	// Update default section
	hb.WriteTextElement("h3", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "default"))

	hb.WriteElementOpen("form", "class", "fw p", "method", "post")
	hb.WriteElementOpen("select", "name", "defaultsection")
	for _, section := range srd.sections {
		hb.WriteTextElement("option", section.Name, "value", section.Name, lo.If(section.Name == srd.defaultSection, "selected").Else(""), "")
	}
	hb.WriteElementClose("select")
	hb.WriteElementOpen(
		"input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "update"),
		"formaction", rd.Blog.getRelativePath(settingsPath+settingsUpdateDefaultSectionPath),
	)
	hb.WriteElementClose("form")

	for _, section := range srd.sections {
		hb.WriteElementOpen("details")

		hb.WriteElementOpen("summary")
		hb.WriteTextElement("h3", section.Name)
		hb.WriteElementClose("summary")

		hb.WriteElementOpen("form", "class", "fw p", "method", "post")

		hb.WriteElementOpen("input", "type", "hidden", "name", "sectionname", "value", section.Name)

		// Title
		hb.WriteElementOpen("input", "type", "text", "name", "sectiontitle", "placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "sectiontitle"), "required", "", "value", section.Title)
		// Description
		hb.WriteTextElement(
			"textarea", section.Description,
			"name", "sectiondescription",
			"class", "monospace",
			"placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "sectiondescription"),
		)
		// Path template
		hb.WriteElementOpen("input", "type", "text", "name", "sectionpathtemplate", "placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "sectionpathtemplate"), "value", section.PathTemplate)
		// Show full
		hb.WriteElementOpen("input", "type", "checkbox", "name", "sectionshowfull", "id", "showfull-"+section.Name, lo.If(section.ShowFull, "checked").Else(""), "")
		hb.WriteTextElement("label", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "sectionshowfull"), "for", "showfull-"+section.Name)
		hb.WriteElementsClose("br")
		// Hide on start
		hb.WriteElementOpen("input", "type", "checkbox", "name", "sectionhideonstart", "id", "hideonstart-"+section.Name, lo.If(section.HideOnStart, "checked").Else(""), "")
		hb.WriteTextElement("label", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "sectionhideonstart"), "for", "hideonstart-"+section.Name)

		// Actions
		hb.WriteElementOpen("div", "class", "p")
		// Update
		hb.WriteElementOpen(
			"input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "update"),
			"formaction", rd.Blog.getRelativePath(settingsPath+settingsUpdateSectionPath),
		)
		// Delete
		hb.WriteElementOpen(
			"input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "delete"),
			"formaction", rd.Blog.getRelativePath(settingsPath+settingsDeleteSectionPath),
			"class", "confirm", "data-confirmmessage", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "confirmdelete"),
		)
		hb.WriteElementClose("div")

		hb.WriteElementClose("form")
		hb.WriteElementClose("details")
	}

	// Create new section
	hb.WriteElementOpen("form", "class", "fw p", "method", "post")
	// Name
	hb.WriteElementOpen("input", "type", "text", "name", "sectionname", "placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "sectionname"), "required", "")
	// Title
	hb.WriteElementOpen("input", "type", "text", "name", "sectiontitle", "placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "sectiontitle"), "required", "")
	// Create button
	hb.WriteElementOpen("div")
	hb.WriteElementOpen(
		"input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "create"),
		"formaction", rd.Blog.getRelativePath(settingsPath+settingsUpdateSectionPath),
	)
	hb.WriteElementClose("div")
	hb.WriteElementClose("form")
}

func (a *goBlog) renderBooleanSetting(hb *htmlbuilder.HTMLBuilder, rd *renderData, path, description, name string, value, isGlobal bool) {
	if isGlobal {
		description = "🌐 " + description
	}
	hb.WriteElementOpen("form", "class", "fw p", "method", "post", "action", path)

	hb.WriteElementOpen("input", "type", "checkbox", "class", "autosubmit", "name", name, "id", "cb-"+name, lo.If(value, "checked").Else(""), "")
	hb.WriteTextElement("label", description, "for", "cb-"+name)

	hb.WriteElementOpen("noscript")
	hb.WriteElementOpen("div", "class", "p")
	hb.WriteElementOpen(
		"input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "update"),
	)
	hb.WriteElementClose("div")
	hb.WriteElementClose("noscript")

	hb.WriteElementClose("form")
}

func (a *goBlog) renderBlogSettings(hb *htmlbuilder.HTMLBuilder, rd *renderData, srd *settingsRenderData) {
	hb.WriteTextElement("h2", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "blogsettings"))

	// Deprecation warning if title/description still in config
	if a.hasDeprecatedBlogTitleDescriptionConfig() {
		hb.WriteElementsOpen("p", "b")
		hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "deprecatedblogconfigwarning"))
		hb.WriteElementsClose("b", "p")
	}

	hb.WriteElementOpen("form", "class", "fw p", "method", "post")
	hb.WriteElementOpen("input", "type", "text", "name", "blogtitle", "required", "", "value", srd.blogTitle, "placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "settingsblogtitle"))
	hb.WriteElementOpen("input", "type", "text", "name", "blogdescription", "value", srd.blogDescription, "placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "settingsblogdescription"))
	hb.WriteTextElement("textarea", srd.blogLongDescription, "name", "bloglongdescription", "placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "settingsbloglongdescription"))
	hb.WriteElementOpen(
		"input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "update"),
		"formaction", rd.Blog.getRelativePath(settingsPath+settingsUpdateBlogPath),
	)
	hb.WriteElementClose("form")
}

func (a *goBlog) renderUserSettings(hb *htmlbuilder.HTMLBuilder, rd *renderData, srd *settingsRenderData) {
	hb.WriteTextElement("h2", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "user"))

	hb.WriteElementOpen("form", "class", "fw p", "method", "post")
	hb.WriteElementOpen("input", "type", "text", "name", "usernick", "required", "", "value", srd.userNick, "placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "settingsusernick"))
	hb.WriteElementOpen("input", "type", "text", "name", "username", "required", "", "value", srd.userName, "placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "settingsusername"))
	hb.WriteElementOpen("input", "type", "email", "name", "useremail", "value", srd.userEmail, "placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "settingsuseremail"))
	hb.WriteElementOpen(
		"input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "update"),
		"formaction", rd.Blog.getRelativePath(settingsPath+settingsUpdateUserPath),
	)
	hb.WriteElementClose("form")

	hb.WriteTextElement("h3", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "profileimage"))

	hb.WriteElementOpen("form", "class", "fw p", "method", "post", "enctype", "multipart/form-data")
	hb.WriteElementOpen("input", "type", "file", "name", "file")
	hb.WriteElementOpen(
		"input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "upload"),
		"formaction", rd.Blog.getRelativePath(settingsPath+settingsUpdateProfileImagePath),
	)
	hb.WriteElementClose("form")

	hb.WriteElementOpen("form", "class", "fw p", "method", "post")
	hb.WriteElementOpen(
		"input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "delete"),
		"formaction", rd.Blog.getRelativePath(settingsPath+settingsDeleteProfileImagePath),
	)
	hb.WriteElementClose("form")

	a.renderBooleanSetting(hb, rd, rd.Blog.getRelativePath(settingsPath+settingsShowProfileImageInHeaderPath), a.ts.GetTemplateStringVariant(rd.Blog.Lang, "showprofileimageinheaderdesc"), showProfileImageInHeaderSetting, srd.showProfileImageInHeader, true)
}

func (a *goBlog) renderSecuritySettings(hb *htmlbuilder.HTMLBuilder, rd *renderData, srd *settingsRenderData) {
	hb.WriteTextElement("h2", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "security"))

	hb.WriteElementOpen("p")
	hb.WriteTextElement("a", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "connectedapps"), "href", rd.Blog.getRelativePath(settingsPath+settingsConnectedAppsPath))
	hb.WriteElementClose("p")

	// Warning if deprecated config is still present
	if a.hasDeprecatedConfig() {
		hb.WriteElementsOpen("p", "b")
		hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "deprecatedconfigwarning"))
		hb.WriteElementsClose("b", "p")
	}

	// Password section
	hb.WriteTextElement("h3", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "password"))

	// Show current password status
	if srd.hasDBPassword {
		hb.WriteTextElement("p", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "passwordset"))
	} else {
		hb.WriteTextElement("p", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "nopasswordset"))
	}

	hb.WriteElementOpen("form", "class", "fw p", "method", "post", "action", rd.Blog.getRelativePath(settingsPath+settingsUpdatePasswordPath))
	hb.WriteElementOpen("input", "type", "password", "name", "newpassword", "placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "newpassword"), "required", "", "autocomplete", "new-password")
	hb.WriteElementOpen("input", "type", "password", "name", "confirmpassword", "placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "confirmpassword"), "required", "", "autocomplete", "new-password")
	hb.WriteElementOpen("input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "updatepassword"))
	hb.WriteElementClose("form")

	// Show delete password option if passkeys are registered and password is set
	if len(srd.passkeys) > 0 && srd.hasDBPassword {
		hb.WriteElementOpen("form", "class", "fw p", "method", "post", "action", rd.Blog.getRelativePath(settingsPath+settingsDeletePasswordPath))
		hb.WriteElementOpen("input", "type", "submit", "class", "confirm", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "deletepassword"), "data-confirmmessage", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "confirmdelete"))
		hb.WriteElementClose("form")
	}

	// TOTP section
	hb.WriteTextElement("h3", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "totp"))

	if srd.hasTOTP {
		hb.WriteTextElement("p", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "totpenabled"))
		hb.WriteElementOpen("form", "class", "fw p", "method", "post", "action", rd.Blog.getRelativePath(settingsPath+settingsDeleteTOTPPath))
		hb.WriteElementOpen("input", "type", "submit", "class", "confirm", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "deletetotp"), "data-confirmmessage", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "confirmdeletetotp"))
		hb.WriteElementClose("form")
	} else {
		// Instructions
		hb.WriteTextElement("p", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "totpsetupinstructions"))
		// Secret for manual entry
		hb.WriteTextElement("p", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "totpsecretmanual"))
		hb.WriteElementOpen("p", "class", "monospace")
		hb.WriteTextElement("code", srd.newTotpSecret)
		hb.WriteElementClose("p")
		// Verification form
		hb.WriteElementOpen("form", "class", "fw p", "method", "post", "action", rd.Blog.getRelativePath(settingsPath+settingsSetupTOTPPath))
		hb.WriteElementOpen("input", "type", "hidden", "name", "totpsecret", "value", srd.newTotpSecret)
		hb.WriteElementOpen("input", "type", "text", "inputmode", "numeric", "pattern", "[0-9]*", "name", "totpcode", "placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "totpcode"), "required", "")
		hb.WriteElementOpen("input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "setuptotp"))
		hb.WriteElementClose("form")
	}

	// Passkeys section
	hb.WriteTextElement("h3", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "passkeys"))

	// Register new passkey button (full width)
	hb.WriteElementOpen("form", "class", "fw p")
	hb.WriteElementOpen(
		"input", "id", "registerwebauthn", "type", "button", "class", "hide",
		"value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "registerpasskey"),
	)
	hb.WriteElementClose("form")

	// Links to register passkeys for alt addresses
	for _, altAddress := range srd.altAddresses {
		altSettingsURL := getFullAddressStatic(altAddress, settingsPath)
		hb.WriteElementOpen("p")
		hb.WriteTextElement("a", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "registerpasskeyalt")+" "+altAddress, "href", altSettingsURL)
		hb.WriteElementClose("p")
	}

	// List existing passkeys in a table
	if len(srd.passkeys) > 0 {
		hb.WriteElementOpen("table", "class", "settings-table settings-passkeys")
		for _, pk := range srd.passkeys {
			hb.WriteElementOpen("tr")
			// Name column with rename and delete form
			hb.WriteElementOpen("td", "class", "expand")
			hb.WriteElementOpen("form", "method", "post")
			hb.WriteElementOpen("input", "type", "hidden", "name", "passkeyid", "value", pk.ID)
			hb.WriteElementOpen("input", "name", "passkeyname", "value", pk.Name, "required", "")
			hb.WriteTextElement("button", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "rename"), "type", "submit", "formaction", rd.Blog.getRelativePath(settingsPath+settingsRenamePasskeyPath))
			hb.WriteTextElement("button", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "delete"), "type", "submit", "formaction", rd.Blog.getRelativePath(settingsPath+settingsDeletePasskeyPath), "class", "confirm", "data-confirmmessage", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "confirmdelete"))
			hb.WriteElementsClose("form", "td")
			// Date column
			hb.WriteTextElement("td", pk.Created.Format(time.DateTime), "class", "fixed")
			hb.WriteElementClose("tr")
		}
		hb.WriteElementsClose("table")
	}

	// OIDC section
	if srd.oidcEnabled {
		hb.WriteTextElement("h3", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "oidc"))

		if srd.oidcLinked {
			hb.WriteTextElement("p", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "oidclinked"))
			hb.WriteTextElement("p", srd.oidcSubject, "class", "monospace")
			if srd.oidcIssuer != "" {
				hb.WriteTextElement("p", srd.oidcIssuer, "class", "monospace")
			}
			hb.WriteElementOpen("form", "class", "fw p", "method", "post", "action", rd.Blog.getRelativePath(settingsPath+settingsDeleteOIDCPath))
			hb.WriteElementOpen("input", "type", "submit", "class", "confirm", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "unlinkoidc"), "data-confirmmessage", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "confirmdelete"))
			hb.WriteElementClose("form")
		} else {
			hb.WriteTextElement("p", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "oidcdesc"))
			hb.WriteElementOpen("form", "class", "fw p", "method", "post", "action", oidcLoginPath)
			hb.WriteElementOpen("input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "linkoidc"))
			hb.WriteElementClose("form")
		}
	}

	// App passwords section
	hb.WriteTextElement("h3", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "apppasswords"))

	hb.WriteTextElement("p", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "apppasswordsdesc"))

	// Create new app password
	hb.WriteElementOpen("form", "class", "fw p", "method", "post", "action", rd.Blog.getRelativePath(settingsPath+settingsCreateAppPasswordPath))
	hb.WriteElementOpen("input", "type", "text", "name", "apppasswordname", "placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "apppasswordname"), "required", "")
	hb.WriteElementOpen("input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "createapppassword"))
	hb.WriteElementClose("form")

	// List existing app passwords in a table
	if len(srd.appPasswords) > 0 {
		hb.WriteElementOpen("table", "class", "settings-table settings-apppasswords")
		for _, ap := range srd.appPasswords {
			hb.WriteElementOpen("tr")
			// Name column with delete form
			hb.WriteElementOpen("td", "class", "expand")
			hb.WriteElementOpen("form", "method", "post")
			hb.WriteElementOpen("input", "type", "hidden", "name", "apppasswordid", "value", ap.ID)
			hb.WriteElementOpen("input", "name", "apppasswordname", "value", ap.Name, "disabled", "")
			hb.WriteTextElement("button", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "delete"), "type", "submit", "formaction", rd.Blog.getRelativePath(settingsPath+settingsDeleteAppPasswordPath), "class", "confirm", "data-confirmmessage", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "confirmdelete"))
			hb.WriteElementsClose("form", "td")
			// Date column
			hb.WriteTextElement("td", ap.Created.Format(time.DateTime), "class", "fixed")
			hb.WriteElementClose("tr")
		}
		hb.WriteElementsClose("table")
	}
}

func (a *goBlog) renderAppPasswordCreated(hb *htmlbuilder.HTMLBuilder, rd *renderData) {
	data, ok := rd.Data.(*appPasswordCreatedRenderData)
	if !ok {
		return
	}
	a.renderBase(
		hb, rd,
		func(hb *htmlbuilder.HTMLBuilder) {
			a.renderTitleTag(hb, rd.Blog, a.ts.GetTemplateStringVariant(rd.Blog.Lang, "apppasswordcreated"))
		},
		func(hb *htmlbuilder.HTMLBuilder) {
			hb.WriteElementOpen("main")
			hb.WriteTextElement("h1", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "apppasswordcreated"))
			hb.WriteElementOpen("p")
			hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "apppasswordcreatedfor"))
			hb.WriteEscaped(" ")
			hb.WriteTextElement("strong", data.name)
			hb.WriteElementClose("p")
			hb.WriteTextElement("p", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "apppasswordtoken"))
			hb.WriteElementOpen("p", "class", "monospace")
			hb.WriteTextElement("code", data.token)
			hb.WriteElementClose("p")
			hb.WriteElementOpen("p")
			hb.WriteTextElement("strong", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "apppasswordwarning"))
			hb.WriteElementClose("p")
			hb.WriteElementOpen("p")
			hb.WriteTextElement("a", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "backtosettings"), "href", rd.Blog.getRelativePath(settingsPath))
			hb.WriteElementClose("p")
			hb.WriteElementClose("main")
		},
	)
}

func renderMenuLink(hb *htmlbuilder.HTMLBuilder, href, title string) {
	hb.WriteTextElement("a", title, "href", href)
}

func (a *goBlog) renderMenuLinks(hb *htmlbuilder.HTMLBuilder, menu *configMenu) {
	for _, item := range menu.Items {
		renderMenuLink(hb, item.Link, a.renderMdTitle(item.Title))
	}
}

func (a *goBlog) renderFooter(origHb *htmlbuilder.HTMLBuilder, rd *renderData) {
	// Wrap plugins
	hb, finish := a.wrapForPlugins(origHb, pluginUIFooterType, a.getPlugins(pluginUIFooterType), func(plugin any, doc *goquery.Document) {
		plugin.(plugintypes.UIFooter).RenderFooter(rd.prc, doc)
	}, selectorBodyInner)
	defer finish()
	// Render footer
	hb.WriteElementOpen("footer")
	// Footer menu
	if fm := rd.Blog.Menus["footer"]; fm != nil && len(fm.Items) > 0 {
		hb.WriteElementOpen("nav", "class", "menu-bar", "aria-label", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "footermenu"))
		a.renderMenuLinks(hb, fm)
		hb.WriteElementClose("nav")
	}
	// Copyright
	hb.WriteElementOpen("p", "translate", "no")
	hb.WriteUnescaped("&copy; ")
	hb.WriteEscaped(time.Now().Format("2006"))
	hb.WriteUnescaped(" ")
	if user := a.cfg.User; user != nil && user.Name != "" {
		hb.WriteEscaped(user.Name)
	} else {
		hb.WriteEscaped(a.renderMdTitle(rd.Blog.Title))
	}
	hb.WriteElementClose("p")
	// Tor
	a.renderTorNotice(hb, rd)
	hb.WriteElementClose("footer")
}
