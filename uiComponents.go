package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/samber/lo"
	"go.goblog.app/app/pkgs/bufferpool"
)

type summaryTyp string

const (
	defaultSummary summaryTyp = "summary"
	photoSummary   summaryTyp = "photosummary"
)

// post summary on index pages
func (a *goBlog) renderSummary(hb *htmlBuilder, bc *configBlog, p *post, typ summaryTyp) {
	if bc == nil || p == nil {
		return
	}
	if typ == "" {
		typ = defaultSummary
	}
	// Start article
	hb.writeElementOpen("article", "class", "h-entry border-bottom")
	if p.Priority > 0 {
		// Is pinned post
		hb.writeElementOpen("p")
		hb.writeEscaped("📌 ")
		hb.writeEscaped(a.ts.GetTemplateStringVariant(bc.Lang, "pinned"))
		hb.writeElementClose("p")
	}
	if p.RenderedTitle != "" {
		// Has title
		hb.writeElementOpen("h2", "class", "p-name")
		hb.writeElementOpen("a", "class", "u-url", "href", p.Path)
		hb.writeEscaped(p.RenderedTitle)
		hb.writeElementClose("a")
		hb.writeElementClose("h2")
	}
	// Show photos in photo summary
	photos := a.photoLinks(p)
	if typ == photoSummary && len(photos) > 0 {
		for _, photo := range photos {
			_ = a.renderMarkdownToWriter(hb, fmt.Sprintf("![](%s)", photo), false)
		}
	}
	// Post meta
	a.renderPostMeta(hb, p, bc, "summary")
	if typ != photoSummary && a.showFull(p) {
		// Show full content
		hb.writeElementOpen("div", "class", "e-content")
		a.postHtmlToWriter(hb, p, false)
		hb.writeElementClose("div")
	} else {
		// Show summary
		hb.writeElementOpen("p", "class", "p-summary")
		hb.writeEscaped(a.postSummary(p))
		hb.writeElementClose("p")
	}
	// Show link to full post
	hb.writeElementOpen("p")
	prefix := bufferpool.Get()
	if len(photos) > 0 {
		// Contains photos
		prefix.WriteString("🖼️")
	}
	if p.hasTrack() {
		// Has GPX track
		prefix.WriteString("🗺️")
	}
	if prefix.Len() > 0 {
		prefix.WriteRune(' ')
		hb.writeEscaped(prefix.String())
	}
	bufferpool.Put(prefix)
	hb.writeElementOpen("a", "class", "u-url", "href", p.Path)
	hb.writeEscaped(a.ts.GetTemplateStringVariant(bc.Lang, "view"))
	hb.writeElementClose("a")
	hb.writeElementClose("p")
	// Finish article
	hb.writeElementClose("article")
}

// list of post taxonomy values (tags, series, etc.)
func (a *goBlog) renderPostTax(hb *htmlBuilder, p *post, b *configBlog) {
	if b == nil || p == nil {
		return
	}
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
}

