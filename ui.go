package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/kaorimatz/go-opml"
	"github.com/thoas/go-funk"
)

func (a *goBlog) renderEditorPreview(hb *htmlBuilder, bc *configBlog, p *post) {
	if p.RenderedTitle != "" {
		hb.writeElementOpen("h1")
		hb.writeEscaped(p.RenderedTitle)
		hb.writeElementClose("h1")
	}
	a.renderPostMeta(hb, p, bc, "preview")
	if p.Content != "" {
		hb.writeElementOpen("div")
		hb.writeHtml(a.postHtml(p, true))
		hb.writeElementClose("div")
	}
	a.renderPostTax(hb, p, bc)
}

func (a *goBlog) renderBase(hb *htmlBuilder, rd *renderData, title, main func(hb *htmlBuilder)) {
	// Basic HTML things
	hb.write("<!doctype html>")
	hb.writeElementOpen("html", "lang", rd.Blog.Lang)
	hb.writeElementOpen("meta", "charset", "utf-8")
	hb.writeElementOpen("meta", "name", "viewport", "content", "width=device-width,initial-scale=1")
	// CSS
	hb.writeElementOpen("link", "rel", "stylesheet", "href", a.assetFileName("css/styles.css"))
	// Canonical URL
	if rd.Canonical != "" {
		hb.writeElementOpen("link", "rel", "canonical", "href", rd.Canonical)
	}
	// Title
	if title != nil {
		title(hb)
	} else {
		a.renderTitleTag(hb, rd.Blog, "")
	}
	// Feeds
	renderedBlogTitle := a.renderMdTitle(rd.Blog.Title)
	// RSS
	hb.writeElementOpen("link", "rel", "alternate", "type", "application/rss+xml", "title", fmt.Sprintf("RSS (%s)", renderedBlogTitle), "href", rd.Blog.Path+".rss")
	// ATOM
	hb.writeElementOpen("link", "rel", "alternate", "type", "application/atom+xml", "title", fmt.Sprintf("ATOM (%s)", renderedBlogTitle), "href", rd.Blog.Path+".atom")
	// JSON Feed
	hb.writeElementOpen("link", "rel", "alternate", "type", "application/feed+json", "title", fmt.Sprintf("JSON Feed (%s)", renderedBlogTitle), "href", rd.Blog.Path+".json")
	// Webmentions
	hb.writeElementOpen("link", "rel", "webmention", "href", a.getFullAddress("/webmention"))
	// Micropub
	hb.writeElementOpen("link", "rel", "micropub", "href", "/micropub")
	// IndieAuth
	hb.writeElementOpen("link", "rel", "authorization_endpoint", "href", "/indieauth")
	hb.writeElementOpen("link", "rel", "token_endpoint", "href", "/indieauth/token")
	// Rel-Me
	user := a.cfg.User
	if user != nil {
		for _, i := range user.Identities {
			hb.writeElementOpen("link", "rel", "me", "href", i)
		}
	}
	// Opensearch
	if os := openSearchUrl(rd.Blog); os != "" {
		hb.writeElementOpen("link", "rel", "search", "type", "application/opensearchdescription+xml", "href", os, "title", renderedBlogTitle)
	}
	// Announcement
	if ann := rd.Blog.Announcement; ann != nil && ann.Text != "" {
		hb.writeElementOpen("div", "id", "announcement", "data-nosnippet", "")
		_ = a.renderMarkdownToWriter(hb, ann.Text, false)
		hb.writeElementClose("div")
	}
	// Header
	hb.writeElementOpen("header")
	// Blog title
	hb.writeElementOpen("h1")
	hb.writeElementOpen("a", "href", rd.Blog.getRelativePath("/"), "rel", "home", "title", renderedBlogTitle, "translate", "no")
	hb.writeEscaped(renderedBlogTitle)
	hb.writeElementClose("a")
	hb.writeElementClose("h1")
	// Blog description
	if rd.Blog.Description != "" {
		hb.writeElementOpen("p")
		hb.writeElementOpen("i")
		hb.writeEscaped(rd.Blog.Description)
		hb.writeElementClose("i")
		hb.writeElementClose("p")
	}
	// Main menu
	if mm, ok := rd.Blog.Menus["main"]; ok {
		hb.writeElementOpen("nav")
		for i, item := range mm.Items {
			if i > 0 {
				hb.write(" &bull; ")
			}
			hb.writeElementOpen("a", "href", item.Link)
			hb.writeEscaped(a.renderMdTitle(item.Title))
			hb.writeElementClose("a")
		}
		hb.writeElementClose("nav")
	}
	// Logged-in user menu
	if rd.LoggedIn() {
		hb.writeElementOpen("nav")
		hb.writeElementOpen("a", "href", rd.Blog.getRelativePath("/editor"))
		hb.writeEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "editor"))
		hb.writeElementClose("a")
		hb.write(" &bull; ")
		hb.writeElementOpen("a", "href", "/notifications")
		hb.writeEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "notifications"))
		hb.writeElementClose("a")
		if rd.WebmentionReceivingEnabled {
			hb.write(" &bull; ")
			hb.writeElementOpen("a", "href", "/webmention")
			hb.writeEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "webmentions"))
			hb.writeElementClose("a")
		}
		if rd.CommentsEnabled {
			hb.write(" &bull; ")
			hb.writeElementOpen("a", "href", "/comment")
			hb.writeEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "comments"))
			hb.writeElementClose("a")
		}
		hb.write(" &bull; ")
		hb.writeElementOpen("a", "href", "/logout")
		hb.writeEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "logout"))
		hb.writeElementClose("a")
		hb.writeElementClose("nav")
	}
	hb.writeElementClose("header")
	// Main
	if main != nil {
		main(hb)
	}
	// Footer
	hb.writeElementOpen("footer")
	// Footer menu
	if fm, ok := rd.Blog.Menus["footer"]; ok {
		hb.writeElementOpen("nav")
		for i, item := range fm.Items {
			if i > 0 {
				hb.write(" &bull; ")
			}
			hb.writeElementOpen("a", "href", item.Link)
			hb.writeEscaped(a.renderMdTitle(item.Title))
			hb.writeElementClose("a")
		}
		hb.writeElementClose("nav")
	}
	// Copyright
	hb.writeElementOpen("p", "translate", "no")
	hb.write("&copy; ")
	hb.writeEscaped(time.Now().Format("2006"))
	hb.write(" ")
	if user != nil && user.Name != "" {
		hb.writeEscaped(user.Name)
	} else {
		hb.writeEscaped(renderedBlogTitle)
	}
	hb.writeElementClose("p")
	// Tor
	a.renderTorNotice(hb, rd)
	hb.writeElementClose("footer")
	// Easter egg
	if rd.EasterEgg {
		hb.writeElementOpen("script", "src", a.assetFileName("js/easteregg.js"), "defer", "")
		hb.writeElementClose("script")
	}
	hb.writeElementClose("html")
}

