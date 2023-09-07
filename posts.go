package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"reflect"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/samber/lo"
	"github.com/vcraescu/go-paginator/v2"
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
	Visibility postVisibility
	Priority   int
	// Not persisted
	Slug          string
	RenderedTitle string
}

type postStatus string
type postVisibility string

const (
	statusDeletedSuffix postStatus = "-deleted"

	statusNil              postStatus = ""
	statusPublished        postStatus = "published"
	statusPublishedDeleted            = statusPublished + statusDeletedSuffix
	statusDraft            postStatus = "draft"
	statusDraftDeleted                = statusDraft + statusDeletedSuffix
	statusScheduled        postStatus = "scheduled"
	statusScheduledDeleted            = statusScheduled + statusDeletedSuffix

	visibilityNil      postVisibility = ""
	visibilityPublic   postVisibility = "public"
	visibilityUnlisted postVisibility = "unlisted"
	visibilityPrivate  postVisibility = "private"
)

func validPostStatus(s postStatus) bool {
	return s == statusPublished || s == statusPublishedDeleted ||
		s == statusDraft || s == statusDraftDeleted ||
		s == statusScheduled || s == statusScheduledDeleted
}

func validPostVisibility(v postVisibility) bool {
	return v == visibilityPublic || v == visibilityUnlisted || v == visibilityPrivate
}

func (a *goBlog) servePost(w http.ResponseWriter, r *http.Request) {
	p, err := a.getPost(r.URL.Path)
	if errors.Is(err, errPostNotFound) {
		a.serve404(w, r)
		return
	} else if err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	status := http.StatusOK
	if p.Deleted() {
		status = http.StatusGone
	}
	if asRequest, ok := r.Context().Value(asRequestKey).(bool); ok && asRequest {
		if r.URL.Path == a.getRelativePath(p.Blog, "") {
			a.serveActivityStreams(w, r, status, p.Blog)
			return
		}
		a.serveActivityStreamsPost(w, r, status, p)
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
	if p.Visibility != visibilityPublic {
		w.Header().Set("X-Robots-Tag", "noindex")
	}
	w.Header().Add("Link", fmt.Sprintf("<%s>; rel=shortlink", a.shortPostURL(p)))
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
		p.nums = int64(noError(p.a.db.countPosts(p.config)))
	}
	return p.nums, nil
}

func (p *postPaginationAdapter) Slice(offset, length int, data any) error {
	modifiedConfig := *p.config
	modifiedConfig.offset = offset
	modifiedConfig.limit = length

	posts, err := p.a.getPosts(&modifiedConfig)
	reflect.ValueOf(data).Elem().Set(reflect.ValueOf(&posts).Elem())
	return err
}

func (a *goBlog) serveHome(w http.ResponseWriter, r *http.Request) {
	blog, bc := a.getBlog(r)
	if asRequest, ok := r.Context().Value(asRequestKey).(bool); ok && asRequest {
		a.serveActivityStreams(w, r, http.StatusOK, blog)
		return
	}
	a.serveIndex(w, r.WithContext(context.WithValue(r.Context(), indexConfigKey, &indexConfig{
		path:     a.getRelativePath(blog, ""),
		sections: lo.Filter(lo.Values(bc.Sections), func(s *configSection, _ int) bool { return !s.HideOnStart }),
	})))
}

func (a *goBlog) serveDrafts(w http.ResponseWriter, r *http.Request) {
	_, bc := a.getBlog(r)
	a.serveIndex(w, r.WithContext(context.WithValue(r.Context(), indexConfigKey, &indexConfig{
		path:        bc.getRelativePath("/editor/drafts"),
		title:       a.ts.GetTemplateStringVariant(bc.Lang, "drafts"),
		description: a.ts.GetTemplateStringVariant(bc.Lang, "draftsdesc"),
		status:      []postStatus{statusDraft},
	})))
}

func (a *goBlog) servePrivate(w http.ResponseWriter, r *http.Request) {
	_, bc := a.getBlog(r)
	a.serveIndex(w, r.WithContext(context.WithValue(r.Context(), indexConfigKey, &indexConfig{
		path:        bc.getRelativePath("/editor/private"),
		title:       a.ts.GetTemplateStringVariant(bc.Lang, "privateposts"),
		description: a.ts.GetTemplateStringVariant(bc.Lang, "privatepostsdesc"),
		status:      []postStatus{statusPublished},
		visibility:  []postVisibility{visibilityPrivate},
	})))
}

