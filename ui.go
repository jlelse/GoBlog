package main

import (
	"fmt"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/kaorimatz/go-opml"
	"github.com/mergestat/timediff"
	"github.com/samber/lo"
	"go.goblog.app/app/pkgs/contenttype"
	"go.goblog.app/app/pkgs/htmlbuilder"
	"go.goblog.app/app/pkgs/plugintypes"
	"go.hacdias.com/indielib/indieauth"
)

func (a *goBlog) renderEditorPreview(hb *htmlbuilder.HtmlBuilder, bc *configBlog, p *post) {
	a.renderPostTitle(hb, p)
	a.renderPostMeta(hb, p, bc, "preview")
	a.postHtmlToWriter(hb, &postHtmlOptions{p: p, absolute: true})
	// a.renderPostGPX(hb, p, bc)
	a.renderPostTax(hb, p, bc)
}

func (a *goBlog) renderBase(hb *htmlbuilder.HtmlBuilder, rd *renderData, title, main func(hb *htmlbuilder.HtmlBuilder)) {
	// Basic HTML things
	hb.WriteUnescaped("<!doctype html>")
	hb.WriteElementOpen("html", "lang", rd.Blog.Lang)
	hb.WriteElementOpen("meta", "charset", "utf-8")
	hb.WriteElementOpen("meta", "name", "viewport", "content", "width=device-width,initial-scale=1")
	// CSS
	hb.WriteElementOpen("link", "rel", "stylesheet", "href", a.assetFileName("css/styles.css"))
	// Canonical URL
	if rd.Canonical != "" {
		hb.WriteElementOpen("link", "rel", "canonical", "href", rd.Canonical)
	}
	// Title
	if title != nil {
		title(hb)
	} else {
		a.renderTitleTag(hb, rd.Blog, "")
	}
	renderedBlogTitle := a.renderMdTitle(rd.Blog.Title)
	// Feeds
	hb.WriteElementOpen("link", "rel", "alternate", "type", "application/rss+xml", "title", fmt.Sprintf("RSS (%s)", renderedBlogTitle), "href", a.getFullAddress(rd.Blog.Path+".rss"))
	hb.WriteElementOpen("link", "rel", "alternate", "type", "application/atom+xml", "title", fmt.Sprintf("ATOM (%s)", renderedBlogTitle), "href", a.getFullAddress(rd.Blog.Path+".atom"))
	hb.WriteElementOpen("link", "rel", "alternate", "type", "application/feed+json", "title", fmt.Sprintf("JSON Feed (%s)", renderedBlogTitle), "href", a.getFullAddress(rd.Blog.Path+".json"))
	// Webmentions
	hb.WriteElementOpen("link", "rel", "webmention", "href", a.getFullAddress("/webmention"))
	// Micropub
	hb.WriteElementOpen("link", "rel", "micropub", "href", a.getFullAddress("/micropub"))
	// IndieAuth
	hb.WriteElementOpen("link", "rel", "authorization_endpoint", "href", a.getFullAddress("/indieauth"))
	hb.WriteElementOpen("link", "rel", "token_endpoint", "href", a.getFullAddress("/indieauth/token"))
	hb.WriteElementOpen("link", "rel", "indieauth-metadata", "href", a.getFullAddress("/.well-known/oauth-authorization-server"))
	// Rel-Me
	user := a.cfg.User
	if user != nil {
		for _, i := range user.Identities {
			hb.WriteElementOpen("link", "rel", "me", "href", i)
		}
	}
	// Opensearch
	if os := openSearchUrl(rd.Blog); os != "" {
		hb.WriteElementOpen("link", "rel", "search", "type", "application/opensearchdescription+xml", "href", os, "title", renderedBlogTitle)
	}
	// Favicons
	hb.WriteElementOpen("link", "rel", "icon", "type", contenttype.JPEG, "href", a.profileImagePath(profileImageFormatJPEG, 192, 0), "sizes", "192x192")
	hb.WriteElementOpen("link", "rel", "icon", "type", contenttype.JPEG, "href", a.profileImagePath(profileImageFormatJPEG, 256, 0), "sizes", "256x256")
	hb.WriteElementOpen("link", "rel", "icon", "type", contenttype.JPEG, "href", a.profileImagePath(profileImageFormatJPEG, 512, 0), "sizes", "512x512")
	hb.WriteElementOpen("link", "rel", "apple-touch-icon", "href", a.profileImagePath(profileImageFormatPNG, 180, 0))
	// Announcement
	if ann := rd.Blog.Announcement; ann != nil && ann.Text != "" {
		hb.WriteElementOpen("div", "id", "announcement", "data-nosnippet", "")
		_ = a.renderMarkdownToWriter(hb, ann.Text, false)
		hb.WriteElementClose("div")
	}
	// Header
	hb.WriteElementOpen("header")
	// Blog title
	hb.WriteElementOpen("h1")
	hb.WriteElementOpen("a", "href", rd.Blog.getRelativePath("/"), "rel", "home", "title", renderedBlogTitle, "translate", "no")
	hb.WriteEscaped(renderedBlogTitle)
	hb.WriteElementClose("a")
	hb.WriteElementClose("h1")
	// Blog description
	if rd.Blog.Description != "" {
		hb.WriteElementOpen("p")
		hb.WriteElementOpen("i")
		hb.WriteEscaped(rd.Blog.Description)
		hb.WriteElementClose("i")
		hb.WriteElementClose("p")
	}
	// Main menu
	if mm, ok := rd.Blog.Menus["main"]; ok {
		hb.WriteElementOpen("nav")
		for i, item := range mm.Items {
			if i > 0 {
				hb.WriteUnescaped(" &bull; ")
			}
			hb.WriteElementOpen("a", "href", item.Link)
			hb.WriteEscaped(a.renderMdTitle(item.Title))
			hb.WriteElementClose("a")
		}
		hb.WriteElementClose("nav")
	}
	// Logged-in user menu
	if rd.LoggedIn() {
		hb.WriteElementOpen("nav")
		hb.WriteElementOpen("a", "href", rd.Blog.getRelativePath("/editor"))
		hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "editor"))
		hb.WriteElementClose("a")
		hb.WriteUnescaped(" &bull; ")
		hb.WriteElementOpen("a", "href", "/notifications")
		hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "notifications"))
		hb.WriteElementClose("a")
		if rd.WebmentionReceivingEnabled {
			hb.WriteUnescaped(" &bull; ")
			hb.WriteElementOpen("a", "href", "/webmention")
			hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "webmentions"))
			hb.WriteElementClose("a")
		}
		if rd.Blog.commentsEnabled() {
			hb.WriteUnescaped(" &bull; ")
			hb.WriteElementOpen("a", "href", rd.Blog.getRelativePath(commentPath))
			hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "comments"))
			hb.WriteElementClose("a")
		}
		hb.WriteUnescaped(" &bull; ")
		hb.WriteElementOpen("a", "href", rd.Blog.getRelativePath("/settings"))
		hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "settings"))
		hb.WriteElementClose("a")
		hb.WriteUnescaped(" &bull; ")
		hb.WriteElementOpen("a", "href", "/logout")
		hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "logout"))
		hb.WriteElementClose("a")
		hb.WriteElementClose("nav")
	}
	hb.WriteElementClose("header")
	// Main
	if main != nil {
		main(hb)
	}
	// Footer
	a.renderFooter(hb, rd)
	// Easter egg
	if rd.EasterEgg {
		hb.WriteElementOpen("script", "src", a.assetFileName("js/easteregg.js"), "defer", "")
		hb.WriteElementClose("script")
	}
	hb.WriteElementClose("html")
}

type errorRenderData struct {
	Title   string
	Message string
}

func (a *goBlog) renderError(hb *htmlbuilder.HtmlBuilder, rd *renderData) {
	ed, ok := rd.Data.(*errorRenderData)
	if !ok {
		return
	}
	a.renderBase(
		hb, rd,
		func(hb *htmlbuilder.HtmlBuilder) {
			a.renderTitleTag(hb, rd.Blog, ed.Title)
		},
		func(hb *htmlbuilder.HtmlBuilder) {
			if ed.Title != "" {
				hb.WriteElementOpen("h1")
				hb.WriteEscaped(ed.Title)
				hb.WriteElementClose("h1")
			}
			if ed.Message != "" {
				hb.WriteElementOpen("p", "class", "monospace")
				hb.WriteEscaped(ed.Message)
				hb.WriteElementClose("p")
			}
		},
	)
}

type loginRenderData struct {
	loginMethod, loginHeaders, loginBody string
	totp                                 bool
}

