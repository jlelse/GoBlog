package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/vcraescu/go-paginator"
	"go.goblog.app/app/pkgs/bufferpool"
)

var errPostNotFound = errors.New("post not found")

type post struct {
	Path       string
	Content    string
	Published  string
	Updated    string
	Parameters map[string][]string
	Blog       string
	Section    string
	Status     postStatus
	Priority   int
	// Not persisted
	Slug          string
	RenderedTitle string
}

type postStatus string

const (
	statusDeletedSuffix string = "-deleted"

	statusNil              postStatus = ""
	statusPublished        postStatus = "published"
	statusPublishedDeleted postStatus = "published-deleted"
	statusDraft            postStatus = "draft"
	statusDraftDeleted     postStatus = "draft-deleted"
	statusPrivate          postStatus = "private"
	statusPrivateDeleted   postStatus = "private-deleted"
	statusUnlisted         postStatus = "unlisted"
	statusUnlistedDeleted  postStatus = "unlisted-deleted"
	statusScheduled        postStatus = "scheduled"
	statusScheduledDeleted postStatus = "scheduled-deleted"
)

func (a *goBlog) servePost(w http.ResponseWriter, r *http.Request) {
	p, err := a.getPost(r.URL.Path)
	if errors.Is(err, errPostNotFound) {
		a.serve404(w, r)
		return
	} else if err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	if asRequest, ok := r.Context().Value(asRequestKey).(bool); ok && asRequest {
		if r.URL.Path == a.getRelativePath(p.Blog, "") {
			a.serveActivityStreams(p.Blog, w, r)
			return
		}
		a.serveActivityStreamsPost(p, w)
		return
	}
	canonical := p.firstParameter("original")
	if canonical == "" {
		canonical = a.fullPostURL(p)
	}
	renderMethod := a.renderPost
	if p.Path == a.getRelativePath(p.Blog, "") {
		renderMethod = a.renderStaticHome
	}
	w.Header().Add("Link", fmt.Sprintf("<%s>; rel=shortlink", a.shortPostURL(p)))
	status := http.StatusOK
	if strings.HasSuffix(string(p.Status), statusDeletedSuffix) {
		status = http.StatusGone
	}
	a.renderWithStatusCode(w, r, status, renderMethod, &renderData{
		BlogString: p.Blog,
		Canonical:  canonical,
		Data:       p,
	})
}

const defaultRandomPath = "/random"