func (a *goBlog) serveUnlisted(w http.ResponseWriter, r *http.Request) {
	_, bc := a.getBlog(r)
	a.serveIndex(w, r.WithContext(context.WithValue(r.Context(), indexConfigKey, &indexConfig{
		path:        bc.getRelativePath("/editor/unlisted"),
		title:       a.ts.GetTemplateStringVariant(bc.Lang, "unlistedposts"),
		description: a.ts.GetTemplateStringVariant(bc.Lang, "unlistedpostsdesc"),
		status:      []postStatus{statusPublished},
		visibility:  []postVisibility{visibilityUnlisted},
	})))
}

func (a *goBlog) serveScheduled(w http.ResponseWriter, r *http.Request) {
	_, bc := a.getBlog(r)
	a.serveIndex(w, r.WithContext(context.WithValue(r.Context(), indexConfigKey, &indexConfig{
		path:        bc.getRelativePath("/editor/scheduled"),
		title:       a.ts.GetTemplateStringVariant(bc.Lang, "scheduledposts"),
		description: a.ts.GetTemplateStringVariant(bc.Lang, "scheduledpostsdesc"),
		status:      []postStatus{statusScheduled},
	})))
}

func (a *goBlog) serveDeleted(w http.ResponseWriter, r *http.Request) {
	_, bc := a.getBlog(r)
	a.serveIndex(w, r.WithContext(context.WithValue(r.Context(), indexConfigKey, &indexConfig{
		path:        bc.getRelativePath("/editor/deleted"),
		title:       a.ts.GetTemplateStringVariant(bc.Lang, "deletedposts"),
		description: a.ts.GetTemplateStringVariant(bc.Lang, "deletedpostsdesc"),
		status:      []postStatus{statusPublishedDeleted, statusDraftDeleted, statusScheduledDeleted},
	})))
}

func (a *goBlog) serveDate(w http.ResponseWriter, r *http.Request) {
	year, month, day, title, datePath := a.extractDate(r)
	if year == 0 && month == 0 && day == 0 {
		a.serve404(w, r)
		return
	}
	var ic *indexConfig
	if cv := r.Context().Value(indexConfigKey); cv != nil {
		origIc := *(cv.(*indexConfig))
		copyIc := origIc
		ic = &copyIc
		ic.path = path.Join(ic.path, datePath)
		ic.titleSuffix = ": " + title
	} else {
		_, bc := a.getBlog(r)
		ic = &indexConfig{
			path:  bc.getRelativePath(datePath),
			title: title,
		}
	}
	ic.year, ic.month, ic.day = year, month, day
	a.serveIndex(w, r.WithContext(context.WithValue(r.Context(), indexConfigKey, ic)))
}

func (a *goBlog) extractDate(r *http.Request) (year, month, day int, title, datePath string) {
	if ys := chi.URLParam(r, "year"); ys != "" && ys != "x" {
		year = stringToInt(ys)
	}
	if ms := chi.URLParam(r, "month"); ms != "" && ms != "x" {
		month = stringToInt(ms)
	}
	if ds := chi.URLParam(r, "day"); ds != "" {
		day = stringToInt(ds)
	}
	titleBuf, pathBuf := bufferpool.Get(), bufferpool.Get()
	defer bufferpool.Put(titleBuf, pathBuf)
	if year != 0 {
		_, _ = fmt.Fprintf(titleBuf, "%0004d", year)
		_, _ = fmt.Fprintf(pathBuf, "%0004d", year)
	} else {
		_, _ = titleBuf.WriteString("XXXX")
		_, _ = pathBuf.WriteString("x")
	}
	if month != 0 {
		_, _ = fmt.Fprintf(titleBuf, "-%02d", month)
		_, _ = fmt.Fprintf(pathBuf, "/%02d", month)
	} else if day != 0 {
		_, _ = titleBuf.WriteString("-XX")
		_, _ = pathBuf.WriteString("/x")
	}
	if day != 0 {
		_, _ = fmt.Fprintf(titleBuf, "-%02d", day)
		_, _ = fmt.Fprintf(pathBuf, "/%02d", day)
	}
	title = titleBuf.String()
	datePath = pathBuf.String()
	return
}