func (a *goBlog) renderLogin(hb *htmlbuilder.HtmlBuilder, rd *renderData) {
	data, ok := rd.Data.(*loginRenderData)
	if !ok {
		return
	}
	a.renderBase(
		hb, rd,
		func(hb *htmlbuilder.HtmlBuilder) {
			a.renderTitleTag(hb, rd.Blog, a.ts.GetTemplateStringVariant(rd.Blog.Lang, "login"))
		},
		func(hb *htmlbuilder.HtmlBuilder) {
			hb.WriteElementOpen("main")
			// Title
			hb.WriteElementOpen("h1")
			hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "login"))
			hb.WriteElementClose("h1")
			// Form
			hb.WriteElementOpen("form", "class", "fw p", "method", "post")
			// Hidden fields
			hb.WriteElementOpen("input", "type", "hidden", "name", "loginaction", "value", "login")
			hb.WriteElementOpen("input", "type", "hidden", "name", "loginmethod", "value", data.loginMethod)
			hb.WriteElementOpen("input", "type", "hidden", "name", "loginheaders", "value", data.loginHeaders)
			hb.WriteElementOpen("input", "type", "hidden", "name", "loginbody", "value", data.loginBody)
			// Username
			hb.WriteElementOpen("input", "type", "text", "name", "username", "autocomplete", "username", "placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "username"), "required", "")
			// Password
			hb.WriteElementOpen("input", "type", "password", "name", "password", "autocomplete", "current-password", "placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "password"), "required", "")
			// TOTP
			if data.totp {
				hb.WriteElementOpen("input", "type", "text", "inputmode", "numeric", "pattern", "[0-9]*", "name", "token", "autocomplete", "one-time-code", "placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "totp"), "required", "")
			}
			// Submit
			hb.WriteElementOpen("input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "login"))
			hb.WriteElementClose("form")
			// Author (required for some IndieWeb apps)
			a.renderAuthor(hb)
			hb.WriteElementClose("main")
		},
	)
}

func (a *goBlog) renderSearch(hb *htmlbuilder.HtmlBuilder, rd *renderData) {
	sc := rd.Blog.Search
	renderedSearchTitle := a.renderMdTitle(sc.Title)
	a.renderBase(
		hb, rd,
		func(hb *htmlbuilder.HtmlBuilder) {
			a.renderTitleTag(hb, rd.Blog, renderedSearchTitle)
		},
		func(hb *htmlbuilder.HtmlBuilder) {
			hb.WriteElementOpen("main")
			titleOrDesc := false
			// Title
			if renderedSearchTitle != "" {
				titleOrDesc = true
				hb.WriteElementOpen("h1")
				hb.WriteEscaped(renderedSearchTitle)
				hb.WriteElementClose("h1")
			}
			// Description
			if sc.Description != "" {
				titleOrDesc = true
				_ = a.renderMarkdownToWriter(hb, sc.Description, false)
			}
			if titleOrDesc {
				hb.WriteElementOpen("hr")
			}
			// Form
			hb.WriteElementOpen("form", "class", "fw p", "method", "post")
			// Search
			args := []any{"type", "text", "name", "q", "required", ""}
			if sc.Placeholder != "" {
				args = append(args, "placeholder", a.renderMdTitle(sc.Placeholder))
			}
			hb.WriteElementOpen("input", args...)
			// Submit
			hb.WriteElementOpen("input", "type", "submit", "value", "🔍 "+a.ts.GetTemplateStringVariant(rd.Blog.Lang, "search"))
			hb.WriteElementClose("form")
			hb.WriteElementClose("main")
		},
	)
}

func (a *goBlog) renderComment(h *htmlbuilder.HtmlBuilder, rd *renderData) {
	c, ok := rd.Data.(*comment)
	if !ok {
		return
	}
	a.renderBase(
		h, rd,
		func(hb *htmlbuilder.HtmlBuilder) {
			hb.WriteElementOpen("title")
			hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "acommentby"))
			hb.WriteUnescaped(" ")
			hb.WriteEscaped(c.Name)
			hb.WriteElementClose("title")
		},
		func(hb *htmlbuilder.HtmlBuilder) {
			hb.WriteElementOpen("main", "class", "h-entry")
			// Target
			hb.WriteElementOpen("p")
			hb.WriteElementOpen("a", "class", "u-in-reply-to", "href", a.getFullAddress(c.Target))
			hb.WriteEscaped(a.getFullAddress(c.Target))
			hb.WriteElementClose("a")
			hb.WriteElementClose("p")
			// Author
			hb.WriteElementOpen("p", "class", "p-author h-card")
			hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "acommentby"))
			hb.WriteUnescaped(" ")
			if c.Website != "" {
				hb.WriteElementOpen("a", "class", "p-name u-url", "target", "_blank", "rel", "nofollow noopener noreferrer ugc", "href", c.Website)
				hb.WriteEscaped(c.Name)
				hb.WriteElementClose("a")
			} else {
				hb.WriteElementOpen("span", "class", "p-name")
				hb.WriteEscaped(c.Name)
				hb.WriteElementClose("span")
			}
			hb.WriteEscaped(":")
			hb.WriteElementClose("p")
			// Content
			hb.WriteElementOpen("p", "class", "e-content")
			hb.WriteUnescaped(c.Comment) // Already escaped
			hb.WriteElementClose("p")
			// Original
			if c.Original != "" {
				hb.WriteElementOpen("p", "class", "")
				hb.WriteElementOpen("a", "class", "u-url", "target", "_blank", "rel", "nofollow noopener noreferrer ugc", "href", c.Original)
				hb.WriteEscaped(c.Original)
				hb.WriteElementClose("a")
				hb.WriteElementClose("p")
			}
			hb.WriteElementClose("main")
			// Editor
			if rd.LoggedIn() {
				hb.WriteElementOpen("div", "class", "actions")
				hb.WriteElementOpen("a", "class", "button", "href", rd.Blog.getRelativePath(fmt.Sprintf("%s%s?id=%d", commentPath, commentEditSubPath, c.ID)))
				hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "edit"))
				hb.WriteElementClose("a")
				hb.WriteElementClose("div")
			}
			// Interactions
			if rd.Blog.commentsEnabled() {
				a.renderInteractions(hb, rd)
			}
		},
	)
}

type indexRenderData struct {
	title, description string
	posts              []*post
	hasPrev, hasNext   bool
	first, prev, next  string
	paramUrlQuery      string
	summaryTemplate    summaryTyp
}

func (a *goBlog) renderIndex(hb *htmlbuilder.HtmlBuilder, rd *renderData) {
	id, ok := rd.Data.(*indexRenderData)
	if !ok {
		return
	}
	renderedIndexTitle := a.renderMdTitle(id.title)
	a.renderBase(
		hb, rd,
		func(hb *htmlbuilder.HtmlBuilder) {
			// Title
			a.renderTitleTag(hb, rd.Blog, renderedIndexTitle)
			// Feeds
			feedTitle := ""
			if renderedIndexTitle != "" {
				feedTitle = " (" + renderedIndexTitle + ")"
			}
			hb.WriteElementOpen("link", "rel", "alternate", "type", "application/rss+xml", "title", "RSS"+feedTitle, "href", a.getFullAddress(id.first+".rss")+id.paramUrlQuery)
			hb.WriteElementOpen("link", "rel", "alternate", "type", "application/atom+xml", "title", "ATOM"+feedTitle, "href", a.getFullAddress(id.first+".atom")+id.paramUrlQuery)
			hb.WriteElementOpen("link", "rel", "alternate", "type", "application/feed+json", "title", "JSON Feed"+feedTitle, "href", a.getFullAddress(id.first+".json")+id.paramUrlQuery)
		},
		func(hb *htmlbuilder.HtmlBuilder) {
			hb.WriteElementOpen("main", "class", "h-feed")
			titleOrDesc := false
			// Title
			if renderedIndexTitle != "" {
				titleOrDesc = true
				hb.WriteElementOpen("h1", "class", "p-name")
				hb.WriteEscaped(renderedIndexTitle)
				hb.WriteElementClose("h1")
			}
			// Description
			if id.description != "" {
				titleOrDesc = true
				_ = a.renderMarkdownToWriter(hb, id.description, false)
			}
			if titleOrDesc {
				hb.WriteElementOpen("hr")
			}
			if id.posts != nil && len(id.posts) > 0 {
				// Posts
				for _, p := range id.posts {
					a.renderSummary(hb, rd, rd.Blog, p, id.summaryTemplate)
				}
			} else {
				// No posts
				hb.WriteElementOpen("p")
				hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "noposts"))
				hb.WriteElementClose("p")
			}
			// Navigation
			a.renderPagination(hb, rd.Blog, id.hasPrev, id.hasNext, id.prev+id.paramUrlQuery, id.next+id.paramUrlQuery)
			// Author
			a.renderAuthor(hb)
			hb.WriteElementClose("main")
		},
	)
}

