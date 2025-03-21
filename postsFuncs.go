package main

import (
	"fmt"
	"io"
	"strings"
	"time"

	gogeouri "git.jlel.se/jlelse/go-geouri"
	"github.com/PuerkitoBio/goquery"
	"github.com/araddon/dateparse"
	"go.goblog.app/app/pkgs/bufferpool"
	"go.goblog.app/app/pkgs/htmlbuilder"
	"go.goblog.app/app/pkgs/plugintypes"
	"gopkg.in/yaml.v3"
)

func (a *goBlog) fullPostURL(p *post) string {
	return a.getFullAddress(p.Path)
}

func (a *goBlog) shortPostURL(p *post) string {
	s, err := a.db.shortenPath(p.Path)
	if err != nil || s == "" {
		return a.getFullAddress(p.Path)
	}
	if a.cfg.Server.ShortPublicAddress != "" {
		return a.cfg.Server.ShortPublicAddress + s
	}
	return a.getFullAddress(s)
}

func (p *post) firstParameter(parameter string) (result string) {
	if pp := p.Parameters[parameter]; len(pp) > 0 {
		result = pp[0]
	}
	return
}

func (p *post) addParameter(parameter, value string) {
	p.Parameters[parameter] = append(p.Parameters[parameter], value)
}

type postHtmlOptions struct {
	p           *post
	absolute    bool
	activityPub bool
}

func (a *goBlog) postHtml(o *postHtmlOptions) (res string) {
	buf := bufferpool.Get()
	a.postHtmlToWriter(buf, o)
	res = buf.String()
	bufferpool.Put(buf)
	return
}

func (a *goBlog) postHtmlToWriter(w io.Writer, o *postHtmlOptions) {
	// Build HTML
	hb, finish := a.wrapForPlugins(w, a.getPlugins(pluginUiPostContentType), func(plugin any, doc *goquery.Document) {
		plugin.(plugintypes.UIPostContent).RenderPostContent(o.p, doc)
	}, selectorBodyInner)
	defer finish()
	// Add audio to the top
	for _, a := range o.p.Parameters[a.cfg.Micropub.AudioParam] {
		hb.WriteElementOpen("audio", "controls", "preload", "none")
		hb.WriteElementOpen("source", "src", a)
		hb.WriteElementClose("source")
		hb.WriteElementClose("audio")
	}
	// Add IndieWeb context
	if !o.activityPub || o.p.firstParameter(activityPubReplyActorParameter) == "" {
		a.renderPostReplyContext(hb, o.p)
	}
	a.renderPostLikeContext(hb, o.p)
	// Render markdown
	hb.WriteElementOpen("div", "class", "e-content")
	_ = a.renderMarkdownToWriter(hb, o.p.Content, o.absolute)
	hb.WriteElementClose("div")
	// Add bookmark links to the bottom
	for _, l := range o.p.Parameters[a.cfg.Micropub.BookmarkParam] {
		hb.WriteElementOpen("p")
		hb.WriteElementOpen("a", "class", "u-bookmark-of", "href", l, "target", "_blank", "rel", "noopener noreferrer")
		hb.WriteEscaped(l)
		hb.WriteElementClose("a")
		hb.WriteElementClose("p")
	}
}

func (a *goBlog) feedHtml(w io.Writer, p *post) {
	hb := htmlbuilder.NewHtmlBuilder(w)
	// Add TTS audio to the top
	for _, a := range p.Parameters[ttsParameter] {
		hb.WriteElementOpen("audio", "controls", "preload", "none")
		hb.WriteElementOpen("source", "src", a)
		hb.WriteElementClose("source")
		hb.WriteElementClose("audio")
	}
	// Add post HTML
	a.postHtmlToWriter(hb, &postHtmlOptions{p: p, absolute: true})
	// Add link to interactions and comments
	blogConfig := a.getBlogFromPost(p)
	if cc := blogConfig.Comments; cc != nil && cc.Enabled {
		hb.WriteElementOpen("p")
		hb.WriteElementOpen("a", "href", a.getFullAddress(p.Path)+"#interactions")
		hb.WriteEscaped(a.ts.GetTemplateStringVariant(blogConfig.Lang, "interactions"))
		hb.WriteElementClose("a")
		hb.WriteElementClose("p")
	}
}