type errorRenderData struct {
	Title   string
	Message string
}

func (a *goBlog) renderError(hb *htmlBuilder, rd *renderData) {
	ed, ok := rd.Data.(*errorRenderData)
	if !ok {
		return
	}
	a.renderBase(
		hb, rd,
		func(hb *htmlBuilder) {
			a.renderTitleTag(hb, rd.Blog, ed.Title)
		},
		func(hb *htmlBuilder) {
			if ed.Title != "" {
				hb.writeElementOpen("h1")
				hb.writeEscaped(ed.Title)
				hb.writeElementClose("h1")
			}
			if ed.Message != "" {
				hb.writeElementOpen("p", "class", "monospace")
				hb.writeEscaped(ed.Message)
				hb.writeElementClose("p")
			}
		},
	)
}

type loginRenderData struct {
	loginMethod, loginHeaders, loginBody string
	totp                                 bool
}

func (a *goBlog) renderLogin(hb *htmlBuilder, rd *renderData) {
	data, ok := rd.Data.(*loginRenderData)
	if !ok {
		return
	}
	a.renderBase(
		hb, rd,
		func(hb *htmlBuilder) {
			a.renderTitleTag(hb, rd.Blog, a.ts.GetTemplateStringVariant(rd.Blog.Lang, "login"))
		},
		func(hb *htmlBuilder) {
			hb.writeElementOpen("main")
			// Title
			hb.writeElementOpen("h1")
			hb.writeEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "login"))
			hb.writeElementClose("h1")
			// Form
			hb.writeElementOpen("form", "class", "fw p", "method", "post")
			// Hidden fields
			hb.writeElementOpen("input", "type", "hidden", "name", "loginaction", "value", "login")
			hb.writeElementOpen("input", "type", "hidden", "name", "loginmethod", "value", data.loginMethod)
			hb.writeElementOpen("input", "type", "hidden", "name", "loginheaders", "value", data.loginHeaders)
			hb.writeElementOpen("input", "type", "hidden", "name", "loginbody", "value", data.loginBody)
			// Username
			hb.writeElementOpen("input", "type", "text", "name", "username", "autocomplete", "username", "placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "username"), "required", "")
			// Password
			hb.writeElementOpen("input", "type", "password", "name", "password", "autocomplete", "current-password", "placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "password"), "required", "")
			// TOTP
			if data.totp {
				hb.writeElementOpen("input", "type", "text", "inputmode", "numeric", "pattern", "[0-9]*", "name", "token", "autocomplete", "one-time-code", "placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "totp"), "required", "")
			}
			// Submit
			hb.writeElementOpen("input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "login"))
			hb.writeElementClose("form")
			// Author (required for some IndieWeb apps)
			a.renderAuthor(hb)
			hb.writeElementClose("main")
		},
	)
}

func (a *goBlog) renderSearch(hb *htmlBuilder, rd *renderData) {
	sc := rd.Blog.Search
	renderedSearchTitle := a.renderMdTitle(sc.Title)
	a.renderBase(
		hb, rd,
		func(hb *htmlBuilder) {
			a.renderTitleTag(hb, rd.Blog, renderedSearchTitle)
		},
		func(hb *htmlBuilder) {
			hb.writeElementOpen("main")
			titleOrDesc := false
			// Title
			if renderedSearchTitle != "" {
				titleOrDesc = true
				hb.writeElementOpen("h1")
				hb.writeEscaped(renderedSearchTitle)
				hb.writeElementClose("h1")
			}
			// Description
			if sc.Description != "" {
				titleOrDesc = true
				_ = a.renderMarkdownToWriter(hb, sc.Description, false)
			}
			if titleOrDesc {
				hb.writeElementOpen("hr")
			}
			// Form
			hb.writeElementOpen("form", "class", "fw p", "method", "post")
			// Search
			args := []interface{}{"type", "text", "name", "q", "required", ""}
			if sc.Placeholder != "" {
				args = append(args, "placeholder", a.renderMdTitle(sc.Placeholder))
			}
			hb.writeElementOpen("input", args...)
			// Submit
			hb.writeElementOpen("input", "type", "submit", "value", "🔍 "+a.ts.GetTemplateStringVariant(rd.Blog.Lang, "search"))
			hb.writeElementClose("form")
			hb.writeElementClose("main")
		},
	)
}

func (a *goBlog) renderComment(h *htmlBuilder, rd *renderData) {
	c, ok := rd.Data.(*comment)
	if !ok {
		return
	}
	a.renderBase(
		h, rd,
		func(hb *htmlBuilder) {
			hb.writeElementOpen("title")
			hb.writeEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "acommentby"))
			hb.write(" ")
			hb.writeEscaped(c.Name)
			hb.writeElementClose("title")
		},
		func(hb *htmlBuilder) {
			hb.writeElementOpen("main", "class", "h-entry")
			// Target
			hb.writeElementOpen("p")
			hb.writeElementOpen("a", "class", "u-in-reply-to", "href", a.getFullAddress(c.Target))
			hb.writeEscaped(a.getFullAddress(c.Target))
			hb.writeElementClose("a")
			hb.writeElementClose("p")
			// Author
			hb.writeElementOpen("p", "class", "p-author h-card")
			hb.writeEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "acommentby"))
			hb.write(" ")
			if c.Website != "" {
				hb.writeElementOpen("a", "class", "p-name u-url", "target", "_blank", "rel", "nofollow noopener noreferrer ugc", "href", c.Website)
				hb.writeEscaped(c.Name)
				hb.writeElementClose("a")
			} else {
				hb.writeElementOpen("span", "class", "p-name")
				hb.writeEscaped(c.Name)
				hb.writeElementClose("span")
			}
			hb.writeEscaped(":")
			hb.writeElementClose("p")
			// Content
			hb.writeElementOpen("p", "class", "e-content")
			hb.write(c.Comment) // Already escaped
			hb.writeElementClose("p")
			hb.writeElementClose("main")
		},
	)
}