type blogStatsRenderData struct {
	tableUrl string
}

func (a *goBlog) renderBlogStats(hb *htmlbuilder.HtmlBuilder, rd *renderData) {
	bsd, ok := rd.Data.(*blogStatsRenderData)
	if !ok {
		return
	}
	bs := rd.Blog.BlogStats
	renderedBSTitle := a.renderMdTitle(bs.Title)
	a.renderBase(
		hb, rd,
		func(hb *htmlbuilder.HtmlBuilder) {
			a.renderTitleTag(hb, rd.Blog, renderedBSTitle)
		},
		func(hb *htmlbuilder.HtmlBuilder) {
			hb.WriteElementOpen("main")
			// Title
			if renderedBSTitle != "" {
				hb.WriteElementOpen("h1")
				hb.WriteEscaped(renderedBSTitle)
				hb.WriteElementClose("h1")
			}
			// Description
			if bs.Description != "" {
				_ = a.renderMarkdownToWriter(hb, bs.Description, false)
			}
			// Table
			hb.WriteElementOpen("p", "id", "loading", "data-table", bsd.tableUrl)
			hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "loading"))
			hb.WriteElementClose("p")
			hb.WriteElementOpen("script", "src", a.assetFileName("js/blogstats.js"), "defer", "")
			hb.WriteElementClose("script")
			hb.WriteElementClose("main")
			// Interactions
			if rd.Blog.commentsEnabled() {
				a.renderInteractions(hb, rd)
			}
		},
	)
}

func (a *goBlog) renderBlogStatsTable(hb *htmlbuilder.HtmlBuilder, rd *renderData) {
	bsd, ok := rd.Data.(*blogStatsData)
	if !ok {
		return
	}
	hb.WriteElementOpen("table")
	// Table header
	hb.WriteElementOpen("thead")
	// Year
	hb.WriteElementOpen("th", "class", "tal")
	hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "year"))
	hb.WriteElementClose("th")
	// Posts
	hb.WriteElementOpen("th", "class", "tar")
	hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "posts"))
	hb.WriteElementClose("th")
	// Chars, Words, Words/Post
	for _, s := range []string{"chars", "words", "wordsperpost"} {
		hb.WriteElementOpen("th", "class", "tar")
		hb.WriteUnescaped("~")
		hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, s))
		hb.WriteElementClose("th")
	}
	hb.WriteElementClose("thead")
	// Table body
	hb.WriteElementOpen("tbody")
	// Iterate over years
	for _, y := range bsd.Years {
		// Stats for year
		hb.WriteElementOpen("tr", "class", "statsyear", "data-year", y.Name)
		hb.WriteElementOpen("td", "class", "tal")
		hb.WriteEscaped(y.Name)
		hb.WriteElementClose("td")
		hb.WriteElementOpen("td", "class", "tar")
		hb.WriteEscaped(y.Posts)
		hb.WriteElementClose("td")
		hb.WriteElementOpen("td", "class", "tar")
		hb.WriteEscaped(y.Chars)
		hb.WriteElementClose("td")
		hb.WriteElementOpen("td", "class", "tar")
		hb.WriteEscaped(y.Words)
		hb.WriteElementClose("td")
		hb.WriteElementOpen("td", "class", "tar")
		hb.WriteEscaped(y.WordsPerPost)
		hb.WriteElementClose("td")
		hb.WriteElementClose("tr")
		// Iterate over months
		for _, m := range bsd.Months[y.Name] {
			// Stats for month
			hb.WriteElementOpen("tr", "class", "statsmonth hide", "data-year", y.Name)
			hb.WriteElementOpen("td", "class", "tal")
			hb.WriteEscaped(y.Name)
			hb.WriteUnescaped("-")
			hb.WriteEscaped(m.Name)
			hb.WriteElementClose("td")
			hb.WriteElementOpen("td", "class", "tar")
			hb.WriteEscaped(m.Posts)
			hb.WriteElementClose("td")
			hb.WriteElementOpen("td", "class", "tar")
			hb.WriteEscaped(m.Chars)
			hb.WriteElementClose("td")
			hb.WriteElementOpen("td", "class", "tar")
			hb.WriteEscaped(m.Words)
			hb.WriteElementClose("td")
			hb.WriteElementOpen("td", "class", "tar")
			hb.WriteEscaped(m.WordsPerPost)
			hb.WriteElementClose("td")
			hb.WriteElementClose("tr")
		}
	}
	// Posts without date
	hb.WriteElementOpen("tr")
	hb.WriteElementOpen("td", "class", "tal")
	hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "withoutdate"))
	hb.WriteElementClose("td")
	hb.WriteElementOpen("td", "class", "tar")
	hb.WriteEscaped(bsd.NoDate.Posts)
	hb.WriteElementClose("td")
	hb.WriteElementOpen("td", "class", "tar")
	hb.WriteEscaped(bsd.NoDate.Chars)
	hb.WriteElementClose("td")
	hb.WriteElementOpen("td", "class", "tar")
	hb.WriteEscaped(bsd.NoDate.Words)
	hb.WriteElementClose("td")
	hb.WriteElementOpen("td", "class", "tar")
	hb.WriteEscaped(bsd.NoDate.WordsPerPost)
	hb.WriteElementClose("td")
	hb.WriteElementClose("tr")
	// Total
	hb.WriteElementOpen("tr")
	hb.WriteElementOpen("td", "class", "tal")
	hb.WriteElementOpen("strong")
	hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "total"))
	hb.WriteElementClose("strong")
	hb.WriteElementClose("td")
	hb.WriteElementOpen("td", "class", "tar")
	hb.WriteEscaped(bsd.Total.Posts)
	hb.WriteElementClose("td")
	hb.WriteElementOpen("td", "class", "tar")
	hb.WriteEscaped(bsd.Total.Chars)
	hb.WriteElementClose("td")
	hb.WriteElementOpen("td", "class", "tar")
	hb.WriteEscaped(bsd.Total.Words)
	hb.WriteElementClose("td")
	hb.WriteElementOpen("td", "class", "tar")
	hb.WriteEscaped(bsd.Total.WordsPerPost)
	hb.WriteElementClose("td")
	hb.WriteElementClose("tr")
	hb.WriteElementClose("tbody")
	hb.WriteElementClose("table")
}

type geoMapRenderData struct {
	noLocations bool
	locations   string
	tracks      string
	attribution string
	minZoom     int
	maxZoom     int
}

func (a *goBlog) renderGeoMap(hb *htmlbuilder.HtmlBuilder, rd *renderData) {
	gmd, ok := rd.Data.(*geoMapRenderData)
	if !ok {
		return
	}
	a.renderBase(
		hb, rd,
		func(hb *htmlbuilder.HtmlBuilder) {
			a.renderTitleTag(hb, rd.Blog, "")
		},
		func(hb *htmlbuilder.HtmlBuilder) {
			hb.WriteElementOpen("main")
			if gmd.noLocations {
				hb.WriteElementOpen("p")
				hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "nolocations"))
				hb.WriteElementClose("p")
			} else {
				hb.WriteElementOpen(
					"div", "id", "map", "class", "p",
					"data-locations", gmd.locations,
					"data-tracks", gmd.tracks,
					"data-minzoom", gmd.minZoom,
					"data-maxzoom", gmd.maxZoom,
					"data-attribution", gmd.attribution,
				)
				hb.WriteElementClose("div")
				hb.WriteElementOpen("script", "src", a.assetFileName("js/geomap.js"))
				hb.WriteElementClose("script")
			}
			hb.WriteElementClose("main")
			if rd.Blog.commentsEnabled() {
				a.renderInteractions(hb, rd)
			}
		},
	)
}

type blogrollRenderData struct {
	title       string
	description string
	outlines    []*opml.Outline
	download    string
}