func (a *goBlog) minFeedHtml(w io.Writer, p *post) {
	hb := htmlbuilder.NewHtmlBuilder(w)
	// Add post HTML
	a.postHtmlToWriter(hb, &postHtmlOptions{p: p, absolute: true})
}

const summaryDivider = "<!--more-->"

func (a *goBlog) postSummary(p *post) (summary string) {
	summary = p.firstParameter("summary")
	if summary != "" {
		return
	}
	splitted := strings.Split(p.Content, summaryDivider)
	hasDivider := len(splitted) > 1
	markdown := splitted[0]
	summary = a.renderTextSafe(markdown)
	if !hasDivider {
		summary = strings.Split(summary, "\n\n")[0]
	}
	summary = strings.TrimSpace(strings.ReplaceAll(summary, "\n\n", " "))
	return
}

func (a *goBlog) fallbackTitle(p *post) string {
	return truncateStringWithEllipsis(a.postSummary(p), 30)
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
	return p.Section != "" && p.Status == statusPublished
}

func (p *post) isPublicPublishedSectionPost() bool {
	return p.isPublishedSectionPost() && p.Visibility == visibilityPublic
}

func (a *goBlog) postToMfMap(p *post) map[string]any {
	return map[string]any{
		"type":       []string{"h-entry"},
		"properties": a.postMfProperties(p, true),
	}
}

func (a *goBlog) postMfProperties(p *post, contentWithParams bool) map[string][]any {
	var mfStatus, mfVisibility string

	switch p.Status {
	case statusDraft:
		mfStatus = "draft"
	case statusPublished, statusScheduled:
		mfStatus = "published"
	case statusPublishedDeleted, statusDraftDeleted, statusScheduledDeleted:
		mfStatus = "deleted"
	}

	switch p.Visibility {
	case visibilityPublic:
		mfVisibility = "public"
	case visibilityUnlisted:
		mfVisibility = "unlisted"
	case visibilityPrivate:
		mfVisibility = "private"
	}

	properties := map[string][]any{}
	addIfNotEmpty := func(key string, value []string) {
		if len(value) > 0 {
			properties[key] = []any{}
			for _, val := range value {
				properties[key] = append(properties[key], val)
			}
		}
	}
	addIfNotEmpty("name", p.Parameters["title"])
	addIfNotEmpty("published", []string{p.Published})
	addIfNotEmpty("updated", []string{p.Updated})
	addIfNotEmpty("post-status", []string{mfStatus})
	addIfNotEmpty("visibility", []string{mfVisibility})
	addIfNotEmpty("category", p.Parameters[a.cfg.Micropub.CategoryParam])
	if contentWithParams {
		addIfNotEmpty("content", []string{p.contentWithParams()})
	} else {
		addIfNotEmpty("content", []string{p.Content})
	}
	addIfNotEmpty("url", []string{a.fullPostURL(p)})
	addIfNotEmpty("in-reply-to", p.Parameters[a.cfg.Micropub.ReplyParam])
	addIfNotEmpty("like-of", p.Parameters[a.cfg.Micropub.LikeParam])
	addIfNotEmpty("bookmark-of", p.Parameters[a.cfg.Micropub.BookmarkParam])
	addIfNotEmpty("mp-slug", []string{p.Slug})
	addIfNotEmpty("audio", p.Parameters[a.cfg.Micropub.AudioParam])
	addIfNotEmpty("mp-channel", []string{p.getChannel()})
	addIfNotEmpty("location", p.Parameters[a.cfg.Micropub.LocationParam])

	return properties
}