type indexRenderData struct {
	title, description string
	posts              []*post
	hasPrev, hasNext   bool
	first, prev, next  string
	summaryTemplate    summaryTyp
}

func (a *goBlog) renderIndex(hb *htmlBuilder, rd *renderData) {
	id, ok := rd.Data.(*indexRenderData)
	if !ok {
		return
	}
	renderedIndexTitle := a.renderMdTitle(id.title)
	a.renderBase(
		hb, rd,
		func(hb *htmlBuilder) {
			// Title
			a.renderTitleTag(hb, rd.Blog, renderedIndexTitle)
			// Feeds
			feedTitle := ""
			if renderedIndexTitle != "" {
				feedTitle = " (" + renderedIndexTitle + ")"
			}
			// RSS
			hb.writeElementOpen("link", "rel", "alternate", "type", "application/rss+xml", "title", "RSS"+feedTitle, "href", id.first+".rss")
			// ATOM
			hb.writeElementOpen("link", "rel", "alternate", "type", "application/atom+xml", "title", "AROM"+feedTitle, "href", id.first+".atom")
			// JSON Feed
			hb.writeElementOpen("link", "rel", "alternate", "type", "application/feed+json", "title", "JSON Feed"+feedTitle, "href", id.first+".json")
		},
		func(hb *htmlBuilder) {
			hb.writeElementOpen("main", "class", "h-feed")
			titleOrDesc := false
			// Title
			if renderedIndexTitle != "" {
				titleOrDesc = true
				hb.writeElementOpen("h1", "class", "p-name")
				hb.writeEscaped(renderedIndexTitle)
				hb.writeElementClose("h1")
			}
			// Description
			if id.description != "" {
				titleOrDesc = true
				_ = a.renderMarkdownToWriter(hb, id.description, false)
			}
			if titleOrDesc {
				hb.writeElementOpen("hr")
			}
			if id.posts != nil && len(id.posts) > 0 {
				// Posts
				for _, p := range id.posts {
					a.renderSummary(hb, rd.Blog, p, id.summaryTemplate)
				}
			} else {
				// No posts
				hb.writeElementOpen("p")
				hb.writeEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "noposts"))
				hb.writeElementClose("p")
			}
			// Navigation
			if id.hasPrev {
				hb.writeElementOpen("p")
				hb.writeElementOpen("a", "href", id.prev) // TODO: rel=prev?
				hb.writeEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "prev"))
				hb.writeElementClose("a")
				hb.writeElementClose("p")
			}
			if id.hasNext {
				hb.writeElementOpen("p")
				hb.writeElementOpen("a", "href", id.next) // TODO: rel=next?
				hb.writeEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "next"))
				hb.writeElementClose("a")
				hb.writeElementClose("p")
			}
			// Author
			a.renderAuthor(hb)
			hb.writeElementClose("main")
		},
	)
}