func (a *goBlog) renderBlogroll(hb *htmlbuilder.HtmlBuilder, rd *renderData) {
	bd, ok := rd.Data.(*blogrollRenderData)
	if !ok {
		return
	}
	renderedTitle := a.renderMdTitle(bd.title)
	a.renderBase(
		hb, rd,
		func(hb *htmlbuilder.HtmlBuilder) {
			a.renderTitleTag(hb, rd.Blog, renderedTitle)
		},
		func(hb *htmlbuilder.HtmlBuilder) {
			hb.WriteElementOpen("main")
			// Title
			if renderedTitle != "" {
				hb.WriteElementOpen("h1")
				hb.WriteEscaped(renderedTitle)
				hb.WriteElementClose("h1")
			}
			// Description
			if bd.description != "" {
				hb.WriteElementOpen("p")
				_ = a.renderMarkdownToWriter(hb, bd.description, false)
				hb.WriteElementClose("p")
			}
			// Download button
			hb.WriteElementOpen("p")
			hb.WriteElementOpen("a", "href", rd.Blog.getRelativePath(bd.download), "class", "button", "download", "")
			hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "download"))
			hb.WriteElementClose("a")
			hb.WriteElementClose("p")
			// Outlines
			for _, outline := range bd.outlines {
				title := outline.Title
				if title == "" {
					title = outline.Text
				}
				hb.WriteElementOpen("h2", "id", urlize(title))
				hb.WriteEscaped(fmt.Sprintf("%s (%d)", title, len(outline.Outlines)))
				hb.WriteElementClose("h2")
				hb.WriteElementOpen("ul")
				for _, subOutline := range outline.Outlines {
					subTitle := subOutline.Title
					if subTitle == "" {
						subTitle = subOutline.Text
					}
					hb.WriteElementOpen("li")
					hb.WriteElementOpen("a", "href", subOutline.HTMLURL, "target", "_blank")
					hb.WriteEscaped(subTitle)
					hb.WriteElementClose("a")
					hb.WriteUnescaped(" (")
					hb.WriteElementOpen("a", "href", subOutline.XMLURL, "target", "_blank")
					hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "feed"))
					hb.WriteElementClose("a")
					hb.WriteUnescaped(")")
					hb.WriteElementClose("li")
				}
				hb.WriteElementClose("ul")
			}
			hb.WriteElementClose("main")
			// Interactions
			if rd.Blog.commentsEnabled() {
				a.renderInteractions(hb, rd)
			}
		},
	)
}

type contactRenderData struct {
	title       string
	description string
	privacy     string
}