// post meta information.
// typ can be "summary", "post" or "preview".
func (a *goBlog) renderPostMeta(hb *htmlBuilder, p *post, b *configBlog, typ string) {
	if b == nil || p == nil || typ != "summary" && typ != "post" && typ != "preview" {
		return
	}
	if typ == "summary" || typ == "post" {
		hb.writeElementOpen("div", "class", "p")
	}
	// Published time
	if published := toLocalTime(p.Published); !published.IsZero() {
		hb.writeElementOpen("div")
		hb.writeEscaped(a.ts.GetTemplateStringVariant(b.Lang, "publishedon"))
		hb.write(" ")
		hb.writeElementOpen("time", "class", "dt-published", "datetime", published.Format(time.RFC3339))
		hb.writeEscaped(published.Format(isoDateFormat))
		hb.writeElementClose("time")
		// Section
		if p.Section != "" {
			if section := b.Sections[p.Section]; section != nil {
				hb.write(" in ") // TODO: Replace with a proper translation
				hb.writeElementOpen("a", "href", b.getRelativePath(section.Name))
				hb.writeEscaped(a.renderMdTitle(section.Title))
				hb.writeElementClose("a")
			}
		}
		hb.writeElementClose("div")
	}
	// Updated time
	if updated := toLocalTime(p.Updated); !updated.IsZero() {
		hb.writeElementOpen("div")
		hb.writeEscaped(a.ts.GetTemplateStringVariant(b.Lang, "updatedon"))
		hb.write(" ")
		hb.writeElementOpen("time", "class", "dt-updated", "datetime", updated.Format(time.RFC3339))
		hb.writeEscaped(updated.Format(isoDateFormat))
		hb.writeElementClose("time")
		hb.writeElementClose("div")
	}
	// IndieWeb Meta
	a.renderPostReplyContext(hb, p, "")
	a.renderPostLikeContext(hb, p, "")
	// Like ("u-like-of")
	if likeLink := a.likeLink(p); likeLink != "" {
		hb.writeElementOpen("div")
		hb.writeEscaped(a.ts.GetTemplateStringVariant(b.Lang, "likeof"))
		hb.writeEscaped(": ")
		hb.writeElementOpen("a", "class", "u-like-of", "rel", "noopener", "target", "_blank", "href", likeLink)
		if likeTitle := a.likeTitle(p); likeTitle != "" {
			hb.writeEscaped(likeTitle)
		} else {
			hb.writeEscaped(likeLink)
		}
		hb.writeElementClose("a")
		hb.writeElementClose("div")
	}
	// Geo
	if geoURIs := a.geoURIs(p); len(geoURIs) != 0 {
		hb.writeElementOpen("div")
		hb.writeEscaped("📍 ")
		for i, geoURI := range geoURIs {
			if i > 0 {
				hb.writeEscaped(", ")
			}
			hb.writeElementOpen("a", "class", "p-location h-geo", "target", "_blank", "rel", "nofollow noopener noreferrer", "href", geoOSMLink(geoURI))
			hb.writeElementOpen("span", "class", "p-name")
			hb.writeEscaped(a.geoTitle(geoURI, b.Lang))
			hb.writeElementClose("span")
			hb.writeElementOpen("data", "class", "p-longitude", "value", fmt.Sprintf("%f", geoURI.Longitude))
			hb.writeElementClose("data")
			hb.writeElementOpen("data", "class", "p-latitude", "value", fmt.Sprintf("%f", geoURI.Latitude))
			hb.writeElementClose("data")
			hb.writeElementClose("a")
		}
		hb.writeElementClose("div")
	}
	// Post specific elements
	if typ == "post" {
		// Translations
		if translations := a.postTranslations(p); len(translations) > 0 {
			hb.writeElementOpen("div")
			hb.writeEscaped(a.ts.GetTemplateStringVariant(b.Lang, "translations"))
			hb.writeEscaped(": ")
			for i, translation := range translations {
				if i > 0 {
					hb.writeEscaped(", ")
				}
				hb.writeElementOpen("a", "translate", "no", "href", translation.Path)
				hb.writeEscaped(translation.RenderedTitle)
				hb.writeElementClose("a")
			}
			hb.writeElementClose("div")
		}
		// Short link
		if shortLink := a.shortPostURL(p); shortLink != "" {
			hb.writeElementOpen("div")
			hb.writeEscaped(a.ts.GetTemplateStringVariant(b.Lang, "shorturl"))
			hb.writeEscaped(" ")
			hb.writeElementOpen("a", "rel", "shortlink", "href", shortLink)
			hb.writeEscaped(shortLink)
			hb.writeElementClose("a")
			hb.writeElementClose("div")
		}
		// Status
		if p.Status != statusPublished {
			hb.writeElementOpen("div")
			hb.writeEscaped(a.ts.GetTemplateStringVariant(b.Lang, "status"))
			hb.writeEscaped(": ")
			hb.writeEscaped(string(p.Status))
			hb.writeElementClose("div")
		}
	}
	if typ == "summary" || typ == "post" {
		hb.writeElementClose("div")
	}
}