func (a *goBlog) redirectToRandomPost(rw http.ResponseWriter, r *http.Request) {
	blog, _ := a.getBlog(r)
	randomPath, err := a.getRandomPostPath(blog)
	if err != nil {
		a.serveError(rw, r, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(rw, r, randomPath, http.StatusFound)
}

const defaultOnThisDayPath = "/onthisday"

func (a *goBlog) redirectToOnThisDay(w http.ResponseWriter, r *http.Request) {
	_, bc := a.getBlog(r)
	// Get current local month and day
	now := time.Now()
	month := now.Month()
	day := now.Day()
	// Build the path
	targetPath := fmt.Sprintf("/x/%02d/%02d", month, day)
	targetPath = bc.getRelativePath(targetPath)
	// Redirect
	http.Redirect(w, r, targetPath, http.StatusFound)
}

type postPaginationAdapter struct {
	config *postsRequestConfig
	nums   int64
	a      *goBlog
}

func (p *postPaginationAdapter) Nums() (int64, error) {
	if p.nums == 0 {
		nums, _ := p.a.db.countPosts(p.config)
		p.nums = int64(nums)
	}
	return p.nums, nil
}

func (p *postPaginationAdapter) Slice(offset, length int, data interface{}) error {
	modifiedConfig := *p.config
	modifiedConfig.offset = offset
	modifiedConfig.limit = length

	posts, err := p.a.getPosts(&modifiedConfig)
	reflect.ValueOf(data).Elem().Set(reflect.ValueOf(&posts).Elem())
	return err
}

func (a *goBlog) serveHome(w http.ResponseWriter, r *http.Request) {
	blog, _ := a.getBlog(r)
	if asRequest, ok := r.Context().Value(asRequestKey).(bool); ok && asRequest {
		a.serveActivityStreams(blog, w, r)
		return
	}
	a.serveIndex(w, r.WithContext(context.WithValue(r.Context(), indexConfigKey, &indexConfig{
		path: a.getRelativePath(blog, ""),
	})))
}

func (a *goBlog) serveDrafts(w http.ResponseWriter, r *http.Request) {
	_, bc := a.getBlog(r)
	a.serveIndex(w, r.WithContext(context.WithValue(r.Context(), indexConfigKey, &indexConfig{
		path:        bc.getRelativePath("/editor/drafts"),
		title:       a.ts.GetTemplateStringVariant(bc.Lang, "drafts"),
		description: a.ts.GetTemplateStringVariant(bc.Lang, "draftsdesc"),
		status:      statusDraft,
	})))
}

func (a *goBlog) servePrivate(w http.ResponseWriter, r *http.Request) {
	_, bc := a.getBlog(r)
	a.serveIndex(w, r.WithContext(context.WithValue(r.Context(), indexConfigKey, &indexConfig{
		path:        bc.getRelativePath("/editor/private"),
		title:       a.ts.GetTemplateStringVariant(bc.Lang, "privateposts"),
		description: a.ts.GetTemplateStringVariant(bc.Lang, "privatepostsdesc"),
		status:      statusPrivate,
	})))
}

func (a *goBlog) serveUnlisted(w http.ResponseWriter, r *http.Request) {
	_, bc := a.getBlog(r)
	a.serveIndex(w, r.WithContext(context.WithValue(r.Context(), indexConfigKey, &indexConfig{
		path:        bc.getRelativePath("/editor/unlisted"),
		title:       a.ts.GetTemplateStringVariant(bc.Lang, "unlistedposts"),
		description: a.ts.GetTemplateStringVariant(bc.Lang, "unlistedpostsdesc"),
		status:      statusUnlisted,
	})))
}

func (a *goBlog) serveScheduled(w http.ResponseWriter, r *http.Request) {
	_, bc := a.getBlog(r)
	a.serveIndex(w, r.WithContext(context.WithValue(r.Context(), indexConfigKey, &indexConfig{
		path:        bc.getRelativePath("/editor/scheduled"),
		title:       a.ts.GetTemplateStringVariant(bc.Lang, "scheduledposts"),
		description: a.ts.GetTemplateStringVariant(bc.Lang, "scheduledpostsdesc"),
		status:      statusScheduled,
	})))
}

func (a *goBlog) serveDeleted(w http.ResponseWriter, r *http.Request) {
	_, bc := a.getBlog(r)
	a.serveIndex(w, r.WithContext(context.WithValue(r.Context(), indexConfigKey, &indexConfig{
		path:        bc.getRelativePath("/editor/deleted"),
		title:       a.ts.GetTemplateStringVariant(bc.Lang, "deletedposts"),
		description: a.ts.GetTemplateStringVariant(bc.Lang, "deletedpostsdesc"),
		statusse:    []postStatus{statusPublishedDeleted, statusDraftDeleted, statusScheduledDeleted, statusPrivateDeleted, statusUnlistedDeleted},
	})))
}

func (a *goBlog) serveDate(w http.ResponseWriter, r *http.Request) {
	var year, month, day int
	if ys := chi.URLParam(r, "year"); ys != "" && ys != "x" {
		year = stringToInt(ys)
	}
	if ms := chi.URLParam(r, "month"); ms != "" && ms != "x" {
		month = stringToInt(ms)
	}
	if ds := chi.URLParam(r, "day"); ds != "" {
		day = stringToInt(ds)
	}
	if year == 0 && month == 0 && day == 0 {
		a.serve404(w, r)
		return
	}
	title, dPath := bufferpool.Get(), bufferpool.Get()
	if year != 0 {
		_, _ = fmt.Fprintf(title, "%0004d", year)
		_, _ = fmt.Fprintf(dPath, "%0004d", year)
	} else {
		_, _ = title.WriteString("XXXX")
		_, _ = dPath.WriteString("x")
	}
	if month != 0 {
		_, _ = fmt.Fprintf(title, "-%02d", month)
		_, _ = fmt.Fprintf(dPath, "/%02d", month)
	} else if day != 0 {
		_, _ = title.WriteString("-XX")
		_, _ = dPath.WriteString("/x")
	}
	if day != 0 {
		_, _ = fmt.Fprintf(title, "-%02d", day)
		_, _ = fmt.Fprintf(dPath, "/%02d", day)
	}
	_, bc := a.getBlog(r)
	a.serveIndex(w, r.WithContext(context.WithValue(r.Context(), indexConfigKey, &indexConfig{
		path:  bc.getRelativePath(dPath.String()),
		year:  year,
		month: month,
		day:   day,
		title: title.String(),
	})))
	bufferpool.Put(title, dPath)
}

type indexConfig struct {
	path             string
	section          *configSection
	tax              *configTaxonomy
	taxValue         string
	parameter        string
	year, month, day int
	title            string
	description      string
	summaryTemplate  summaryTyp
	status           postStatus
	statusse         []postStatus
}

const defaultPhotosPath = "/photos"

const indexConfigKey contextKey = "indexConfig"

func (a *goBlog) serveIndex(w http.ResponseWriter, r *http.Request) {
	ic := r.Context().Value(indexConfigKey).(*indexConfig)
	blog, bc := a.getBlog(r)
	search := chi.URLParam(r, "search")
	if search != "" {
		// Decode and sanitize search
		search = cleanHTMLText(searchDecode(search))
	}
	var sections []string
	if ic.section != nil {
		sections = []string{ic.section.Name}
	} else {
		for sectionKey := range bc.Sections {
			sections = append(sections, sectionKey)
		}
	}
	statusse := ic.statusse
	if ic.status != statusNil {
		statusse = []postStatus{ic.status}
	}
	if len(statusse) == 0 {
		statusse = []postStatus{statusPublished}
	}
	p := paginator.New(&postPaginationAdapter{config: &postsRequestConfig{
		blog:           blog,
		sections:       sections,
		taxonomy:       ic.tax,
		taxonomyValue:  ic.taxValue,
		parameter:      ic.parameter,
		search:         search,
		publishedYear:  ic.year,
		publishedMonth: ic.month,
		publishedDay:   ic.day,
		statusse:       statusse,
		priorityOrder:  true,
	}, a: a}, bc.Pagination)
	p.SetPage(stringToInt(chi.URLParam(r, "page")))
	var posts []*post
	err := p.Results(&posts)
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	// Meta
	title := ic.title
	description := ic.description
	if ic.tax != nil {
		title = fmt.Sprintf("%s: %s", ic.tax.Title, ic.taxValue)
	} else if ic.section != nil {
		title = ic.section.Title
		description = ic.section.Description
	} else if search != "" {
		title = fmt.Sprintf("%s: %s", bc.Search.Title, search)
	}
	// Check if feed
	if ft := feedType(chi.URLParam(r, "feed")); ft != noFeed {
		a.generateFeed(blog, ft, w, r, posts, title, description)
		return
	}
	// Path
	path := ic.path
	if strings.Contains(path, searchPlaceholder) {
		path = strings.ReplaceAll(path, searchPlaceholder, searchEncode(search))
	}
	// Navigation
	var hasPrev, hasNext bool
	var prevPage, nextPage int
	var prevPath, nextPath string
	hasPrev, _ = p.HasPrev()
	if hasPrev {
		prevPage, _ = p.PrevPage()
	} else {
		prevPage, _ = p.Page()
	}
	if prevPage < 2 {
		prevPath = path
	} else {
		prevPath = fmt.Sprintf("%s/page/%d", strings.TrimSuffix(path, "/"), prevPage)
	}
	hasNext, _ = p.HasNext()
	if hasNext {
		nextPage, _ = p.NextPage()
	} else {
		nextPage, _ = p.Page()
	}
	nextPath = fmt.Sprintf("%s/page/%d", strings.TrimSuffix(path, "/"), nextPage)
	summaryTemplate := ic.summaryTemplate
	if summaryTemplate == "" {
		summaryTemplate = defaultSummary
	}
	a.render(w, r, a.renderIndex, &renderData{
		Canonical: a.getFullAddress(path),
		Data: &indexRenderData{
			title:           title,
			description:     description,
			posts:           posts,
			hasPrev:         hasPrev,
			hasNext:         hasNext,
			first:           path,
			prev:            prevPath,
			next:            nextPath,
			summaryTemplate: summaryTemplate,
		},
	})
}
