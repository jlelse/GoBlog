package main

import (
	"cmp"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/samber/lo"
	"go.goblog.app/app/pkgs/contenttype"
	"go.goblog.app/app/pkgs/gpxhelper"
	"go.goblog.app/app/pkgs/htmlbuilder"
	"go.goblog.app/app/pkgs/plugintypes"
)

type summaryTyp string

const (
	defaultSummary summaryTyp = "summary"
	photoSummary   summaryTyp = "photosummary"
)

// post summary on index pages
func (a *goBlog) renderSummary(origHb *htmlbuilder.HtmlBuilder, rd *renderData, bc *configBlog, p *post, typ summaryTyp) {
	if bc == nil || p == nil {
		return
	}
	if typ == "" {
		typ = defaultSummary
	}
	// Plugin handling
	hb, finish := a.wrapForPlugins(origHb, a.getPlugins(pluginUiSummaryType), func(plugin any, doc *goquery.Document) {
		plugin.(plugintypes.UISummary).RenderSummaryForPost(rd.prc, p, doc)
	}, selectorBodyInner)
	defer finish()
	// Start article
	hb.WriteElementOpen("article", "class", "h-entry border-bottom")
	if p.Priority > 0 {
		// Is pinned post
		hb.WriteElementOpen("p")
		hb.WriteEscaped("📌 ")
		hb.WriteEscaped(a.ts.GetTemplateStringVariant(bc.Lang, "pinned"))
		hb.WriteElementClose("p")
	}
	if p.RenderedTitle != "" {
		// Has title
		hb.WriteElementOpen("h2", "class", "p-name")
		hb.WriteElementOpen("a", "class", "u-url", "href", p.Path)
		hb.WriteEscaped(p.RenderedTitle)
		hb.WriteElementClose("a")
		hb.WriteElementClose("h2")
	}
	// Show photos in photo summary
	photos := a.photoLinks(p)
	if typ == photoSummary && len(photos) > 0 {
		for _, photo := range photos {
			hb.WriteElementOpen("p")
			hb.WriteElementOpen("img", "src", photo, "class", "u-photo")
			hb.WriteElementClose("img")
			hb.WriteElementClose("p")
		}
	}
	// Post meta
	a.renderPostMeta(hb, p, bc, "summary")
	if typ != photoSummary && a.showFull(p) {
		// Show full content
		a.postHtmlToWriter(hb, &postHtmlOptions{p: p})
	} else {
		// Show IndieWeb context
		a.renderPostReplyContext(hb, p)
		a.renderPostLikeContext(hb, p)
		// Show summary
		hb.WriteElementOpen("p", "class", "p-summary")
		hb.WriteEscaped(a.postSummary(p))
		hb.WriteElementClose("p")
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
	hb.WriteElementOpen("a", "class", "u-url", "href", p.Path)
	hb.WriteEscaped(a.ts.GetTemplateStringVariant(bc.Lang, "view"))
	hb.WriteElementClose("a")
	hb.WriteElementClose("p")
	// Finish article
	hb.WriteElementClose("article")
}

// list of post taxonomy values (tags, series, etc.)
func (a *goBlog) renderPostTax(hb *htmlbuilder.HtmlBuilder, p *post, b *configBlog) {
	if b == nil || p == nil {
		return
	}
	// Iterate over all taxonomies
	for _, tax := range b.Taxonomies {
		// Get all sorted taxonomy values for this post
		if taxValues := sortedStrings(p.Parameters[tax.Name]); len(taxValues) > 0 {
			// Start new paragraph
			hb.WriteElementOpen("p")
			// Add taxonomy name
			hb.WriteElementOpen("strong")
			hb.WriteEscaped(a.renderMdTitle(tax.Title))
			hb.WriteElementClose("strong")
			hb.WriteUnescaped(": ")
			// Add taxonomy values
			for i, taxValue := range taxValues {
				if i > 0 {
					hb.WriteUnescaped(", ")
				}
				hb.WriteElementOpen(
					"a",
					"class", "p-category",
					"rel", "tag",
					"href", b.getRelativePath(fmt.Sprintf("/%s/%s", tax.Name, urlize(taxValue))),
				)
				hb.WriteEscaped(a.renderMdTitle(taxValue))
				hb.WriteElementClose("a")
			}
			// End paragraph
			hb.WriteElementClose("p")
		}
	}
}

// post meta information.
// typ can be "summary", "post" or "preview".
func (a *goBlog) renderPostMeta(hb *htmlbuilder.HtmlBuilder, p *post, b *configBlog, typ string) {
	if b == nil || p == nil || typ != "summary" && typ != "post" && typ != "preview" {
		return
	}
	if typ == "summary" || typ == "post" {
		hb.WriteElementOpen("div", "class", "p")
	}
	// Published time
	if published := toLocalTime(p.Published); !published.IsZero() {
		hb.WriteElementOpen("div")
		hb.WriteEscaped(a.ts.GetTemplateStringVariant(b.Lang, "publishedon"))
		hb.WriteUnescaped(" ")
		hb.WriteElementOpen("time", "class", "dt-published", "datetime", published.Format(time.RFC3339))
		hb.WriteEscaped(published.Format(isoDateFormat))
		hb.WriteElementClose("time")
		// Section
		if p.Section != "" {
			if section := b.Sections[p.Section]; section != nil {
				hb.WriteUnescaped(" in ") // TODO: Replace with a proper translation
				hb.WriteElementOpen("a", "href", b.getRelativePath(section.Name))
				hb.WriteEscaped(a.renderMdTitle(section.Title))
				hb.WriteElementClose("a")
			}
		}
		hb.WriteElementClose("div")
	}
	// Updated time
	if updated := toLocalTime(p.Updated); !updated.IsZero() {
		hb.WriteElementOpen("div")
		hb.WriteEscaped(a.ts.GetTemplateStringVariant(b.Lang, "updatedon"))
		hb.WriteUnescaped(" ")
		hb.WriteElementOpen("time", "class", "dt-updated", "datetime", updated.Format(time.RFC3339))
		hb.WriteEscaped(updated.Format(isoDateFormat))
		hb.WriteElementClose("time")
		hb.WriteElementClose("div")
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
			hb.WriteElementOpen("span", "class", "p-name")
			hb.WriteEscaped(a.geoTitle(geoURI, b.Lang))
			hb.WriteElementClose("span")
			hb.WriteElementOpen("data", "class", "p-longitude", "value", fmt.Sprintf("%f", geoURI.Longitude))
			hb.WriteElementClose("data")
			hb.WriteElementOpen("data", "class", "p-latitude", "value", fmt.Sprintf("%f", geoURI.Latitude))
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
				hb.WriteElementOpen("a", "translate", "no", "href", translation.Path)
				hb.WriteEscaped(translation.RenderedTitle)
				hb.WriteElementClose("a")
			}
			hb.WriteElementClose("div")
		}
		// Short link
		if shortLink := a.shortPostURL(p); shortLink != "" {
			hb.WriteElementOpen("div")
			hb.WriteEscaped(a.ts.GetTemplateStringVariant(b.Lang, "shorturl"))
			hb.WriteEscaped(" ")
			hb.WriteElementOpen("a", "rel", "shortlink", "href", shortLink)
			hb.WriteEscaped(shortLink)
			hb.WriteElementClose("a")
			hb.WriteElementClose("div")
		}
		// Status
		if p.Status != statusPublished {
			hb.WriteElementOpen("div")
			hb.WriteEscaped(a.ts.GetTemplateStringVariant(b.Lang, "status"))
			hb.WriteEscaped(": ")
			hb.WriteEscaped(string(p.Status))
			hb.WriteElementClose("div")
		}
		// Visibility
		if p.Visibility != visibilityPublic {
			hb.WriteElementOpen("div")
			hb.WriteEscaped(a.ts.GetTemplateStringVariant(b.Lang, "visibility"))
			hb.WriteEscaped(": ")
			hb.WriteEscaped(string(p.Visibility))
			hb.WriteElementClose("div")
		}
	}
	if typ == "summary" || typ == "post" {
		hb.WriteElementClose("div")
	}
}