// Reply ("u-in-reply-to")
func (a *goBlog) renderPostReplyContext(hb *htmlBuilder, p *post, htmlWrapperElement string) {
	if htmlWrapperElement == "" {
		htmlWrapperElement = "div"
	}
	if replyLink := a.replyLink(p); replyLink != "" {
		hb.writeElementOpen(htmlWrapperElement)
		hb.writeEscaped(a.ts.GetTemplateStringVariant(a.getBlogFromPost(p).Lang, "replyto"))
		hb.writeEscaped(": ")
		hb.writeElementOpen("a", "class", "u-in-reply-to", "rel", "noopener", "target", "_blank", "href", replyLink)
		if replyTitle := a.replyTitle(p); replyTitle != "" {
			hb.writeEscaped(replyTitle)
		} else {
			hb.writeEscaped(replyLink)
		}
		hb.writeElementClose("a")
		hb.writeElementClose(htmlWrapperElement)
	}
}

// Like ("u-like-of")
func (a *goBlog) renderPostLikeContext(hb *htmlBuilder, p *post, htmlWrapperElement string) {
	if htmlWrapperElement == "" {
		htmlWrapperElement = "div"
	}
	if likeLink := a.likeLink(p); likeLink != "" {
		hb.writeElementOpen(htmlWrapperElement)
		hb.writeEscaped(a.ts.GetTemplateStringVariant(a.getBlogFromPost(p).Lang, "likeof"))
		hb.writeEscaped(": ")
		hb.writeElementOpen("a", "class", "u-like-of", "rel", "noopener", "target", "_blank", "href", likeLink)
		if likeTitle := a.likeTitle(p); likeTitle != "" {
			hb.writeEscaped(likeTitle)
		} else {
			hb.writeEscaped(likeLink)
		}
		hb.writeElementClose("a")
		hb.writeElementClose(htmlWrapperElement)
	}
}

// warning for old posts
func (a *goBlog) renderOldContentWarning(hb *htmlBuilder, p *post, b *configBlog) {
	if b == nil || b.hideOldContentWarning || p == nil || !p.Old() {
		return
	}
	hb.writeElementOpen("strong", "class", "p border-top border-bottom")
	hb.writeEscaped(a.ts.GetTemplateStringVariant(b.Lang, "oldcontent"))
	hb.writeElementClose("strong")
}