type blogStatsRenderData struct {
	tableUrl string
}

func (a *goBlog) renderBlogStats(hb *htmlBuilder, rd *renderData) {
	bsd, ok := rd.Data.(*blogStatsRenderData)
	if !ok {
		return
	}
	bs := rd.Blog.BlogStats
	renderedBSTitle := a.renderMdTitle(bs.Title)
	a.renderBase(
		hb, rd,
		func(hb *htmlBuilder) {
			a.renderTitleTag(hb, rd.Blog, renderedBSTitle)
		},
		func(hb *htmlBuilder) {
			hb.writeElementOpen("main")
			// Title
			if renderedBSTitle != "" {
				hb.writeElementOpen("h1")
				hb.writeEscaped(renderedBSTitle)
				hb.writeElementClose("h1")
			}
			// Description
			if bs.Description != "" {
				_ = a.renderMarkdownToWriter(hb, bs.Description, false)
			}
			// Table
			hb.writeElementOpen("p", "id", "loading", "data-table", bsd.tableUrl)
			hb.writeEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "loading"))
			hb.writeElementClose("p")
			hb.writeElementOpen("script", "src", a.assetFileName("js/blogstats.js"), "defer", "")
			hb.writeElementClose("script")
			hb.writeElementClose("main")
			// Interactions
			if rd.CommentsEnabled {
				a.renderInteractions(hb, rd.Blog, rd.Canonical)
			}
		},
	)
}

func (a *goBlog) renderBlogStatsTable(hb *htmlBuilder, rd *renderData) {
	bsd, ok := rd.Data.(*blogStatsData)
	if !ok {
		return
	}
	hb.writeElementOpen("table")
	// Table header
	hb.writeElementOpen("thead")
	hb.writeElementOpen("tr")
	// Year
	hb.writeElementOpen("th", "class", "tal")
	hb.writeEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "year"))
	hb.writeElementClose("th")
	// Posts
	hb.writeElementOpen("th", "class", "tar")
	hb.writeEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "posts"))
	hb.writeElementClose("th")
	// Chars
	hb.writeElementOpen("th", "class", "tar")
	hb.write("~")
	hb.writeEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "chars"))
	hb.writeElementClose("th")
	// Words
	hb.writeElementOpen("th", "class", "tar")
	hb.write("~")
	hb.writeEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "words"))
	hb.writeElementClose("th")
	// Words/post
	hb.writeElementOpen("th", "class", "tar")
	hb.write("~")
	hb.writeEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "wordsperpost"))
	hb.writeElementClose("th")
	hb.writeElementClose("thead")
	// Table body
	hb.writeElementOpen("tbody")
	// Iterate over years
	for _, y := range bsd.Years {
		// Stats for year
		hb.writeElementOpen("tr", "class", "statsyear", "data-year", y.Name)
		hb.writeElementOpen("td", "class", "tal")
		hb.writeEscaped(y.Name)
		hb.writeElementClose("td")
		hb.writeElementOpen("td", "class", "tar")
		hb.writeEscaped(y.Posts)
		hb.writeElementClose("td")
		hb.writeElementOpen("td", "class", "tar")
		hb.writeEscaped(y.Chars)
		hb.writeElementClose("td")
		hb.writeElementOpen("td", "class", "tar")
		hb.writeEscaped(y.Words)
		hb.writeElementClose("td")
		hb.writeElementOpen("td", "class", "tar")
		hb.writeEscaped(y.WordsPerPost)
		hb.writeElementClose("td")
		hb.writeElementClose("tr")
		// Iterate over months
		for _, m := range bsd.Months[y.Name] {
			// Stats for month
			hb.writeElementOpen("tr", "class", "statsmonth hide", "data-year", y.Name)
			hb.writeElementOpen("td", "class", "tal")
			hb.writeEscaped(y.Name)
			hb.write("-")
			hb.writeEscaped(m.Name)
			hb.writeElementClose("td")
			hb.writeElementOpen("td", "class", "tar")
			hb.writeEscaped(m.Posts)
			hb.writeElementClose("td")
			hb.writeElementOpen("td", "class", "tar")
			hb.writeEscaped(m.Chars)
			hb.writeElementClose("td")
			hb.writeElementOpen("td", "class", "tar")
			hb.writeEscaped(m.Words)
			hb.writeElementClose("td")
			hb.writeElementOpen("td", "class", "tar")
			hb.writeEscaped(m.WordsPerPost)
			hb.writeElementClose("td")
			hb.writeElementClose("tr")
		}
	}
	// Posts without date
	hb.writeElementOpen("tr")
	hb.writeElementOpen("td", "class", "tal")
	hb.writeEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "withoutdate"))
	hb.writeElementClose("td")
	hb.writeElementOpen("td", "class", "tar")
	hb.writeEscaped(bsd.NoDate.Posts)
	hb.writeElementClose("td")
	hb.writeElementOpen("td", "class", "tar")
	hb.writeEscaped(bsd.NoDate.Chars)
	hb.writeElementClose("td")
	hb.writeElementOpen("td", "class", "tar")
	hb.writeEscaped(bsd.NoDate.Words)
	hb.writeElementClose("td")
	hb.writeElementOpen("td", "class", "tar")
	hb.writeEscaped(bsd.NoDate.WordsPerPost)
	hb.writeElementClose("td")
	hb.writeElementClose("tr")
	// Total
	hb.writeElementOpen("tr")
	hb.writeElementOpen("td", "class", "tal")
	hb.writeElementOpen("strong")
	hb.writeEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "total"))
	hb.writeElementClose("strong")
	hb.writeElementClose("td")
	hb.writeElementOpen("td", "class", "tar")
	hb.writeEscaped(bsd.Total.Posts)
	hb.writeElementClose("td")
	hb.writeElementOpen("td", "class", "tar")
	hb.writeEscaped(bsd.Total.Chars)
	hb.writeElementClose("td")
	hb.writeElementOpen("td", "class", "tar")
	hb.writeEscaped(bsd.Total.Words)
	hb.writeElementClose("td")
	hb.writeElementOpen("td", "class", "tar")
	hb.writeEscaped(bsd.Total.WordsPerPost)
	hb.writeElementClose("td")
	hb.writeElementClose("tr")
	hb.writeElementClose("tbody")
	hb.writeElementClose("table")
}