// Reply ("u-in-reply-to")
func (a *goBlog) renderPostReplyContext(hb *htmlbuilder.HtmlBuilder, p *post) {
	a.renderPostLikeReplyContext(hb, "u-in-reply-to", a.ts.GetTemplateStringVariant(a.getBlogFromPost(p).Lang, "replyto"), a.replyLink(p), a.replyTitle(p), a.replyContext(p))
}

// Like ("u-like-of")
func (a *goBlog) renderPostLikeContext(hb *htmlbuilder.HtmlBuilder, p *post) {
	a.renderPostLikeReplyContext(hb, "u-like-of", a.ts.GetTemplateStringVariant(a.getBlogFromPost(p).Lang, "likeof"), a.likeLink(p), a.likeTitle(p), a.likeContext(p))
}

func (a *goBlog) renderPostLikeReplyContext(hb *htmlbuilder.HtmlBuilder, class, pretext, link, title, content string) {
	if link == "" {
		return
	}

	hb.WriteElementOpen("div", "class", "h-cite "+class)

	hb.WriteElementOpen("p")
	hb.WriteElementOpen("strong")
	hb.WriteEscaped(pretext)
	hb.WriteEscaped(": ")
	hb.WriteElementOpen("a", "class", "u-url", "rel", "noopener", "target", "_blank", "href", link)
	hb.WriteEscaped(lo.If(title != "", title).Else(link))
	hb.WriteElementClose("a")
	hb.WriteElementClose("strong")
	hb.WriteElementClose("p")

	if content != "" {
		hb.WriteElementOpen("blockquote")
		hb.WriteElementOpen("p", "class", "e-content")
		hb.WriteEscaped(content)
		hb.WriteElementClose("p")
		hb.WriteElementClose("blockquote")
	}

	hb.WriteElementClose("div")
}