func (a *goBlog) renderContact(hb *htmlbuilder.HtmlBuilder, rd *renderData) {
	cd, ok := rd.Data.(*contactRenderData)
	if !ok {
		return
	}
	renderedTitle := a.renderMdTitle(cd.title)
	a.renderBase(
		hb, rd,
		func(hb *htmlbuilder.HtmlBuilder) {
			a.renderTitleTag(hb, rd.Blog, renderedTitle)
		},
		func(hb *htmlbuilder.HtmlBuilder) {
			hb.WriteElementOpen("main")
			// Title
			if renderedTitle != "" {
				hb.WriteElementOpen("h1")
				hb.WriteEscaped(renderedTitle)
				hb.WriteElementClose("h1")
			}
			// Description
			if cd.description != "" {
				_ = a.renderMarkdownToWriter(hb, cd.description, false)
			}
			// Form
			hb.WriteElementOpen("form", "class", "fw p", "method", "post")
			// Name (optional)
			hb.WriteElementOpen("input", "type", "text", "name", "name", "placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "nameopt"))
			// Website (optional)
			hb.WriteElementOpen("input", "type", "url", "name", "website", "placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "websiteopt"))
			// Email (optional)
			hb.WriteElementOpen("input", "type", "email", "name", "email", "placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "emailopt"))
			// Message (required)
			hb.WriteElementOpen("textarea", "name", "message", "placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "message"), "required", "")
			hb.WriteElementClose("textarea")
			// Send
			if cd.privacy != "" {
				_ = a.renderMarkdownToWriter(hb, cd.privacy, false)
				hb.WriteElementOpen("input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "contactagreesend"))
			} else {
				hb.WriteElementOpen("input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "contactsend"))
			}
			hb.WriteElementsClose("form", "main")
		},
	)
}

func (a *goBlog) renderContactSent(hb *htmlbuilder.HtmlBuilder, rd *renderData) {
	a.renderBase(
		hb, rd, nil,
		func(hb *htmlbuilder.HtmlBuilder) {
			hb.WriteElementsOpen("main", "p")
			hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "messagesent"))
			hb.WriteElementsClose("p", "main")
		},
	)
}

type captchaRenderData struct {
	captchaMethod  string
	captchaHeaders string
	captchaBody    string
	captchaId      string
}

func (a *goBlog) renderCaptcha(hb *htmlbuilder.HtmlBuilder, rd *renderData) {
	crd, ok := rd.Data.(*captchaRenderData)
	if !ok {
		return
	}
	a.renderBase(
		hb, rd,
		func(hb *htmlbuilder.HtmlBuilder) {
			a.renderTitleTag(hb, rd.Blog, "")
		},
		func(hb *htmlbuilder.HtmlBuilder) {
			hb.WriteElementOpen("main")
			// Captcha image
			hb.WriteElementOpen("p")
			hb.WriteElementOpen("img", "src", "/captcha/"+crd.captchaId+".png", "class", "captchaimg")
			hb.WriteElementClose("p")
			// Form
			hb.WriteElementOpen("form", "class", "fw p", "method", "post")
			// Hidden fields
			hb.WriteElementOpen("input", "type", "hidden", "name", "captchaaction", "value", "captcha")
			hb.WriteElementOpen("input", "type", "hidden", "name", "captchamethod", "value", crd.captchaMethod)
			hb.WriteElementOpen("input", "type", "hidden", "name", "captchaheaders", "value", crd.captchaHeaders)
			hb.WriteElementOpen("input", "type", "hidden", "name", "captchabody", "value", crd.captchaBody)
			// Text
			hb.WriteElementOpen("input", "type", "text", "name", "digits", "placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "captchainstructions"), "required", "")
			// Submit
			hb.WriteElementOpen("input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "submit"))
			hb.WriteElementClose("form")
			hb.WriteElementClose("main")
		},
	)
}

type taxonomyRenderData struct {
	taxonomy    *configTaxonomy
	valueGroups []stringGroup
}

func (a *goBlog) renderTaxonomy(hb *htmlbuilder.HtmlBuilder, rd *renderData) {
	trd, ok := rd.Data.(*taxonomyRenderData)
	if !ok {
		return
	}
	renderedTitle := a.renderMdTitle(trd.taxonomy.Title)
	a.renderBase(
		hb, rd,
		func(hb *htmlbuilder.HtmlBuilder) {
			a.renderTitleTag(hb, rd.Blog, renderedTitle)
		},
		func(hb *htmlbuilder.HtmlBuilder) {
			hb.WriteElementOpen("main")
			// Title
			if renderedTitle != "" {
				hb.WriteElementOpen("h1")
				hb.WriteEscaped(renderedTitle)
				hb.WriteElementClose("h1")
			}
			// Description
			if trd.taxonomy.Description != "" {
				_ = a.renderMarkdownToWriter(hb, trd.taxonomy.Description, false)
			}
			// List
			for _, valGroup := range trd.valueGroups {
				// Title
				hb.WriteElementOpen("h2")
				hb.WriteEscaped(valGroup.Identifier)
				hb.WriteElementClose("h2")
				// List
				hb.WriteElementOpen("p")
				for i, val := range valGroup.Strings {
					if i > 0 {
						hb.WriteUnescaped(" &bull; ")
					}
					hb.WriteElementOpen("a", "href", rd.Blog.getRelativePath(fmt.Sprintf("/%s/%s", trd.taxonomy.Name, urlize(val))))
					hb.WriteEscaped(val)
					hb.WriteElementClose("a")
				}
				hb.WriteElementClose("p")
			}
		},
	)
}

func (a *goBlog) renderPost(hb *htmlbuilder.HtmlBuilder, rd *renderData) {
	p, ok := rd.Data.(*post)
	if !ok {
		return
	}
	a.renderBase(
		hb, rd,
		func(hb *htmlbuilder.HtmlBuilder) {
			if p.RenderedTitle != "" {
				a.renderTitleTag(hb, rd.Blog, p.RenderedTitle)
			} else {
				a.renderTitleTag(hb, rd.Blog, a.fallbackTitle(p))
			}
			hb.WriteElementOpen("link", "rel", "stylesheet", "href", a.assetFileName("css/chroma.css"))
			a.renderPostHeadMeta(hb, p)
			if su := a.shortPostURL(p); su != "" {
				hb.WriteElementOpen("link", "rel", "shortlink", "href", su)
			}
		},
		func(origHb *htmlbuilder.HtmlBuilder) {
			// Wrap plugins
			hb, finish := a.wrapForPlugins(origHb, a.getPlugins(pluginUiPostType), func(plugin any, doc *goquery.Document) {
				plugin.(plugintypes.UIPost).RenderPost(rd.prc, p, doc)
			})
			defer finish()
			// Render...
			hb.WriteElementOpen("main", "class", "h-entry")
			// URL (hidden just for microformats)
			hb.WriteElementOpen("data", "value", a.getFullAddress(p.Path), "class", "u-url hide")
			hb.WriteElementClose("data")
			// Start article
			hb.WriteElementOpen("article")
			// Title
			a.renderPostTitle(hb, p)
			// Post meta
			a.renderPostMeta(hb, p, rd.Blog, "post")
			// Post actions
			hb.WriteElementOpen("div", "class", "actions")
			// Share button
			a.renderShareButton(hb, p, rd.Blog)
			// Translate button
			a.renderTranslateButton(hb, p, rd.Blog)
			// Speak button
			hb.WriteElementOpen("button", "id", "speakBtn", "class", "hide", "data-speak", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "speak"), "data-stopspeak", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "stopspeak"))
			hb.WriteElementClose("button")
			hb.WriteElementOpen("script", "defer", "", "src", lo.If(p.TTS() != "", a.assetFileName("js/tts.js")).Else(a.assetFileName("js/speak.js")))
			hb.WriteElementClose("script")
			// Close post actions
			hb.WriteElementClose("div")
			// TTS
			if tts := p.TTS(); tts != "" {
				hb.WriteElementOpen("div", "class", "p hide", "id", "tts")
				hb.WriteElementOpen("audio", "controls", "", "preload", "none", "id", "tts-audio")
				hb.WriteElementOpen("source", "src", tts)
				hb.WriteElementClose("source")
				hb.WriteElementClose("audio")
				hb.WriteElementClose("div")
			}
			// Old content warning
			a.renderOldContentWarning(hb, p, rd.Blog)
			// Content
			a.postHtmlToWriter(hb, &postHtmlOptions{p: p})
			// External Videp
			a.renderPostVideo(hb, p)
			// GPS Track
			a.renderPostGPX(hb, p, rd.Blog)
			// Taxonomies
			a.renderPostTax(hb, p, rd.Blog)
			hb.WriteElementClose("article")
			// Author
			a.renderAuthor(hb)
			hb.WriteElementClose("main")
			// Reactions
			a.renderPostReactions(hb, p)
			// Post edit actions
			if rd.LoggedIn() {
				hb.WriteElementOpen("div", "class", "actions")
				// Update
				hb.WriteElementOpen("form", "method", "post", "action", rd.Blog.getRelativePath("/editor")+"#update")
				hb.WriteElementOpen("input", "type", "hidden", "name", "editoraction", "value", "loadupdate")
				hb.WriteElementOpen("input", "type", "hidden", "name", "path", "value", p.Path)
				hb.WriteElementOpen("input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "update"))
				hb.WriteElementClose("form")
				// Delete
				hb.WriteElementOpen("form", "method", "post", "action", rd.Blog.getRelativePath("/editor"))
				hb.WriteElementOpen("input", "type", "hidden", "name", "editoraction", "value", "delete")
				hb.WriteElementOpen("input", "type", "hidden", "name", "url", "value", rd.Canonical)
				hb.WriteElementOpen("input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "delete"), "class", "confirm", "data-confirmmessage", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "confirmdelete"))
				hb.WriteElementClose("form")
				// Undelete
				if p.Deleted() {
					hb.WriteElementOpen("form", "method", "post", "action", rd.Blog.getRelativePath("/editor"))
					hb.WriteElementOpen("input", "type", "hidden", "name", "editoraction", "value", "undelete")
					hb.WriteElementOpen("input", "type", "hidden", "name", "url", "value", rd.Canonical)
					hb.WriteElementOpen("input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "undelete"))
					hb.WriteElementClose("form")
				}
				// TTS
				if a.ttsEnabled() {
					hb.WriteElementOpen("form", "method", "post", "action", rd.Blog.getRelativePath("/editor"))
					hb.WriteElementOpen("input", "type", "hidden", "name", "editoraction", "value", "tts")
					hb.WriteElementOpen("input", "type", "hidden", "name", "url", "value", rd.Canonical)
					hb.WriteElementOpen("input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "gentts"))
					hb.WriteElementClose("form")
				}
				hb.WriteElementOpen("script", "defer", "", "src", a.assetFileName("js/formconfirm.js"))
				hb.WriteElementClose("script")
				hb.WriteElementClose("div")
			}
			// Comments
			if a.commentsEnabledForPost(p) {
				a.renderInteractions(hb, rd)
			}
		},
	)
}

func (a *goBlog) renderStaticHome(hb *htmlbuilder.HtmlBuilder, rd *renderData) {
	p, ok := rd.Data.(*post)
	if !ok {
		return
	}
	a.renderBase(
		hb, rd,
		func(hb *htmlbuilder.HtmlBuilder) {
			a.renderTitleTag(hb, rd.Blog, "")
			a.renderPostHeadMeta(hb, p)
		},
		func(hb *htmlbuilder.HtmlBuilder) {
			hb.WriteElementOpen("main", "class", "h-entry")
			hb.WriteElementOpen("article")
			// URL (hidden just for microformats)
			hb.WriteElementOpen("data", "value", a.getFullAddress(p.Path), "class", "u-url hide")
			hb.WriteElementClose("data")
			// Content
			if p.Content != "" {
				// Content
				a.postHtmlToWriter(hb, &postHtmlOptions{p: p})
			}
			// Author
			a.renderAuthor(hb)
			hb.WriteElementClose("article")
			hb.WriteElementClose("main")
			// Update
			if rd.LoggedIn() {
				hb.WriteElementOpen("div", "class", "actions")
				hb.WriteElementOpen("form", "method", "post", "action", rd.Blog.getRelativePath("/editor")+"#update")
				hb.WriteElementOpen("input", "type", "hidden", "name", "editoraction", "value", "loadupdate")
				hb.WriteElementOpen("input", "type", "hidden", "name", "path", "value", p.Path)
				hb.WriteElementOpen("input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "update"))
				hb.WriteElementClose("form")
				hb.WriteElementClose("div")
			}
		},
	)
}

func (a *goBlog) renderIndieAuth(hb *htmlbuilder.HtmlBuilder, rd *renderData) {
	indieAuthRequest, ok := rd.Data.(*indieauth.AuthenticationRequest)
	if !ok {
		return
	}
	a.renderBase(
		hb, rd,
		func(hb *htmlbuilder.HtmlBuilder) {
			a.renderTitleTag(hb, rd.Blog, a.ts.GetTemplateStringVariant(rd.Blog.Lang, "indieauth"))
		},
		func(hb *htmlbuilder.HtmlBuilder) {
			hb.WriteElementOpen("main")
			// Title
			hb.WriteElementOpen("h1")
			hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "indieauth"))
			hb.WriteElementClose("h1")
			hb.WriteElementClose("main")
			// Form
			hb.WriteElementOpen("form", "method", "post", "action", "/indieauth/accept", "class", "p")
			// Scopes
			if scopes := indieAuthRequest.Scopes; len(scopes) > 0 {
				hb.WriteElementOpen("h3")
				hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "scopes"))
				hb.WriteElementClose("h3")
				hb.WriteElementOpen("ul")
				for _, scope := range scopes {
					hb.WriteElementOpen("li")
					hb.WriteElementOpen("input", "type", "checkbox", "name", "scopes", "value", scope, "id", "scope-"+scope, "checked", "")
					hb.WriteElementOpen("label", "for", "scope-"+scope)
					hb.WriteEscaped(scope)
					hb.WriteElementClose("label")
					hb.WriteElementClose("li")
				}
				hb.WriteElementClose("ul")
			}
			// Client ID
			hb.WriteElementOpen("p")
			hb.WriteElementOpen("strong")
			hb.WriteEscaped("client_id:")
			hb.WriteElementClose("strong")
			hb.WriteUnescaped(" ")
			hb.WriteEscaped(indieAuthRequest.ClientID)
			hb.WriteElementClose("p")
			// Redirect URI
			hb.WriteElementOpen("p")
			hb.WriteElementOpen("strong")
			hb.WriteEscaped("redirect_uri:")
			hb.WriteElementClose("strong")
			hb.WriteUnescaped(" ")
			hb.WriteEscaped(indieAuthRequest.RedirectURI)
			hb.WriteElementClose("p")
			// Hidden form fields
			hb.WriteElementOpen("input", "type", "hidden", "name", "client_id", "value", indieAuthRequest.ClientID)
			hb.WriteElementOpen("input", "type", "hidden", "name", "redirect_uri", "value", indieAuthRequest.RedirectURI)
			hb.WriteElementOpen("input", "type", "hidden", "name", "state", "value", indieAuthRequest.State)
			hb.WriteElementOpen("input", "type", "hidden", "name", "code_challenge", "value", indieAuthRequest.CodeChallenge)
			hb.WriteElementOpen("input", "type", "hidden", "name", "code_challenge_method", "value", indieAuthRequest.CodeChallengeMethod)
			// Submit button
			hb.WriteElementOpen("input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "authenticate"))
			hb.WriteElementClose("form")
		},
	)
}

type editorFilesRenderData struct {
	files []*mediaFile
	uses  []int
}

func (a *goBlog) renderEditorFiles(hb *htmlbuilder.HtmlBuilder, rd *renderData) {
	ef, ok := rd.Data.(*editorFilesRenderData)
	if !ok {
		return
	}
	a.renderBase(
		hb, rd,
		func(hb *htmlbuilder.HtmlBuilder) {
			a.renderTitleTag(hb, rd.Blog, a.ts.GetTemplateStringVariant(rd.Blog.Lang, "mediafiles"))
		},
		func(hb *htmlbuilder.HtmlBuilder) {
			hb.WriteElementOpen("main")
			// Title
			hb.WriteElementOpen("h1")
			hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "mediafiles"))
			hb.WriteElementClose("h1")
			// Files
			if len(ef.files) > 0 {
				// Form
				hb.WriteElementOpen("form", "method", "post", "class", "fw p")
				// Select with number of uses
				hb.WriteElementOpen("select", "name", "filename")
				usesString := a.ts.GetTemplateStringVariant(rd.Blog.Lang, "fileuses")
				for i, f := range ef.files {
					hb.WriteElementOpen("option", "value", f.Name)
					hb.WriteEscaped(fmt.Sprintf("%s (%s), %s, ~%d %s", f.Name, f.Time.Local().Format(isoDateFormat), mBytesString(f.Size), ef.uses[i], usesString))
					hb.WriteElementClose("option")
				}
				hb.WriteElementClose("select")
				// View button
				hb.WriteElementOpen(
					"input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "view"),
					"formaction", rd.Blog.getRelativePath("/editor/files/view"),
				)
				// Delete button
				hb.WriteElementOpen(
					"input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "delete"),
					"formaction", rd.Blog.getRelativePath("/editor/files/delete"),
					"class", "confirm", "data-confirmmessage", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "confirmdelete"),
				)
				hb.WriteElementOpen("script", "src", a.assetFileName("js/formconfirm.js"), "defer", "")
				hb.WriteElementClose("script")
				hb.WriteElementClose("form")
			} else {
				hb.WriteElementOpen("p")
				hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "nofiles"))
				hb.WriteElementClose("p")
			}
			hb.WriteElementClose("main")
		},
	)
}

type notificationsRenderData struct {
	notifications    []*notification
	hasPrev, hasNext bool
	prev, next       string
}

func (a *goBlog) renderNotificationsAdmin(hb *htmlbuilder.HtmlBuilder, rd *renderData) {
	nrd, ok := rd.Data.(*notificationsRenderData)
	if !ok {
		return
	}
	a.renderBase(
		hb, rd,
		func(hb *htmlbuilder.HtmlBuilder) {
			a.renderTitleTag(hb, rd.Blog, a.ts.GetTemplateStringVariant(rd.Blog.Lang, "notifications"))
		},
		func(hb *htmlbuilder.HtmlBuilder) {
			hb.WriteElementOpen("main")
			// Title
			hb.WriteElementOpen("h1")
			hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "notifications"))
			hb.WriteElementClose("h1")
			// Delete all form
			hb.WriteElementOpen("form", "class", "actions", "method", "post", "action", "/notifications/delete")
			hb.WriteElementOpen("input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "deleteall"))
			hb.WriteElementClose("form")
			// Notifications
			tdLocale := matchTimeDiffLocale(rd.Blog.Lang)
			for _, n := range nrd.notifications {
				hb.WriteElementOpen("div", "class", "p")
				// Date
				hb.WriteElementOpen("p")
				hb.WriteElementOpen("i")
				hb.WriteEscaped(timediff.TimeDiff(time.Unix(n.Time, 0), timediff.WithLocale(tdLocale)))
				hb.WriteElementClose("i")
				hb.WriteElementClose("p")
				// Message
				hb.WriteElementOpen("pre")
				hb.WriteEscaped(n.Text)
				hb.WriteElementClose("pre")
				// Delete form
				hb.WriteElementOpen("form", "class", "actions", "method", "post", "action", "/notifications/delete")
				hb.WriteElementOpen("input", "type", "hidden", "name", "notificationid", "value", n.ID)
				hb.WriteElementOpen("input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "delete"))
				hb.WriteElementClose("form")
				hb.WriteElementClose("div")
			}
			// Pagination
			a.renderPagination(hb, rd.Blog, nrd.hasPrev, nrd.hasNext, nrd.prev, nrd.next)
			hb.WriteElementClose("main")
		},
	)
}

type commentsRenderData struct {
	comments         []*comment
	hasPrev, hasNext bool
	prev, next       string
}

func (a *goBlog) renderCommentsAdmin(hb *htmlbuilder.HtmlBuilder, rd *renderData) {
	crd, ok := rd.Data.(*commentsRenderData)
	if !ok {
		return
	}
	a.renderBase(
		hb, rd,
		func(hb *htmlbuilder.HtmlBuilder) {
			a.renderTitleTag(hb, rd.Blog, a.ts.GetTemplateStringVariant(rd.Blog.Lang, "comments"))
		},
		func(hb *htmlbuilder.HtmlBuilder) {
			hb.WriteElementOpen("main")
			// Title
			hb.WriteElementOpen("h1")
			hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "comments"))
			hb.WriteElementClose("h1")
			// Comments
			for _, c := range crd.comments {
				hb.WriteElementOpen("div", "class", "p")
				// ID, Target, Name
				hb.WriteElementOpen("p")
				hb.WriteEscaped("ID: ")
				hb.WriteEscaped(fmt.Sprintf("%d", c.ID))
				hb.WriteElementOpen("br")
				hb.WriteEscaped("Target: ")
				hb.WriteElementOpen("a", "href", c.Target, "target", "_blank")
				hb.WriteEscaped(c.Target)
				hb.WriteElementClose("a")
				hb.WriteElementOpen("br")
				hb.WriteEscaped("Name: ")
				if c.Website != "" {
					hb.WriteElementOpen("a", "href", c.Website, "target", "_blank", "rel", "nofollow noopener noreferrer ugc")
				}
				hb.WriteEscaped(c.Name)
				if c.Website != "" {
					hb.WriteElementClose("a")
				}
				if c.Original != "" {
					hb.WriteElementOpen("br")
					hb.WriteEscaped("Original: ")
					hb.WriteElementOpen("a", "href", c.Original, "target", "_blank", "rel", "nofollow noopener noreferrer ugc")
					hb.WriteEscaped(c.Original)
					hb.WriteElementClose("a")
				}
				hb.WriteElementClose("p")
				// Comment
				hb.WriteElementOpen("p")
				hb.WriteUnescaped(c.Comment)
				hb.WriteElementClose("p")
				// Delete form
				hb.WriteElementOpen("form", "class", "actions", "method", "post", "action", rd.Blog.getRelativePath(commentPath+commentDeleteSubPath))
				hb.WriteElementOpen("input", "type", "hidden", "name", "commentid", "value", c.ID)
				hb.WriteElementOpen("input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "delete"))
				hb.WriteElementClose("form")
				hb.WriteElementClose("div")
			}
			// Pagination
			a.renderPagination(hb, rd.Blog, crd.hasPrev, crd.hasNext, crd.prev, crd.next)
			hb.WriteElementClose("main")
		},
	)
}