func (a *goBlog) renderInteractions(hb *htmlBuilder, rd *renderData) {
	// Start accordion
	hb.writeElementOpen("details", "class", "p", "id", "interactions")
	hb.writeElementOpen("summary")
	hb.writeElementOpen("strong")
	hb.writeEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "interactions"))
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
	renderMentions(a.db.getWebmentionsByAddress(rd.Canonical))
	// Show form to send a webmention
	hb.writeElementOpen("form", "class", "fw p", "method", "post", "action", "/webmention")
	hb.writeElementOpen("label", "for", "wm-source", "class", "p")
	hb.writeEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "interactionslabel"))
	hb.writeElementClose("label")
	hb.writeElementOpen("input", "id", "wm-source", "type", "url", "name", "source", "placeholder", "URL", "required", "")
	hb.writeElementOpen("input", "type", "hidden", "name", "target", "value", rd.Canonical)
	hb.writeElementOpen("input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "send"))
	hb.writeElementClose("form")
	// Show form to create a new comment
	hb.writeElementOpen("form", "class", "fw p", "method", "post", "action", "/comment")
	hb.writeElementOpen("input", "type", "hidden", "name", "target", "value", rd.Canonical)
	hb.writeElementOpen("input", "type", "text", "name", "name", "placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "nameopt"))
	hb.writeElementOpen("input", "type", "url", "name", "website", "placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "websiteopt"))
	hb.writeElementOpen("textarea", "name", "comment", "required", "", "placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "comment"))
	hb.writeElementClose("textarea")
	hb.writeElementOpen("input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "docomment"))
	hb.writeElementClose("form")
	// Finish accordion
	hb.writeElementClose("details")
}

// author h-card
func (a *goBlog) renderAuthor(hb *htmlBuilder) {
	user := a.cfg.User
	if user == nil {
		return
	}
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
}

// head meta tags for a post
func (a *goBlog) renderPostHeadMeta(hb *htmlBuilder, p *post, canonical string) {
	if p == nil {
		return
	}
	if canonical != "" {
		hb.writeElementOpen("meta", "property", "og:url", "content", canonical)
		hb.writeElementOpen("meta", "property", "twitter:url", "content", canonical)
	}
	if p.RenderedTitle != "" {
		hb.writeElementOpen("meta", "property", "og:title", "content", p.RenderedTitle)
		hb.writeElementOpen("meta", "property", "twitter:title", "content", p.RenderedTitle)
	}
	if summary := a.postSummary(p); summary != "" {
		hb.writeElementOpen("meta", "name", "description", "content", summary)
		hb.writeElementOpen("meta", "property", "og:description", "content", summary)
		hb.writeElementOpen("meta", "property", "twitter:description", "content", summary)
	}
	if published := toLocalTime(p.Published); !published.IsZero() {
		hb.writeElementOpen("meta", "itemprop", "datePublished", "content", published.Format(time.RFC3339))
	}
	if updated := toLocalTime(p.Updated); !updated.IsZero() {
		hb.writeElementOpen("meta", "itemprop", "dateModified", "content", updated.Format(time.RFC3339))
	}
	for _, img := range a.photoLinks(p) {
		hb.writeElementOpen("meta", "itemprop", "image", "content", img)
		hb.writeElementOpen("meta", "property", "og:image", "content", img)
		hb.writeElementOpen("meta", "property", "twitter:image", "content", img)
	}
}

// TOR notice in the footer
func (a *goBlog) renderTorNotice(hb *htmlBuilder, rd *renderData) {
	if !a.cfg.Server.Tor || (!rd.TorUsed && rd.TorAddress == "") {
		return
	}
	if rd.TorUsed {
		hb.writeElementOpen("p", "id", "tor")
		hb.writeEscaped("🔐 ")
		hb.writeEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "connectedviator"))
		hb.writeElementClose("p")
	} else if rd.TorAddress != "" {
		hb.writeElementOpen("p", "id", "tor")
		hb.writeEscaped("🔓 ")
		hb.writeElementOpen("a", "href", rd.TorAddress)
		hb.writeEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "connectviator"))
		hb.writeElementClose("a")
		hb.writeEscaped(" ")
		hb.writeElementOpen("a", "href", "https://www.torproject.org/", "target", "_blank", "rel", "nofollow noopener noreferrer")
		hb.writeEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "whatistor"))
		hb.writeElementClose("a")
		hb.writeElementClose("p")
	}
}

func (a *goBlog) renderTitleTag(hb *htmlBuilder, blog *configBlog, optionalTitle string) {
	hb.writeElementOpen("title")
	if optionalTitle != "" {
		hb.writeEscaped(optionalTitle)
		hb.writeEscaped(" - ")
	}
	hb.writeEscaped(a.renderMdTitle(blog.Title))
	hb.writeElementClose("title")
}