// warning for old posts
func (a *goBlog) renderOldContentWarning(hb *htmlbuilder.HtmlBuilder, p *post, b *configBlog) {
	if b == nil || b.hideOldContentWarning || p == nil || !p.Old() {
		return
	}
	hb.WriteElementOpen("strong", "class", "p border-top border-bottom", "id", "oldcontentwarning")
	hb.WriteEscaped(a.ts.GetTemplateStringVariant(b.Lang, "oldcontent"))
	hb.WriteElementClose("strong")
}

func (a *goBlog) renderShareButton(hb *htmlbuilder.HtmlBuilder, p *post, b *configBlog) {
	if b == nil || b.hideShareButton || p == nil {
		return
	}

	// Share button
	hb.WriteElementOpen(
		"button",
		"type", "button",
		"id", "shareBtn",
		"class", "button",
	)
	hb.WriteEscaped(a.ts.GetTemplateStringVariant(b.Lang, "share"))
	hb.WriteElementClose("button")

	// Share modal
	hb.WriteElementOpen("div", "id", "shareModal", "class", "hide")
	hb.WriteElement("div", "id", "shareModalOverlay", "class", "share-modal-close")
	hb.WriteElementOpen("div", "id", "shareModalPanel")
	hb.WriteElementOpen("div", "id", "shareModalHeader")
	hb.WriteElementOpen("strong")
	hb.WriteEscaped(a.ts.GetTemplateStringVariant(b.Lang, "sharemodalheading"))
	hb.WriteElementClose("strong")
	hb.WriteElementOpen("button", "type", "button", "class", "share-modal-close")
	hb.WriteEscaped("×")
	hb.WriteElementsClose("button", "div")
	hb.WriteElementOpen("button", "id", "shareModalNative", "type", "button", "class", "button fw hide")
	hb.WriteEscaped(a.ts.GetTemplateStringVariant(b.Lang, "sharenativeshare"))
	hb.WriteElementClose("button")
	hb.WriteElementOpen("button", "id", "shareModalCopy", "type", "button", "class", "button fw", "data-share-copy-feedback", a.ts.GetTemplateStringVariant(b.Lang, "sharecopyfeedback"))
	hb.WriteEscaped(a.ts.GetTemplateStringVariant(b.Lang, "sharecopy"))
	hb.WriteElementClose("button")
	hb.WriteElement("div", "id", "shareModalServices")
	hb.WriteElementsClose("div", "div")
	hb.WriteElementOpen("script", "type", contenttype.JSON, "id", "shareData")
	_ = json.NewEncoder(hb).Encode(newShareData(cmp.Or(p.RenderedTitle, a.fallbackTitle(p)), a.shortPostURL(p)))
	hb.WriteElementClose("script")
	hb.WriteElement("script", "defer", "", "src", a.assetFileName("js/share.js"))
}

func (a *goBlog) renderTranslateButton(hb *htmlbuilder.HtmlBuilder, p *post, b *configBlog) {
	if b == nil || b.hideTranslateButton {
		return
	}
	hb.WriteElementOpen(
		"a", "id", "translateBtn",
		"class", "button",
		"href", fmt.Sprintf("https://translate.google.com/translate?u=%s", a.getFullAddress(p.Path)),
		"target", "_blank", "rel", "nofollow noopener noreferrer",
		"title", a.ts.GetTemplateStringVariant(b.Lang, "translate"),
		"translate", "no",
	)
	hb.WriteEscaped("A ⇄ 文")
	hb.WriteElementClose("a")
	hb.WriteElementOpen("script", "defer", "", "src", a.assetFileName("js/translate.js"))
	hb.WriteElementClose("script")
}