type webmentionRenderData struct {
	mentions            []*mention
	hasPrev, hasNext    bool
	prev, current, next string
}

func (a *goBlog) renderWebmentionAdmin(hb *htmlbuilder.HtmlBuilder, rd *renderData) {
	wrd, ok := rd.Data.(*webmentionRenderData)
	if !ok {
		return
	}
	a.renderBase(
		hb, rd,
		func(hb *htmlbuilder.HtmlBuilder) {
			a.renderTitleTag(hb, rd.Blog, a.ts.GetTemplateStringVariant(rd.Blog.Lang, "webmentions"))
		},
		func(hb *htmlbuilder.HtmlBuilder) {
			hb.WriteElementOpen("main")
			// Title
			hb.WriteElementOpen("h1")
			hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "webmentions"))
			hb.WriteElementClose("h1")
			// Notifications
			tdLocale := matchTimeDiffLocale(rd.Blog.Lang)
			for _, m := range wrd.mentions {
				hb.WriteElementOpen("div", "id", fmt.Sprintf("mention-%d", m.ID), "class", "p")
				hb.WriteElementOpen("p")
				// Source
				hb.WriteEscaped("From: ")
				hb.WriteElementOpen("a", "href", m.Source, "target", "_blank", "rel", "noopener noreferrer")
				hb.WriteEscaped(m.Source)
				hb.WriteElementClose("a")
				hb.WriteElementOpen("br")
				// u-url
				if m.Source != m.Url {
					hb.WriteEscaped("u-url: ")
					hb.WriteElementOpen("a", "href", m.Url, "target", "_blank", "rel", "noopener noreferrer")
					hb.WriteEscaped(m.Url)
					hb.WriteElementClose("a")
					hb.WriteElementOpen("br")
				}
				// Target
				hb.WriteEscaped("To: ")
				hb.WriteElementOpen("a", "href", m.Target, "target", "_blank")
				hb.WriteEscaped(m.Target)
				hb.WriteElementClose("a")
				hb.WriteElementOpen("br")
				// Date
				hb.WriteEscaped("Created: ")
				hb.WriteEscaped(timediff.TimeDiff(time.Unix(m.Created, 0), timediff.WithLocale(tdLocale)))
				hb.WriteElementOpen("br")
				hb.WriteElementOpen("br")
				// Author
				if m.Author != "" {
					hb.WriteEscaped(m.Author)
					hb.WriteElementOpen("br")
				}
				// Title
				if m.Title != "" {
					hb.WriteElementOpen("strong")
					hb.WriteEscaped(m.Title)
					hb.WriteElementClose("strong")
					hb.WriteElementOpen("br")
				}
				// Content
				if m.Content != "" {
					hb.WriteElementOpen("i")
					hb.WriteEscaped(m.Content)
					hb.WriteElementClose("i")
					hb.WriteElementOpen("br")
				}
				hb.WriteElementClose("p")
				// Actions
				hb.WriteElementOpen("form", "method", "post", "class", "actions")
				hb.WriteElementOpen("input", "type", "hidden", "name", "mentionid", "value", m.ID)
				hb.WriteElementOpen("input", "type", "hidden", "name", "redir", "value", fmt.Sprintf("%s#mention-%d", wrd.current, m.ID))
				if m.Status == webmentionStatusVerified {
					// Approve verified mention
					hb.WriteElementOpen("input", "type", "submit", "formaction", "/webmention/approve", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "approve"))
				}
				// Delete mention
				hb.WriteElementOpen("input", "type", "submit", "formaction", "/webmention/delete", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "delete"))
				// Reverify mention
				hb.WriteElementOpen("input", "type", "submit", "formaction", "/webmention/reverify", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "reverify"))
				hb.WriteElementClose("form")
			}
			// Pagination
			a.renderPagination(hb, rd.Blog, wrd.hasPrev, wrd.hasNext, wrd.prev, wrd.next)
			hb.WriteElementClose("main")
		},
	)
}