type geoMapRenderData struct {
	noLocations bool
	locations   string
	tracks      string
	attribution string
	minZoom     int
	maxZoom     int
}

func (a *goBlog) renderGeoMap(hb *htmlBuilder, rd *renderData) {
	gmd, ok := rd.Data.(*geoMapRenderData)
	if !ok {
		return
	}
	a.renderBase(
		hb, rd,
		func(hb *htmlBuilder) {
			a.renderTitleTag(hb, rd.Blog, "")
			if !gmd.noLocations {
				hb.writeElementOpen("link", "rel", "stylesheet", "href", "/-/leaflet/leaflet.css")
				hb.writeElementOpen("script", "src", "/-/leaflet/leaflet.js")
				hb.writeElementClose("script")
			}
		},
		func(hb *htmlBuilder) {
			hb.writeElementOpen("main")
			if gmd.noLocations {
				hb.writeElementOpen("p")
				hb.writeEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "nolocations"))
				hb.writeElementClose("p")
			} else {
				hb.writeElementOpen(
					"div", "id", "map", "class", "p",
					"data-locations", gmd.locations,
					"data-tracks", gmd.tracks,
					"data-minzoom", gmd.minZoom,
					"data-maxzoom", gmd.maxZoom,
					"data-attribution", gmd.attribution,
				)
				hb.writeElementClose("div")
				hb.writeElementOpen("script", "src", a.assetFileName("js/geomap.js"))
				hb.writeElementClose("script")
			}
			hb.writeElementClose("main")
			if rd.CommentsEnabled {
				a.renderInteractions(hb, rd.Blog, rd.Canonical)
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

func (a *goBlog) renderBlogroll(hb *htmlBuilder, rd *renderData) {
	bd, ok := rd.Data.(*blogrollRenderData)
	if !ok {
		return
	}
	renderedTitle := a.renderMdTitle(bd.title)
	a.renderBase(
		hb, rd,
		func(hb *htmlBuilder) {
			a.renderTitleTag(hb, rd.Blog, renderedTitle)
		},
		func(hb *htmlBuilder) {
			hb.writeElementOpen("main")
			// Title
			if renderedTitle != "" {
				hb.writeElementOpen("h1")
				hb.writeEscaped(renderedTitle)
				hb.writeElementClose("h1")
			}
			// Description
			if bd.description != "" {
				hb.writeElementOpen("p")
				_ = a.renderMarkdownToWriter(hb, bd.description, false)
				hb.writeElementClose("p")
			}
			// Download button
			hb.writeElementOpen("p")
			hb.writeElementOpen("a", "href", rd.Blog.getRelativePath(bd.download), "class", "button", "download", "")
			hb.writeEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "download"))
			hb.writeElementClose("a")
			hb.writeElementClose("p")
			// Outlines
			for _, outline := range bd.outlines {
				title := outline.Title
				if title == "" {
					title = outline.Text
				}
				hb.writeElementOpen("h2", "id", urlize(title))
				hb.writeEscaped(fmt.Sprintf("%s (%d)", title, len(outline.Outlines)))
				hb.writeElementClose("h2")
				hb.writeElementOpen("ul")
				for _, subOutline := range outline.Outlines {
					subTitle := subOutline.Title
					if subTitle == "" {
						subTitle = subOutline.Text
					}
					hb.writeElementOpen("li")
					hb.writeElementOpen("a", "href", subOutline.HTMLURL, "target", "_blank")
					hb.writeEscaped(subTitle)
					hb.writeElementClose("a")
					hb.write(" (")
					hb.writeElementOpen("a", "href", subOutline.XMLURL, "target", "_blank")
					hb.writeEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "feed"))
					hb.writeElementClose("a")
					hb.write(")")
					hb.writeElementClose("li")
				}
				hb.writeElementClose("ul")
			}
			hb.writeElementClose("main")
			// Interactions
			if rd.CommentsEnabled {
				a.renderInteractions(hb, rd.Blog, rd.Canonical)
			}
		},
	)
}