func (a *goBlog) renderInteractions(hb *htmlbuilder.HtmlBuilder, rd *renderData) {
	// Start accordion
	hb.WriteElementOpen("details", "class", "p", "id", "interactions")
	hb.WriteElementOpen("summary")
	hb.WriteElementOpen("strong")
	hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "interactions"))
	hb.WriteElementClose("strong")
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
			hb.WriteElementOpen("a", "href", mention.Url, "target", "_blank", "rel", "nofollow noopener noreferrer ugc")
			hb.WriteEscaped(cmp.Or(mention.Author, mention.Url))
			hb.WriteElementClose("a")
			if mention.Title != "" {
				hb.WriteUnescaped(" ")
				hb.WriteElementOpen("strong")
				hb.WriteEscaped(mention.Title)
				hb.WriteElementClose("strong")
			}
			if mention.Content != "" {
				hb.WriteUnescaped(" ")
				hb.WriteElementOpen("i")
				hb.WriteEscaped(mention.Content)
				hb.WriteElementClose("i")
			}
			if len(mention.Replies) > 0 {
				hb.WriteElementOpen("ul")
				for _, reply := range mention.Replies {
					hb.WriteElementOpen("li")
					hb.WriteElementOpen("a", "href", reply.Path, "target", "_blank")
					hb.WriteEscaped(cmp.Or(a.cfg.User.Name, a.getFullAddress(reply.Path)))
					hb.WriteElementClose("a")
					if reply.RenderedTitle != "" {
						hb.WriteUnescaped(" ")
						hb.WriteElementOpen("strong")
						hb.WriteEscaped(reply.RenderedTitle)
						hb.WriteElementClose("strong")
					}
					if summary := a.postSummary(reply); summary != "" {
						hb.WriteUnescaped(" ")
						hb.WriteElementOpen("i")
						hb.WriteEscaped(summary)
						hb.WriteElementClose("i")
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
	hb.WriteElementOpen("label", "for", "wm-source", "class", "p")
	hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "interactionslabel"))
	hb.WriteElementClose("label")
	hb.WriteElementOpen("input", "id", "wm-source", "type", "url", "name", "source", "placeholder", "URL", "required", "")
	hb.WriteElementOpen("input", "type", "hidden", "name", "target", "value", rd.Canonical)
	hb.WriteElementOpen("input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "send"))
	hb.WriteElementClose("form")
	// Show form to create a new comment
	hb.WriteElementOpen("form", "class", "fw p", "method", "post", "action", rd.Blog.getRelativePath(commentPath))
	hb.WriteElementOpen("input", "type", "hidden", "name", "target", "value", rd.Canonical)
	hb.WriteElementOpen("input", "type", "text", "name", "name", "placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "nameopt"))
	hb.WriteElementOpen("input", "type", "url", "name", "website", "placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "websiteopt"))
	hb.WriteElementOpen("textarea", "name", "comment", "required", "", "placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "comment"))
	hb.WriteElementClose("textarea")
	hb.WriteElementOpen("input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "docomment"))
	hb.WriteElementClose("form")
	// Finish accordion
	hb.WriteElementClose("details")
}

// author h-card
func (a *goBlog) renderAuthor(hb *htmlbuilder.HtmlBuilder) {
	user := a.cfg.User
	if user == nil {
		return
	}
	hb.WriteElementOpen("div", "class", "p-author h-card hide")
	if a.hasProfileImage() {
		hb.WriteElementOpen("data", "class", "u-photo", "value", a.getFullAddress(a.profileImagePath(profileImageFormatJPEG, 0, 0)))
		hb.WriteElementClose("data")
	}
	if user.Name != "" {
		hb.WriteElementOpen("a", "class", "p-name u-url", "rel", "me", "href", cmp.Or(user.Link, "/"))
		hb.WriteEscaped(user.Name)
		hb.WriteElementClose("a")
	}
	hb.WriteElementClose("div")
}

// head meta tags for a post
func (a *goBlog) renderPostHeadMeta(hb *htmlbuilder.HtmlBuilder, p *post) {
	if p == nil {
		return
	}
	if summary := a.postSummary(p); summary != "" {
		hb.WriteElementOpen("meta", "name", "description", "content", summary)
	}
	if published := toLocalTime(p.Published); !published.IsZero() {
		hb.WriteElementOpen("meta", "itemprop", "datePublished", "content", published.Format(time.RFC3339))
	}
	if updated := toLocalTime(p.Updated); !updated.IsZero() {
		hb.WriteElementOpen("meta", "itemprop", "dateModified", "content", updated.Format(time.RFC3339))
	}
	for _, img := range a.photoLinks(p) {
		hb.WriteElementOpen("meta", "itemprop", "image", "content", img)
	}
	if a.apEnabled() {
		if userHandle, ok := a.apUserHandle[p.Blog]; ok {
			hb.WriteElementOpen("meta", "name", "fediverse:creator", "property", "fediverse:creator", "content", userHandle)
		}
	}
}

// TOR notice in the footer
func (a *goBlog) renderTorNotice(hb *htmlbuilder.HtmlBuilder, rd *renderData) {
	if !a.cfg.Server.Tor || (!rd.TorUsed && rd.TorAddress == "") {
		return
	}
	if rd.TorUsed {
		hb.WriteElementOpen("p", "id", "tor")
		hb.WriteEscaped("🔐 ")
		hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "connectedviator"))
		hb.WriteElementClose("p")
	} else if rd.TorAddress != "" {
		hb.WriteElementOpen("p", "id", "tor")
		hb.WriteEscaped("🔓 ")
		hb.WriteElementOpen("a", "href", rd.TorAddress)
		hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "connectviator"))
		hb.WriteElementClose("a")
		hb.WriteEscaped(" ")
		hb.WriteElementOpen("a", "href", "https://www.torproject.org/", "target", "_blank", "rel", "nofollow noopener noreferrer")
		hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "whatistor"))
		hb.WriteElementClose("a")
		hb.WriteElementClose("p")
	}
}