type editorRenderData struct {
	updatePostUrl     string
	updatePostContent string
	presetParams      map[string][]string
}

func (a *goBlog) renderEditor(hb *htmlbuilder.HtmlBuilder, rd *renderData) {
	edrd, ok := rd.Data.(*editorRenderData)
	if !ok {
		return
	}
	a.renderBase(
		hb, rd,
		func(hb *htmlbuilder.HtmlBuilder) {
			a.renderTitleTag(hb, rd.Blog, a.ts.GetTemplateStringVariant(rd.Blog.Lang, "editor"))
			// Chroma CSS
			hb.WriteElementOpen("link", "rel", "stylesheet", "href", a.assetFileName("css/chroma.css"))
		},
		func(hb *htmlbuilder.HtmlBuilder) {
			hb.WriteElementOpen("main")
			// Title
			hb.WriteElementOpen("h1")
			hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "editor"))
			hb.WriteElementClose("h1")

			// Create
			hb.WriteElementOpen("h2")
			hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "create"))
			hb.WriteElementClose("h2")
			_ = a.renderMarkdownToWriter(hb, a.editorPostDesc(rd.Blog), false)
			hb.WriteElementOpen("form", "method", "post", "class", "fw p")
			hb.WriteElementOpen("input", "type", "hidden", "name", "editoraction", "value", "createpost")
			hb.WriteElementOpen(
				"input", "id", "templatebtn", "type", "button",
				"value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "editorusetemplate"),
			)
			hb.WriteElementOpen(
				"textarea",
				"id", "editor-create",
				"name", "content",
				"class", "monospace h400p",
				"id", "create-input",
				"data-preview", "post-preview",
				"data-previewws", rd.Blog.getRelativePath("/editor/preview"),
				"data-syncws", rd.Blog.getRelativePath("/editor/sync"),
				"data-template", a.editorPostTemplate(rd.BlogString, rd.Blog, edrd.presetParams),
			)
			hb.WriteElementClose("textarea")
			hb.WriteElementOpen("div", "id", "post-preview", "class", "hide")
			hb.WriteElementClose("div")
			hb.WriteElementOpen("input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "create"))
			hb.WriteElementClose("form")

			// Update
			if edrd.updatePostUrl != "" {
				hb.WriteElementOpen("h2", "id", "update")
				hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "update"))
				hb.WriteElementClose("h2")
				hb.WriteElementOpen("form", "method", "post", "class", "fw p", "action", "#update")
				hb.WriteElementOpen("input", "type", "hidden", "name", "editoraction", "value", "updatepost")
				hb.WriteElementOpen("input", "type", "hidden", "name", "url", "value", edrd.updatePostUrl)
				hb.WriteElementOpen(
					"textarea",
					"id", "editor-update",
					"name", "content",
					"class", "monospace h400p",
					"data-preview", "update-preview",
					"data-previewws", rd.Blog.getRelativePath("/editor/preview"),
				)
				hb.WriteEscaped(edrd.updatePostContent)
				hb.WriteElementClose("textarea")
				hb.WriteElementOpen("div", "id", "update-preview", "class", "hide")
				hb.WriteElementClose("div")
				hb.WriteElementOpen("input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "update"))
				hb.WriteElementClose("form")
			}

			// Posts
			hb.WriteElementOpen("h2")
			hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "posts"))
			hb.WriteElementClose("h2")
			// Template
			postsListLink := func(path, title string) {
				hb.WriteElementOpen("p")
				hb.WriteElementOpen("a", "href", rd.Blog.getRelativePath(path))
				hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, title))
				hb.WriteElementClose("a")
				hb.WriteElementClose("p")
			}
			// Drafts
			postsListLink("/editor/drafts", "drafts")
			// Private
			postsListLink("/editor/private", "privateposts")
			// Unlisted
			postsListLink("/editor/unlisted", "unlistedposts")
			// Scheduled
			postsListLink("/editor/scheduled", "scheduledposts")
			// Deleted
			postsListLink("/editor/deleted", "deletedposts")

			// Upload
			hb.WriteElementOpen("h2")
			hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "upload"))
			hb.WriteElementClose("h2")
			hb.WriteElementOpen("form", "class", "fw p", "method", "post", "enctype", "multipart/form-data")
			hb.WriteElementOpen("input", "type", "hidden", "name", "editoraction", "value", "upload")
			hb.WriteElementOpen("input", "type", "file", "name", "file")
			hb.WriteElementOpen("input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "upload"))
			hb.WriteElementClose("form")
			// Media files
			hb.WriteElementOpen("p")
			hb.WriteElementOpen("a", "href", rd.Blog.getRelativePath("/editor/files"))
			hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "mediafiles"))
			hb.WriteElementClose("a")
			hb.WriteElementClose("p")

			// Location-Helper
			hb.WriteElementOpen("h2")
			hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "location"))
			hb.WriteElementClose("h2")
			hb.WriteElementOpen("form", "class", "fw p")
			hb.WriteElementOpen(
				"input", "id", "geobtn", "type", "button",
				"value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "locationget"),
				"data-failed", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "locationfailed"),
				"data-notsupported", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "locationnotsupported"),
			)
			hb.WriteElementOpen("input", "id", "geostatus", "type", "text", "class", "hide", "readonly", "")
			hb.WriteElementClose("form")

			// GPX-Helper
			hb.WriteElementOpen("h2")
			hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "gpxhelper"))
			hb.WriteElementClose("h2")
			hb.WriteElementOpen("p")
			hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "gpxhelperdesc"))
			hb.WriteElementClose("p")
			hb.WriteElementOpen("form", "class", "fw p", "method", "post", "enctype", "multipart/form-data")
			hb.WriteElementOpen("input", "type", "hidden", "name", "editoraction", "value", "helpgpx")
			hb.WriteElementOpen("input", "type", "file", "name", "file")
			hb.WriteElementOpen("input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "upload"))
			hb.WriteElementClose("form")

			hb.WriteElementClose("main")

			// Script
			hb.WriteElementOpen("script", "src", a.assetFileName("js/editor.js"), "defer", "")
			hb.WriteElementClose("script")
		},
	)
}