func (a *goBlog) renderPagination(hb *htmlBuilder, blog *configBlog, hasPrev, hasNext bool, prev, next string) {
	// Navigation
	if hasPrev {
		hb.writeElementOpen("p")
		hb.writeElementOpen("a", "href", prev) // TODO: rel=prev?
		hb.writeEscaped(a.ts.GetTemplateStringVariant(blog.Lang, "prev"))
		hb.writeElementClose("a")
		hb.writeElementClose("p")
	}
	if hasNext {
		hb.writeElementOpen("p")
		hb.writeElementOpen("a", "href", next) // TODO: rel=next?
		hb.writeEscaped(a.ts.GetTemplateStringVariant(blog.Lang, "next"))
		hb.writeElementClose("a")
		hb.writeElementClose("p")
	}
}

func (*goBlog) renderPostTitle(hb *htmlBuilder, p *post) {
	if p == nil || p.RenderedTitle == "" {
		return
	}
	hb.writeElementOpen("h1", "class", "p-name")
	hb.writeEscaped(p.RenderedTitle)
	hb.writeElementClose("h1")
}

func (a *goBlog) renderPostGPX(hb *htmlBuilder, p *post, b *configBlog) {
	if p == nil || !p.hasTrack() {
		return
	}
	track, err := a.getTrack(p, p.showTrackRoute())
	if err != nil || track == nil {
		return
	}
	// Track stats
	hb.writeElementOpen("p")
	if track.Name != "" {
		hb.writeElementOpen("strong")
		hb.writeEscaped(track.Name)
		hb.writeElementClose("strong")
		hb.write(" ")
	}
	if track.Kilometers != "" {
		hb.write("🏁 ")
		hb.writeEscaped(track.Kilometers)
		hb.write(" ")
		hb.writeEscaped(a.ts.GetTemplateStringVariant(b.Lang, "kilometers"))
		hb.write(" ")
	}
	if track.Hours != "" {
		hb.write("⏱ ")
		hb.writeEscaped(track.Hours)
	}
	hb.writeElementClose("p")
	// Map (only show if it has features)
	if track.hasMapFeatures() {
		hb.writeElementOpen(
			"div", "id", "map", "class", "p",
			"data-paths", track.PathsJSON,
			"data-points", track.PointsJSON,
			"data-minzoom", track.MinZoom, "data-maxzoom", track.MaxZoom,
			"data-attribution", track.MapAttribution,
		)
		hb.writeElementClose("div")
		hb.writeElementOpen("script", "defer", "", "src", a.assetFileName("js/geomap.js"))
		hb.writeElementClose("script")
	}
}

func (a *goBlog) renderPostReactions(hb *htmlBuilder, p *post) {
	if !a.reactionsEnabledForPost(p) {
		return
	}
	hb.writeElementOpen("div", "id", "reactions", "class", "actions", "data-path", p.Path, "data-allowed", strings.Join(allowedReactions, ","))
	hb.writeElementClose("div")
	hb.writeElementOpen("script", "defer", "", "src", a.assetFileName("js/reactions.js"))
	hb.writeElementClose("script")
}

func (a *goBlog) renderPostVideo(hb *htmlBuilder, p *post) {
	if !p.hasVideoPlaylist() {
		return
	}
	hb.writeElementOpen("div", "id", "video", "data-url", p.firstParameter(videoPlaylistParam))
	hb.writeElementClose("div")
	hb.writeElementOpen("script", "defer", "", "src", a.assetFileName("js/video.js"))
	hb.writeElementClose("script")
}