func (a *goBlog) renderTitleTag(hb *htmlbuilder.HtmlBuilder, blog *configBlog, optionalTitle string) {
	hb.WriteElementOpen("title")
	if optionalTitle != "" {
		hb.WriteEscaped(optionalTitle)
		hb.WriteEscaped(" - ")
	}
	hb.WriteEscaped(a.renderMdTitle(blog.Title))
	hb.WriteElementClose("title")
}

func (a *goBlog) renderPagination(hb *htmlbuilder.HtmlBuilder, blog *configBlog, hasPrev, hasNext bool, prev, next string) {
	// Navigation
	if hasPrev {
		hb.WriteElementOpen("p")
		hb.WriteElementOpen("a", "href", prev) // TODO: rel=prev?
		hb.WriteEscaped(a.ts.GetTemplateStringVariant(blog.Lang, "prev"))
		hb.WriteElementClose("a")
		hb.WriteElementClose("p")
	}
	if hasNext {
		hb.WriteElementOpen("p")
		hb.WriteElementOpen("a", "href", next) // TODO: rel=next?
		hb.WriteEscaped(a.ts.GetTemplateStringVariant(blog.Lang, "next"))
		hb.WriteElementClose("a")
		hb.WriteElementClose("p")
	}
}

func (*goBlog) renderPostTitle(hb *htmlbuilder.HtmlBuilder, p *post) {
	if p == nil || p.RenderedTitle == "" {
		return
	}
	hb.WriteElementOpen("h1", "class", "p-name")
	hb.WriteEscaped(p.RenderedTitle)
	hb.WriteElementClose("h1")
}