func (a *goBlog) showFull(p *post) bool {
	if p.Section == "" {
		return false
	}
	sec, ok := a.getBlogFromPost(p).Sections[p.Section]
	return ok && sec != nil && sec.ShowFull
}

func (a *goBlog) geoURIs(p *post) []*gogeouri.Geo {
	res := []*gogeouri.Geo{}
	for _, loc := range p.Parameters[a.cfg.Micropub.LocationParam] {
		if loc == "" {
			continue
		}
		g, _ := gogeouri.Parse(loc)
		if g != nil {
			res = append(res, g)
		}
	}
	return res
}

func (a *goBlog) replyLink(p *post) string {
	return p.firstParameter(a.cfg.Micropub.ReplyParam)
}

func (a *goBlog) replyTitle(p *post) string {
	return p.firstParameter(a.cfg.Micropub.ReplyTitleParam)
}

func (a *goBlog) replyContext(p *post) string {
	return p.firstParameter(a.cfg.Micropub.ReplyContextParam)
}

func (a *goBlog) likeLink(p *post) string {
	return p.firstParameter(a.cfg.Micropub.LikeParam)
}

func (a *goBlog) likeTitle(p *post) string {
	return p.firstParameter(a.cfg.Micropub.LikeTitleParam)
}

func (a *goBlog) likeContext(p *post) string {
	return p.firstParameter(a.cfg.Micropub.LikeContextParam)
}

func (a *goBlog) photoLinks(p *post) []string {
	return p.Parameters[a.cfg.Micropub.PhotoParam]
}

func (p *post) contentWithParams() string {
	params := map[string]any{}
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
	params["visibility"] = string(p.Visibility)
	params["priority"] = p.Priority
	pb, _ := yaml.Marshal(params)
	return fmt.Sprintf("---\n%s---\n%s", string(pb), p.Content)
}

func (p *post) setChannel(channel string) {
	if channel == "" {
		return
	}
	channelParts := strings.SplitN(channel, "/", 2)
	p.Blog = channelParts[0]
	if len(channelParts) > 1 {
		p.Section = channelParts[1]
	}
}

func (p *post) getChannel() string {
	if p.Section == "" {
		return p.Blog
	}
	return p.Blog + "/" + p.Section
}

func (a *goBlog) addTitleAndContext(p *post, linkParam, titleParam, contextParam string, addTitleFlag, addContextFlag bool) {
	if link := p.firstParameter(linkParam); link != "" {
		addTitle := p.firstParameter(titleParam) == "" && addTitleFlag
		addContext := p.firstParameter(contextParam) == "" && addContextFlag
		if !addTitle && !addContext {
			return
		}
		if mf, err := a.parseMicroformats(link, true); err == nil {
			if addTitle && mf.Title != "" {
				p.addParameter(titleParam, mf.Title)
			}
			if addContext && mf.Content != "" {
				p.addParameter(contextParam, mf.Content)
			}
		}
	}
}

func (a *goBlog) addReplyTitleAndContext(p *post) {
	bc := a.getBlogFromPost(p)
	a.addTitleAndContext(p, a.cfg.Micropub.ReplyParam, a.cfg.Micropub.ReplyTitleParam, a.cfg.Micropub.ReplyContextParam, bc.addReplyTitle, bc.addReplyContext)
}

func (a *goBlog) addLikeTitleAndContext(p *post) {
	bc := a.getBlogFromPost(p)
	a.addTitleAndContext(p, a.cfg.Micropub.LikeParam, a.cfg.Micropub.LikeTitleParam, a.cfg.Micropub.LikeContextParam, bc.addLikeTitle, bc.addLikeContext)
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

func (p *post) TTS() string {
	return p.firstParameter(ttsParameter)
}

func (p *post) Deleted() bool {
	return strings.HasSuffix(string(p.Status), string(statusDeletedSuffix))
}