func (a *goBlog) renderPostSectionSettings(hb *htmlBuilder, rd *renderData, srd *settingsRenderData) {
	hb.writeElementOpen("h2")
	hb.writeEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "postsections"))
	hb.writeElementClose("h2")

	// Update default section
	hb.writeElementOpen("h3")
	hb.writeEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "default"))
	hb.writeElementClose("h3")

	hb.writeElementOpen("form", "class", "fw p", "method", "post")
	hb.writeElementOpen("select", "name", "defaultsection")
	for _, section := range srd.sections {
		hb.writeElementOpen("option", "value", section.Name, lo.If(section.Name == srd.defaultSection, "selected").Else(""), "")
		hb.writeEscaped(section.Name)
		hb.writeElementClose("option")
	}
	hb.writeElementClose("select")
	hb.writeElementOpen(
		"input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "update"),
		"formaction", rd.Blog.getRelativePath(settingsPath+settingsUpdateDefaultSectionPath),
	)
	hb.writeElementClose("form")

	for _, section := range srd.sections {
		hb.writeElementOpen("details")

		hb.writeElementOpen("summary")
		hb.writeElementOpen("h3")
		hb.writeEscaped(section.Name)
		hb.writeElementClose("h3")
		hb.writeElementClose("summary")

		hb.writeElementOpen("form", "class", "fw p", "method", "post")

		hb.writeElementOpen("input", "type", "hidden", "name", "sectionname", "value", section.Name)

		// Title
		hb.writeElementOpen("input", "type", "text", "name", "sectiontitle", "placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "sectiontitle"), "required", "", "value", section.Title)
		// Description
		hb.writeElementOpen(
			"textarea",
			"name", "sectiondescription",
			"class", "monospace",
			"placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "sectiondescription"),
		)
		hb.writeEscaped(section.Description)
		hb.writeElementClose("textarea")
		// Path template
		hb.writeElementOpen("input", "type", "text", "name", "sectionpathtemplate", "placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "sectionpathtemplate"), "value", section.PathTemplate)
		// Show full
		hb.writeElementOpen("input", "type", "checkbox", "name", "sectionshowfull", "id", "showfull-"+section.Name, lo.If(section.ShowFull, "checked").Else(""), "")
		hb.writeElementOpen("label", "for", "showfull-"+section.Name)
		hb.writeEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "sectionshowfull"))
		hb.writeElementClose("label")

		// Actions
		hb.writeElementOpen("div", "class", "p")
		// Update
		hb.writeElementOpen(
			"input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "update"),
			"formaction", rd.Blog.getRelativePath(settingsPath+settingsUpdateSectionPath),
		)
		// Delete
		hb.writeElementOpen(
			"input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "delete"),
			"formaction", rd.Blog.getRelativePath(settingsPath+settingsDeleteSectionPath),
			"class", "confirm", "data-confirmmessage", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "confirmdelete"),
		)
		hb.writeElementClose("div")

		hb.writeElementClose("form")
		hb.writeElementClose("details")
	}

	// Create new section
	hb.writeElementOpen("form", "class", "fw p", "method", "post")
	// Name
	hb.writeElementOpen("input", "type", "text", "name", "sectionname", "placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "sectionname"), "required", "")
	// Title
	hb.writeElementOpen("input", "type", "text", "name", "sectiontitle", "placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "sectiontitle"), "required", "")
	// Create button
	hb.writeElementOpen("div")
	hb.writeElementOpen(
		"input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "create"),
		"formaction", rd.Blog.getRelativePath(settingsPath+settingsCreateSectionPath),
	)
	hb.writeElementClose("div")
	hb.writeElementClose("form")
}

func (a *goBlog) renderCollapsibleBooleanSetting(hb *htmlBuilder, rd *renderData, path, title, description, name string, value bool) {
	hb.writeElementOpen("details")

	hb.writeElementOpen("summary")
	hb.writeElementOpen("h3")
	hb.writeEscaped(title)
	hb.writeElementClose("h3")
	hb.writeElementClose("summary")

	hb.writeElementOpen("form", "class", "fw p", "method", "post")

	hb.writeElementOpen("input", "type", "checkbox", "name", name, "id", "cb-"+name, lo.If(value, "checked").Else(""), "")
	hb.writeElementOpen("label", "for", "cb-"+name)
	hb.writeEscaped(description)
	hb.writeElementClose("label")

	hb.writeElementOpen("div", "class", "p")
	hb.writeElementOpen(
		"input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "update"), "formaction", path,
	)
	hb.writeElementClose("div")

	hb.writeElementClose("form")

	hb.writeElementClose("details")
}