func (a *goBlog) renderPostTrack(hb *htmlbuilder.HtmlBuilder, p *post, b *configBlog, disableInteractiveMap bool) {
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

func (a *goBlog) renderPostTrackStatistics(hb *htmlbuilder.HtmlBuilder, track *trackResult, b *configBlog) {
	hb.WriteElementOpen("p")
	if track.Name != "" {
		hb.WriteElementOpen("strong")
		hb.WriteEscaped(track.Name)
		hb.WriteElementClose("strong")
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

func (a *goBlog) renderPostTrackMap(hb *htmlbuilder.HtmlBuilder, track *trackResult) {
	pathsJSON, _ := json.Marshal(track.Paths)
	pointsJSON, _ := json.Marshal(track.Points)
	hb.WriteElementOpen(
		"div", "id", "map", "class", "p",
		"data-paths", string(pathsJSON),
		"data-points", string(pointsJSON),
		"data-minzoom", a.getMinZoom(),
		"data-maxzoom", a.getMaxZoom(),
		"data-attribution", a.getMapAttribution(),
	)
	hb.WriteElementClose("div")
	hb.WriteElementOpen("script", "defer", "", "src", a.assetFileName("js/geomap.js"))
	hb.WriteElementClose("script")
}

func (a *goBlog) renderPostTrackSVG(hb *htmlbuilder.HtmlBuilder, track *trackResult) {
	const width, height = 700.0, 400.0
	// Calculate min/max values in Web Mercator projection
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, path := range track.Paths {
		for _, point := range path {
			x, y := gpxhelper.WebMercatorX(point.Lon()), gpxhelper.WebMercatorY(point.Lat())
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
	for _, path := range track.Paths {
		hb.WriteString(`<polyline points="`)
		for _, pt := range path {
			x := xOffset + (gpxhelper.WebMercatorX(pt.Lon())-minX)*scale
			y := height - (yOffset + (gpxhelper.WebMercatorY(pt.Lat())-minY)*scale)
			hb.WriteString(fmt.Sprintf("%.2f,%.2f ", x, y))
		}
		hb.WriteString(`" fill="none" stroke="currentColor" stroke-width="3" />`)
	}
	hb.WriteElementClose("svg")
}

func (a *goBlog) renderPostReactions(hb *htmlbuilder.HtmlBuilder, p *post) {
	if !a.reactionsEnabledForPost(p) {
		return
	}
	hb.WriteElementOpen("div", "id", "reactions", "class", "actions", "data-path", p.Path, "data-allowed", strings.Join(allowedReactions, ","))
	hb.WriteElementClose("div")
	hb.WriteElementOpen("script", "defer", "", "src", a.assetFileName("js/reactions.js"))
	hb.WriteElementClose("script")
}

func (a *goBlog) renderPostVideo(hb *htmlbuilder.HtmlBuilder, p *post) {
	if !p.hasVideoPlaylist() {
		return
	}
	hb.WriteElementOpen("div", "id", "video", "data-url", p.firstParameter(videoPlaylistParam))
	hb.WriteElementClose("div")
	hb.WriteElementOpen("script", "defer", "", "src", a.assetFileName("js/video.js"))
	hb.WriteElementClose("script")
}

func (a *goBlog) renderPostSectionSettings(hb *htmlbuilder.HtmlBuilder, rd *renderData, srd *settingsRenderData) {
	hb.WriteElementOpen("h2")
	hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "postsections"))
	hb.WriteElementClose("h2")

	// Update default section
	hb.WriteElementOpen("h3")
	hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "default"))
	hb.WriteElementClose("h3")

	hb.WriteElementOpen("form", "class", "fw p", "method", "post")
	hb.WriteElementOpen("select", "name", "defaultsection")
	for _, section := range srd.sections {
		hb.WriteElementOpen("option", "value", section.Name, lo.If(section.Name == srd.defaultSection, "selected").Else(""), "")
		hb.WriteEscaped(section.Name)
		hb.WriteElementClose("option")
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
		hb.WriteElementOpen("h3")
		hb.WriteEscaped(section.Name)
		hb.WriteElementClose("h3")
		hb.WriteElementClose("summary")

		hb.WriteElementOpen("form", "class", "fw p", "method", "post")

		hb.WriteElementOpen("input", "type", "hidden", "name", "sectionname", "value", section.Name)

		// Title
		hb.WriteElementOpen("input", "type", "text", "name", "sectiontitle", "placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "sectiontitle"), "required", "", "value", section.Title)
		// Description
		hb.WriteElementOpen(
			"textarea",
			"name", "sectiondescription",
			"class", "monospace",
			"placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "sectiondescription"),
		)
		hb.WriteEscaped(section.Description)
		hb.WriteElementClose("textarea")
		// Path template
		hb.WriteElementOpen("input", "type", "text", "name", "sectionpathtemplate", "placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "sectionpathtemplate"), "value", section.PathTemplate)
		// Show full
		hb.WriteElementOpen("input", "type", "checkbox", "name", "sectionshowfull", "id", "showfull-"+section.Name, lo.If(section.ShowFull, "checked").Else(""), "")
		hb.WriteElementOpen("label", "for", "showfull-"+section.Name)
		hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "sectionshowfull"))
		hb.WriteElementClose("label")
		hb.WriteElementsClose("br")
		// Hide on start
		hb.WriteElementOpen("input", "type", "checkbox", "name", "sectionhideonstart", "id", "hideonstart-"+section.Name, lo.If(section.HideOnStart, "checked").Else(""), "")
		hb.WriteElementOpen("label", "for", "hideonstart-"+section.Name)
		hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "sectionhideonstart"))
		hb.WriteElementClose("label")

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

func (a *goBlog) renderBooleanSetting(hb *htmlbuilder.HtmlBuilder, rd *renderData, path, description, name string, value bool) {
	hb.WriteElementOpen("form", "class", "fw p", "method", "post", "action", path)

	hb.WriteElementOpen("input", "type", "checkbox", "class", "autosubmit", "name", name, "id", "cb-"+name, lo.If(value, "checked").Else(""), "")
	hb.WriteElementOpen("label", "for", "cb-"+name)
	hb.WriteEscaped(description)
	hb.WriteElementClose("label")

	hb.WriteElementOpen("noscript")
	hb.WriteElementOpen("div", "class", "p")
	hb.WriteElementOpen(
		"input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "update"),
	)
	hb.WriteElementClose("div")
	hb.WriteElementClose("noscript")

	hb.WriteElementClose("form")
}

func (a *goBlog) renderUserSettings(hb *htmlbuilder.HtmlBuilder, rd *renderData, srd *settingsRenderData) {
	hb.WriteElementOpen("h2")
	hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "user"))
	hb.WriteElementClose("h2")

	hb.WriteElementOpen("form", "class", "fw p", "method", "post")
	hb.WriteElementOpen("input", "type", "text", "name", "usernick", "required", "", "value", srd.userNick, "placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "settingsusernick"))
	hb.WriteElementOpen("input", "type", "text", "name", "username", "required", "", "value", srd.userName, "placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "settingsusername"))
	hb.WriteElementOpen(
		"input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "update"),
		"formaction", rd.Blog.getRelativePath(settingsPath+settingsUpdateUserPath),
	)
	hb.WriteElementClose("form")

	hb.WriteElementOpen("h3")
	hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "profileimage"))
	hb.WriteElementClose("h3")

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
}