type indexConfig struct {
	path             string
	section          *configSection
	sections         []*configSection
	tax              *configTaxonomy
	taxValue         string
	parameter        string
	year, month, day int
	title            string
	titleSuffix      string
	description      string
	summaryTemplate  summaryTyp
	status           []postStatus
	visibility       []postVisibility
	search           string
}

const defaultPhotosPath = "/photos"

const indexConfigKey contextKey = "indexConfig"

func (a *goBlog) serveIndex(w http.ResponseWriter, r *http.Request) {
	ic := r.Context().Value(indexConfigKey).(*indexConfig)
	blog, bc := a.getBlog(r)
	sections := lo.Map(ic.sections, func(i *configSection, _ int) string { return i.Name })
	if ic.section != nil {
		sections = append(sections, ic.section.Name)
	}
	defaultStatus, defaultVisibility := a.getDefaultPostStates(r)
	status := ic.status
	if len(status) == 0 {
		status = defaultStatus
	}
	visibility := ic.visibility
	if len(visibility) == 0 {
		visibility = defaultVisibility
	}
	// Parameter filter
	params, paramValues := []string{}, []string{}
	paramUrlValues := url.Values{}
	for param, values := range r.URL.Query() {
		if strings.HasPrefix(param, "p:") {
			paramKey := strings.TrimPrefix(param, "p:")
			for _, value := range values {
				params, paramValues = append(params, paramKey), append(paramValues, value)
				paramUrlValues.Add(param, value)
			}
		}
	}
	paramUrlQuery := ""
	if len(paramUrlValues) > 0 {
		paramUrlQuery += "?" + paramUrlValues.Encode()
	}
	// Create paginator
	p := paginator.New(&postPaginationAdapter{config: &postsRequestConfig{
		blog:           blog,
		sections:       sections,
		taxonomy:       ic.tax,
		taxonomyValue:  ic.taxValue,
		parameter:      ic.parameter,
		allParams:      params,
		allParamValues: paramValues,
		search:         ic.search,
		publishedYear:  ic.year,
		publishedMonth: ic.month,
		publishedDay:   ic.day,
		status:         status,
		visibility:     visibility,
		priorityOrder:  true,
	}, a: a}, bc.Pagination)
	p.SetPage(stringToInt(chi.URLParam(r, "page")))
	var posts []*post
	err := p.Results(&posts)
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	// Title
	var title string
	if ic.title != "" {
		title = ic.title
	} else if ic.section != nil {
		title = ic.section.Title
	} else if ic.tax != nil {
		title = fmt.Sprintf("%s: %s", ic.tax.Title, ic.taxValue)
	} else if ic.search != "" {
		title = fmt.Sprintf("%s: %s", bc.Search.Title, ic.search)
	}
	title += ic.titleSuffix
	// Description
	var description string
	if ic.description != "" {
		description = ic.description
	} else if ic.section != nil {
		description = ic.section.Description
	}
	// Check if feed
	if ft := feedType(chi.URLParam(r, "feed")); ft != noFeed {
		a.generateFeed(blog, ft, w, r, posts, title, description, ic.path, paramUrlQuery)
		return
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
		prevPath = ic.path
	} else {
		prevPath = fmt.Sprintf("%s/page/%d", strings.TrimSuffix(ic.path, "/"), prevPage)
	}
	hasNext, _ = p.HasNext()
	if hasNext {
		nextPage, _ = p.NextPage()
	} else {
		nextPage, _ = p.Page()
	}
	nextPath = fmt.Sprintf("%s/page/%d", strings.TrimSuffix(ic.path, "/"), nextPage)
	summaryTemplate := ic.summaryTemplate
	if summaryTemplate == "" {
		summaryTemplate = defaultSummary
	}
	a.render(w, r, a.renderIndex, &renderData{
		Canonical: a.getFullAddress(ic.path) + paramUrlQuery,
		Data: &indexRenderData{
			title:           title,
			description:     description,
			posts:           posts,
			hasPrev:         hasPrev,
			hasNext:         hasNext,
			first:           ic.path,
			prev:            prevPath,
			next:            nextPath,
			summaryTemplate: summaryTemplate,
			paramUrlQuery:   paramUrlQuery,
		},
	})
}