type contactRenderData struct {
	title       string
	description string
	privacy     string
	sent        bool
}

func (a *goBlog) renderContact(hb *htmlBuilder, rd *renderData) {
	cd, ok := rd.Data.(*contactRenderData)
	if !ok {
		return
	}
	renderedTitle := a.renderMdTitle(cd.title)
	a.renderBase(
		hb, rd,
		func(hb *htmlBuilder) {
			a.renderTitleTag(hb, rd.Blog, renderedTitle)
		},
		func(hb *htmlBuilder) {
			if cd.sent {
				hb.writeElementOpen("main")
				hb.writeElementOpen("p")
				hb.writeEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "messagesent"))
				hb.writeElementClose("p")
				hb.writeElementClose("main")
				return
			}
			hb.writeElementOpen("main")
			// Title
			if renderedTitle != "" {
				hb.writeElementOpen("h1")
				hb.writeEscaped(renderedTitle)
				hb.writeElementClose("h1")
			}
			// Description
			if cd.description != "" {
				_ = a.renderMarkdownToWriter(hb, cd.description, false)
			}
			// Form
			hb.writeElementOpen("form", "class", "fw p", "method", "post")
			// Name (optional)
			hb.writeElementOpen("input", "type", "text", "name", "name", "placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "nameopt"))
			// Website (optional)
			hb.writeElementOpen("input", "type", "url", "name", "website", "placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "websiteopt"))
			// Email (optional)
			hb.writeElementOpen("input", "type", "email", "name", "email", "placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "emailopt"))
			// Message (required)
			hb.writeElementOpen("textarea", "name", "message", "placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "message"), "required", "")
			hb.writeElementClose("textarea")
			// Send
			if cd.privacy != "" {
				_ = a.renderMarkdownToWriter(hb, cd.privacy, false)
				hb.writeElementOpen("input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "contactagreesend"))
			} else {
				hb.writeElementOpen("input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "contactsend"))
			}
			hb.writeElementClose("form")
			hb.writeElementClose("main")
		},
	)
}

type captchaRenderData struct {
	captchaMethod  string
	captchaHeaders string
	captchaBody    string
	captchaId      string
}

func (a *goBlog) renderCaptcha(hb *htmlBuilder, rd *renderData) {
	crd, ok := rd.Data.(*captchaRenderData)
	if !ok {
		return
	}
	a.renderBase(
		hb, rd,
		func(hb *htmlBuilder) {
			a.renderTitleTag(hb, rd.Blog, "")
		},
		func(hb *htmlBuilder) {
			hb.writeElementOpen("main")
			// Captcha image
			hb.writeElementOpen("p")
			hb.writeElementOpen("img", "src", "/captcha/"+crd.captchaId+".png", "class", "captchaimg")
			hb.writeElementClose("p")
			// Form
			hb.writeElementOpen("form", "class", "fw p", "method", "post")
			// Hidden fields
			hb.writeElementOpen("input", "type", "hidden", "name", "captchaaction", "value", "captcha")
			hb.writeElementOpen("input", "type", "hidden", "name", "captchamethod", "value", crd.captchaMethod)
			hb.writeElementOpen("input", "type", "hidden", "name", "captchaheaders", "value", crd.captchaHeaders)
			hb.writeElementOpen("input", "type", "hidden", "name", "captchabody", "value", crd.captchaBody)
			// Text
			hb.writeElementOpen("input", "type", "text", "name", "digits", "placeholder", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "captchainstructions"), "required", "")
			// Submit
			hb.writeElementOpen("input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "submit"))
			hb.writeElementClose("form")
			hb.writeElementClose("main")
		},
	)
}

type taxonomyRenderData struct {
	taxonomy    *configTaxonomy
	valueGroups []stringGroup
}

func (a *goBlog) renderTaxonomy(hb *htmlBuilder, rd *renderData) {
	trd, ok := rd.Data.(*taxonomyRenderData)
	if !ok {
		return
	}
	renderedTitle := a.renderMdTitle(trd.taxonomy.Title)
	a.renderBase(
		hb, rd,
		func(hb *htmlBuilder) {
			a.renderTitleTag(hb, rd.Blog, renderedTitle)
		},
		func(hb *htmlBuilder) {
			hb.writeElementOpen("main")
			// Title
			if renderedTitle != "" {
				hb.writeElementOpen("h1")
				hb.writeEscaped(renderedTitle)
				hb.writeElementClose("h1")
			}
			// Description
			if trd.taxonomy.Description != "" {
				_ = a.renderMarkdownToWriter(hb, trd.taxonomy.Description, false)
			}
			// List
			for _, valGroup := range trd.valueGroups {
				// Title
				hb.writeElementOpen("h2")
				hb.writeEscaped(valGroup.Identifier)
				hb.writeElementClose("h2")
				// List
				hb.writeElementOpen("p")
				for i, val := range valGroup.Strings {
					if i > 0 {
						hb.write(" &bull; ")
					}
					hb.writeElementOpen("a", "href", fmt.Sprintf("/%s/%s", trd.taxonomy.Name, urlize(val)))
					hb.writeEscaped(val)
					hb.writeElementClose("a")
				}
				hb.writeElementClose("p")
			}
		},
	)
}

func (a *goBlog) renderPost(hb *htmlBuilder, rd *renderData) {
	p, ok := rd.Data.(*post)
	if !ok {
		return
	}
	postHtml := a.postHtml(p, false)
	a.renderBase(
		hb, rd,
		func(hb *htmlBuilder) {
			a.renderTitleTag(hb, rd.Blog, p.RenderedTitle)
			if strings.Contains(string(postHtml), "c-chroma") {
				hb.writeElementOpen("link", "rel", "stylesheet", "href", a.assetFileName("css/chroma.css"))
			}
			a.renderPostHeadMeta(hb, p, rd.Canonical)
			if su := a.shortPostURL(p); su != "" {
				hb.writeElementOpen("link", "rel", "shortlink", "href", su)
			}
			if p.HasTrack() {
				hb.writeElementOpen("link", "rel", "stylesheet", "href", "/-/leaflet/leaflet.css")
				hb.writeElementOpen("script", "src", "/-/leaflet/leaflet.js")
				hb.writeElementClose("script")
			}
		},
		func(hb *htmlBuilder) {
			hb.writeElementOpen("main", "class", "h-entry")
			hb.writeElementOpen("article")
			// URL (hidden just for microformats)
			hb.writeElementOpen("data", "value", a.getFullAddress(p.Path), "class", "u-url hide")
			hb.writeElementClose("data")
			// Title
			if p.RenderedTitle != "" {
				hb.writeElementOpen("h1", "class", "p-name")
				hb.writeEscaped(p.RenderedTitle)
				hb.writeElementClose("h1")
			}
			// Post meta
			a.renderPostMeta(hb, p, rd.Blog, "post")
			// Post actions
			hb.writeElementOpen("div", "id", "post-actions")
			// Share button
			hb.writeElementOpen("a", "class", "button", "href", fmt.Sprintf("https://www.addtoany.com/share#url=%s%s", a.shortPostURL(p), funk.ShortIf(p.RenderedTitle != "", "&title="+p.RenderedTitle, "")), "target", "_blank", "rel", "nofollow noopener noreferrer")
			hb.writeEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "share"))
			hb.writeElementClose("a")
			// Translate button
			hb.writeElementOpen("a", "id", "translateBtn", "class", "button", "href", fmt.Sprintf("https://translate.google.com/translate?u=%s", a.getFullAddress(p.Path)), "target", "_blank", "rel", "nofollow noopener noreferrer")
			hb.writeEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "translate"))
			hb.writeElementClose("a")
			hb.writeElementOpen("script", "defer", "", "src", a.assetFileName("js/translate.js"))
			hb.writeElementClose("script")
			// Speak button
			hb.writeElementOpen("button", "id", "speakBtn", "class", "hide", "data-speak", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "speak"), "data-stopspeak", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "stopspeak"))
			hb.writeElementClose("button")
			hb.writeElementOpen("script", "defer", "", "src", funk.ShortIf(p.TTS() != "", a.assetFileName("js/tts.js"), a.assetFileName("js/speak.js")))
			hb.writeElementClose("script")
			hb.writeElementClose("div")
			// TTS
			if tts := p.TTS(); tts != "" {
				hb.writeElementOpen("div", "class", "p hide", "id", "tts")
				hb.writeElementOpen("audio", "controls", "", "preload", "none", "id", "tts-audio")
				hb.writeElementOpen("source", "src", tts)
				hb.writeElementClose("source")
				hb.writeElementClose("audio")
				hb.writeElementClose("div")
			}
			// Old content warning
			a.renderOldContentWarning(hb, p, rd.Blog)
			// Content
			if p.Content != "" {
				// Content
				hb.writeElementOpen("div", "class", "e-content")
				hb.writeHtml(postHtml)
				hb.writeElementClose("div")
			}
			// GPS Track
			if p.HasTrack() {
				if track, err := a.getTrack(p); err == nil && track != nil && track.HasPoints {
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
						hb.writeEscaped(a.ts.GetTemplateStringVariant(rd.Blog.Lang, "kilometers"))
						hb.write(" ")
					}
					if track.Hours != "" {
						hb.write("⏱ ")
						hb.writeEscaped(track.Hours)
					}
					hb.writeElementClose("p")
					// Map
					hb.writeElementOpen("div", "id", "map", "class", "p", "data-paths", track.PathsJSON, "data-points", track.PointsJSON, "data-minzoom", track.MinZoom, "data-maxzoom", track.MaxZoom, "data-attribution", track.MapAttribution)
					hb.writeElementClose("div")
					hb.writeElementOpen("script", "defer", "", "src", a.assetFileName("js/geotrack.js"))
					hb.writeElementClose("script")
				}
			}
			// Taxonomies
			a.renderPostTax(hb, p, rd.Blog)
			hb.writeElementClose("article")
			// Author
			a.renderAuthor(hb)
			hb.writeElementClose("main")
			// Post edit actions
			if rd.LoggedIn() {
				hb.writeElementOpen("div", "id", "posteditactions")
				// Update
				hb.writeElementOpen("form", "method", "post", "action", rd.Blog.RelativePath("/editor")+"#update")
				hb.writeElementOpen("input", "type", "hidden", "name", "editoraction", "value", "loadupdate")
				hb.writeElementOpen("input", "type", "hidden", "name", "path", "value", p.Path)
				hb.writeElementOpen("input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "update"))
				hb.writeElementClose("form")
				// Delete
				hb.writeElementOpen("form", "method", "post", "action", rd.Blog.RelativePath("/editor"))
				hb.writeElementOpen("input", "type", "hidden", "name", "action", "value", "delete")
				hb.writeElementOpen("input", "type", "hidden", "name", "url", "value", rd.Canonical)
				hb.writeElementOpen("input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "delete"), "class", "confirm", "data-confirmmessage", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "confirmdelete"))
				hb.writeElementClose("form")
				// Undelete
				if p.Deleted() {
					hb.writeElementOpen("form", "method", "post", "action", rd.Blog.RelativePath("/editor"))
					hb.writeElementOpen("input", "type", "hidden", "name", "action", "value", "undelete")
					hb.writeElementOpen("input", "type", "hidden", "name", "url", "value", rd.Canonical)
					hb.writeElementOpen("input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "undelete"))
					hb.writeElementClose("form")
				}
				// TTS
				if a.ttsEnabled() {
					hb.writeElementOpen("form", "method", "post", "action", rd.Blog.RelativePath("/editor"))
					hb.writeElementOpen("input", "type", "hidden", "name", "editoraction", "value", "tts")
					hb.writeElementOpen("input", "type", "hidden", "name", "url", "value", rd.Canonical)
					hb.writeElementOpen("input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "gentts"))
					hb.writeElementClose("form")
				}
				hb.writeElementOpen("script", "defer", "", "src", a.assetFileName("js/formconfirm.js"))
				hb.writeElementClose("script")
				hb.writeElementClose("div")
			}
			// Comments
			if rd.CommentsEnabled {
				a.renderInteractions(hb, rd.Blog, rd.Canonical)
			}
		},
	)
}

func (a *goBlog) renderStaticHome(hb *htmlBuilder, rd *renderData) {
	p, ok := rd.Data.(*post)
	if !ok {
		return
	}
	a.renderBase(
		hb, rd,
		func(hb *htmlBuilder) {
			a.renderTitleTag(hb, rd.Blog, "")
			a.renderPostHeadMeta(hb, p, rd.Canonical)
		},
		func(hb *htmlBuilder) {
			hb.writeElementOpen("main", "class", "h-entry")
			hb.writeElementOpen("article")
			// URL (hidden just for microformats)
			hb.writeElementOpen("data", "value", a.getFullAddress(p.Path), "class", "u-url hide")
			hb.writeElementClose("data")
			// Content
			if p.Content != "" {
				// Content
				hb.writeElementOpen("div", "class", "e-content")
				hb.writeHtml(a.postHtml(p, false))
				hb.writeElementClose("div")
			}
			// Author
			a.renderAuthor(hb)
			hb.writeElementClose("article")
			hb.writeElementClose("main")
			// Update
			if rd.LoggedIn() {
				hb.writeElementOpen("div", "id", "posteditactions")
				hb.writeElementOpen("form", "method", "post", "action", rd.Blog.RelativePath("/editor")+"#update")
				hb.writeElementOpen("input", "type", "hidden", "name", "editoraction", "value", "loadupdate")
				hb.writeElementOpen("input", "type", "hidden", "name", "path", "value", p.Path)
				hb.writeElementOpen("input", "type", "submit", "value", a.ts.GetTemplateStringVariant(rd.Blog.Lang, "update"))
				hb.writeElementClose("form")
				hb.writeElementClose("div")
			}
		},
	)
}