func (a *goBlog) renderSecuritySettings(hb *htmlbuilder.HtmlBuilder, rd *renderData, srd *settingsRenderData) {
	hb.WriteElementOpen("h2")
	hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "security"))
	hb.WriteElementClose("h2")

	// Warning if deprecated config is still present
	if a.hasDeprecatedConfig() {
		hb.WriteElementsOpen("p", "b")
		hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "deprecatedconfigwarning"))
		hb.WriteElementsClose("b", "p")
	}

	// Password section
	hb.WriteElementOpen("h3")
	hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "password"))
	hb.WriteElementClose("h3")

	// Show current password status
	if srd.hasDBPassword {
		hb.WriteElementOpen("p")
		hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "passwordset"))
		hb.WriteElementClose("p")
	} else {
		hb.WriteElementOpen("p")
		hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "nopasswordset"))
		hb.WriteElementClose("p")
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
	hb.WriteElementOpen("h3")
	hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "totp"))
	hb.WriteElementClose("h3")

	if srd.hasTOTP {
		hb.WriteElementOpen("p")
		hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "totpenabled"))
		hb.WriteElementClose("p")
		hb.WriteElementOpen("form", "class", "fw p", "method", "post", "action", rd.Blog.getRelativePath(settingsPath+settingsDeleteTOTPPath))
		hb.WriteElementOpen("input", "type", "submit", "class", "confirm", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "deletetotp"), "data-confirmmessage", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "confirmdeletetotp"))
		hb.WriteElementClose("form")
	} else {
		// Instructions
		hb.WriteElementOpen("p")
		hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "totpsetupinstructions"))
		hb.WriteElementClose("p")
		// Secret for manual entry
		hb.WriteElementOpen("p")
		hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "totpsecretmanual"))
		hb.WriteElementClose("p")
		hb.WriteElementOpen("p", "class", "monospace")
		hb.WriteElementOpen("code")
		hb.WriteEscaped(srd.newTotpSecret)
		hb.WriteElementClose("code")
		hb.WriteElementClose("p")
		// Verification form
		hb.WriteElementOpen("form", "class", "fw p", "method", "post", "action", rd.Blog.getRelativePath(settingsPath+settingsSetupTOTPPath))
		hb.WriteElementOpen("input", "type", "hidden", "name", "totpsecret", "value", srd.newTotpSecret)
		hb.WriteElementOpen("input", "type", "text", "inputmode", "numeric", "pattern", "[0-9]*", "name", "totpcode", "placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "totpcode"), "required", "")
		hb.WriteElementOpen("input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "setuptotp"))
		hb.WriteElementClose("form")
	}

	// Passkeys section
	hb.WriteElementOpen("h3")
	hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "passkeys"))
	hb.WriteElementClose("h3")

	// Register new passkey button (full width)
	hb.WriteElementOpen("form", "class", "fw p")
	hb.WriteElementOpen(
		"input", "id", "registerwebauthn", "type", "button", "class", "hide",
		"value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "registerpasskey"),
	)
	hb.WriteElementClose("form")

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
			hb.WriteElementOpen("button", "type", "submit", "formaction", rd.Blog.getRelativePath(settingsPath+settingsRenamePasskeyPath))
			hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "rename"))
			hb.WriteElementClose("button")
			hb.WriteElementOpen("button", "type", "submit", "formaction", rd.Blog.getRelativePath(settingsPath+settingsDeletePasskeyPath), "class", "confirm", "data-confirmmessage", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "confirmdelete"))
			hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "delete"))
			hb.WriteElementClose("button")
			hb.WriteElementsClose("form", "td")
			// Date column
			hb.WriteElementOpen("td", "class", "fixed")
			hb.WriteEscaped(pk.Created.Format(time.DateTime))
			hb.WriteElementsClose("td", "tr")
		}
		hb.WriteElementsClose("table")
	}

	// App passwords section
	hb.WriteElementOpen("h3")
	hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "apppasswords"))
	hb.WriteElementClose("h3")

	hb.WriteElementOpen("p")
	hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "apppasswordsdesc"))
	hb.WriteElementClose("p")

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
			hb.WriteElementOpen("button", "type", "submit", "formaction", rd.Blog.getRelativePath(settingsPath+settingsDeleteAppPasswordPath), "class", "confirm", "data-confirmmessage", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "confirmdelete"))
			hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "delete"))
			hb.WriteElementsClose("button", "form", "td")
			// Date column
			hb.WriteElementOpen("td", "class", "fixed")
			hb.WriteEscaped(ap.Created.Format(time.DateTime))
			hb.WriteElementsClose("td", "tr")
		}
		hb.WriteElementsClose("table")
	}
}