type settingsRenderData struct {
	blog                  string
	sections              []*configSection
	defaultSection        string
	hideOldContentWarning bool
	hideShareButton       bool
	hideTranslateButton   bool
	addReplyTitle         bool
	addReplyContext       bool
	addLikeTitle          bool
	addLikeContext        bool
	userNick              string
	userName              string
}

func (a *goBlog) renderSettings(hb *htmlbuilder.HtmlBuilder, rd *renderData) {
	srd, ok := rd.Data.(*settingsRenderData)
	if !ok {
		return
	}
	a.renderBase(
		hb, rd,
		func(hb *htmlbuilder.HtmlBuilder) {
			a.renderTitleTag(hb, rd.Blog, a.ts.GetTemplateStringVariant(rd.Blog.Lang, "settings"))
		},
		func(hb *htmlbuilder.HtmlBuilder) {
			hb.WriteElementOpen("main")

			// Title
			hb.WriteElementOpen("h1")
			hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "settings"))
			hb.WriteElementClose("h1")

			// General
			hb.WriteElementOpen("h2")
			hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "general"))
			hb.WriteElementClose("h2")

			// Hide old content warning
			a.renderBooleanSetting(hb, rd,
				rd.Blog.getRelativePath(settingsPath+settingsHideOldContentWarningPath),
				a.ts.GetTemplateStringVariant(rd.Blog.Lang, "hideoldcontentwarningdesc"),
				hideOldContentWarningSetting,
				srd.hideOldContentWarning,
			)
			// Hide share button
			a.renderBooleanSetting(hb, rd,
				rd.Blog.getRelativePath(settingsPath+settingsHideShareButtonPath),
				a.ts.GetTemplateStringVariant(rd.Blog.Lang, "hidesharebuttondesc"),
				hideShareButtonSetting,
				srd.hideShareButton,
			)
			// Hide translate button
			a.renderBooleanSetting(hb, rd,
				rd.Blog.getRelativePath(settingsPath+settingsHideTranslateButtonPath),
				a.ts.GetTemplateStringVariant(rd.Blog.Lang, "hidetranslatebuttondesc"),
				hideTranslateButtonSetting,
				srd.hideTranslateButton,
			)
			// Add reply title
			a.renderBooleanSetting(hb, rd,
				rd.Blog.getRelativePath(settingsPath+settingsAddReplyTitlePath),
				a.ts.GetTemplateStringVariant(rd.Blog.Lang, "addreplytitledesc"),
				addReplyTitleSetting,
				srd.addReplyTitle,
			)
			// Add reply context
			a.renderBooleanSetting(hb, rd,
				rd.Blog.getRelativePath(settingsPath+settingsAddReplyContextPath),
				a.ts.GetTemplateStringVariant(rd.Blog.Lang, "addreplycontextdesc"),
				addReplyContextSetting,
				srd.addReplyContext,
			)
			// Add like title
			a.renderBooleanSetting(hb, rd,
				rd.Blog.getRelativePath(settingsPath+settingsAddLikeTitlePath),
				a.ts.GetTemplateStringVariant(rd.Blog.Lang, "addliketitledesc"),
				addLikeTitleSetting,
				srd.addLikeTitle,
			)
			// Add like context
			a.renderBooleanSetting(hb, rd,
				rd.Blog.getRelativePath(settingsPath+settingsAddLikeContextPath),
				a.ts.GetTemplateStringVariant(rd.Blog.Lang, "addlikecontextdesc"),
				addLikeContextSetting,
				srd.addLikeContext,
			)

			// User settings
			a.renderUserSettings(hb, rd, srd)

			// Post sections
			a.renderPostSectionSettings(hb, rd, srd)

			// Scripts
			hb.WriteElementOpen("script", "src", a.assetFileName("js/settings.js"), "defer", "")
			hb.WriteElementClose("script")
			hb.WriteElementOpen("script", "src", a.assetFileName("js/formconfirm.js"), "defer", "")
			hb.WriteElementClose("script")

			hb.WriteElementClose("main")
		},
	)
}

type activityPubFollowersRenderData struct {
	apUser    string
	followers []*apFollower
}

func (a *goBlog) renderActivityPubFollowers(hb *htmlbuilder.HtmlBuilder, rd *renderData) {
	aprd, ok := rd.Data.(*activityPubFollowersRenderData)
	if !ok {
		return
	}
	a.renderBase(
		hb, rd,
		func(hb *htmlbuilder.HtmlBuilder) {
			a.renderTitleTag(hb, rd.Blog, a.ts.GetTemplateStringVariant(rd.Blog.Lang, "apfollowers"))
		},
		func(hb *htmlbuilder.HtmlBuilder) {
			hb.WriteElementOpen("main")

			// Title
			hb.WriteElementOpen("h1")
			hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "apfollowers"))
			hb.WriteEscaped(": ")
			hb.WriteEscaped(aprd.apUser)
			hb.WriteElementClose("h1")

			// List followers
			hb.WriteElementOpen("ul")
			for _, follower := range aprd.followers {
				hb.WriteElementOpen("li")
				hb.WriteElementOpen("a", "href", follower.follower, "target", "_blank")
				hb.WriteEscaped(follower.username)
				hb.WriteElementClose("a")
				hb.WriteElementClose("li")
			}
			hb.WriteElementClose("ul")

			hb.WriteElementClose("main")
		},
	)
}

func (a *goBlog) renderActivityPubRemoteFollow(hb *htmlbuilder.HtmlBuilder, rd *renderData) {
	a.renderBase(
		hb, rd,
		func(hb *htmlbuilder.HtmlBuilder) {
			a.renderTitleTag(hb, rd.Blog, a.ts.GetTemplateStringVariant(rd.Blog.Lang, "followusingactivitypub"))
		},
		func(hb *htmlbuilder.HtmlBuilder) {
			hb.WriteElementOpen("main")

			// Title
			hb.WriteElementOpen("h1")
			hb.WriteEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "followusingactivitypub"))
			hb.WriteElementClose("h1")

			// Form
			hb.WriteElementOpen("form", "class", "fw p", "method", "post")
			hb.WriteElementOpen("input", "type", "text", "name", "user", "placeholder", "user@example.org")
			hb.WriteElementOpen("input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "follow"))
			hb.WriteElementClose("form")

			hb.WriteElementClose("main")
		},
	)
}

func (a *goBlog) renderCommentEditor(h *htmlbuilder.HtmlBuilder, rd *renderData) {
	c, ok := rd.Data.(*comment)
	if !ok {
		return
	}
	a.renderBase(
		h, rd,
		func(hb *htmlbuilder.HtmlBuilder) {
			a.renderTitleTag(hb, rd.Blog, a.ts.GetTemplateStringVariant(rd.Blog.Lang, "editcommenttitle"))
		},
		func(hb *htmlbuilder.HtmlBuilder) {
			// Form
			hb.WriteElementOpen("form", "class", "fw p", "method", "post")
			hb.WriteElementOpen("input", "type", "hidden", "name", "id", "value", c.ID)
			hb.WriteElementOpen("input", "type", "text", "disabled", "", "value", c.Target)
			if c.Original != "" {
				hb.WriteElementOpen("input", "type", "text", "disabled", "", "value", c.Original)
			}
			if c.Name != "" {
				hb.WriteElementOpen("input", "type", "text", "name", "name", "placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "nameopt"), "value", c.Name)
			}
			if c.Website != "" {
				hb.WriteElementOpen("input", "type", "url", "name", "website", "placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "websiteopt"), "value", c.Website)
			}
			hb.WriteElementOpen("textarea", "name", "comment", "required", "", "placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "comment"))
			hb.WriteEscaped(c.Comment)
			hb.WriteElementClose("textarea")
			hb.WriteElementOpen("input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "update"))
			hb.WriteElementClose("form")
		},
	)
}