func (a *goBlog) renderAppPasswordCreated(hb *htmlbuilder.HtmlBuilder, rd *renderData) {
	data, ok := rd.Data.(*appPasswordCreatedRenderData)
	if !ok {
		return
	}
	a.renderBase(
		hb, rd,
		func(hb *htmlbuilder.HtmlBuilder) {
			a.renderTitleTag(hb, rd.Blog, a.ts.GetTemplateStringVariant(rd.Blog.Lang, "apppasswordcreated"))
		},
		func(hb *htmlbuilder.HtmlBuilder) {
			hb.WriteElementOpen("main")
			hb.WriteElementOpen("h1")
			hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "apppasswordcreated"))
			hb.WriteElementClose("h1")
			hb.WriteElementOpen("p")
			hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "apppasswordcreatedfor"))
			hb.WriteEscaped(" ")
			hb.WriteElementOpen("strong")
			hb.WriteEscaped(data.name)
			hb.WriteElementClose("strong")
			hb.WriteElementClose("p")
			hb.WriteElementOpen("p")
			hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "apppasswordtoken"))
			hb.WriteElementClose("p")
			hb.WriteElementOpen("p", "class", "monospace")
			hb.WriteElementOpen("code")
			hb.WriteEscaped(data.token)
			hb.WriteElementClose("code")
			hb.WriteElementClose("p")
			hb.WriteElementOpen("p")
			hb.WriteElementOpen("strong")
			hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "apppasswordwarning"))
			hb.WriteElementClose("strong")
			hb.WriteElementClose("p")
			hb.WriteElementOpen("p")
			hb.WriteElementOpen("a", "href", rd.Blog.getRelativePath(settingsPath))
			hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "backtosettings"))
			hb.WriteElementClose("a")
			hb.WriteElementClose("p")
			hb.WriteElementClose("main")
		},
	)
}

func (a *goBlog) renderFooter(origHb *htmlbuilder.HtmlBuilder, rd *renderData) {
	// Wrap plugins
	hb, finish := a.wrapForPlugins(origHb, a.getPlugins(pluginUiFooterType), func(plugin any, doc *goquery.Document) {
		plugin.(plugintypes.UIFooter).RenderFooter(rd.prc, doc)
	}, selectorBodyInner)
	defer finish()
	// Render footer
	hb.WriteElementOpen("footer")
	// Footer menu
	if fm, ok := rd.Blog.Menus["footer"]; ok {
		hb.WriteElementOpen("nav")
		for i, item := range fm.Items {
			if i > 0 {
				hb.WriteUnescaped(" &bull; ")
			}
			hb.WriteElementOpen("a", "href", item.Link)
			hb.WriteEscaped(a.renderMdTitle(item.Title))
			hb.WriteElementClose("a")
		}
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
