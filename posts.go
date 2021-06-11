package main

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/microcosm-cc/bluemonday"
	servertiming "github.com/mitchellh/go-server-timing"
	"github.com/vcraescu/go-paginator"
)

var errPostNotFound = errors.New("post not found")

type post struct {
	Path       string              `json:"path"`
	Content    string              `json:"content"`
	Published  string              `json:"published"`
	Updated    string              `json:"updated"`
	Parameters map[string][]string `json:"parameters"`
	Blog       string              `json:"blog"`
	Section    string              `json:"section"`
	Status     postStatus          `json:"status"`
	// Not persisted
	Slug             string `json:"slug"`
	rendered         template.HTML
	absoluteRendered template.HTML
}

type postStatus string

const (
	statusNil       postStatus = ""
	statusPublished postStatus = "published"
	statusDraft     postStatus = "draft"
)

func (a *goBlog) servePost(w http.ResponseWriter, r *http.Request) {
	t := servertiming.FromContext(r.Context()).NewMetric("gp").Start()
	p, err := a.db.getPost(r.URL.Path)
	t.Stop()
	if err == errPostNotFound {
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
	template := templatePost
	if p.Path == a.getRelativePath(p.Blog, "") {
		template = templateStaticHome
	}
	w.Header().Add("Link", fmt.Sprintf("<%s>; rel=shortlink", a.shortPostURL(p)))
	a.render(w, r, template, &renderData{
		BlogString: p.Blog,
		Canonical:  canonical,
		Data:       p,
	})
}

func (a *goBlog) redirectToRandomPost(rw http.ResponseWriter, r *http.Request) {
	randomPath, err := a.getRandomPostPath(r.Context().Value(blogContextKey).(string))
	if err != nil {
		a.serveError(rw, r, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(rw, r, randomPath, http.StatusFound)
}

type postPaginationAdapter struct {
	config *postsRequestConfig
	nums   int64
	db     *database
}

func (p *postPaginationAdapter) Nums() (int64, error) {
	if p.nums == 0 {
		nums, _ := p.db.countPosts(p.config)
		p.nums = int64(nums)
	}
	return p.nums, nil
}

func (p *postPaginationAdapter) Slice(offset, length int, data interface{}) error {
	modifiedConfig := *p.config
	modifiedConfig.offset = offset
	modifiedConfig.limit = length

	posts, err := p.db.getPosts(&modifiedConfig)
	reflect.ValueOf(data).Elem().Set(reflect.ValueOf(&posts).Elem())
	return err
}

func (a *goBlog) serveHome(w http.ResponseWriter, r *http.Request) {
	blog := r.Context().Value(blogContextKey).(string)
	if asRequest, ok := r.Context().Value(asRequestKey).(bool); ok && asRequest {
		a.serveActivityStreams(blog, w, r)
		return
	}
	a.serveIndex(w, r.WithContext(context.WithValue(r.Context(), indexConfigKey, &indexConfig{
		path: a.getRelativePath(blog, ""),
	})))
}

func (a *goBlog) serveDate(w http.ResponseWriter, r *http.Request) {
	var year, month, day int
	if ys := chi.URLParam(r, "year"); ys != "" && ys != "x" {
		year, _ = strconv.Atoi(ys)
	}
	if ms := chi.URLParam(r, "month"); ms != "" && ms != "x" {
		month, _ = strconv.Atoi(ms)
	}
	if ds := chi.URLParam(r, "day"); ds != "" {
		day, _ = strconv.Atoi(ds)
	}
	if year == 0 && month == 0 && day == 0 {
		a.serve404(w, r)
		return
	}
	var title, dPath strings.Builder
	if year != 0 {
		ys := fmt.Sprintf("%0004d", year)
		title.WriteString(ys)
		dPath.WriteString(ys)
	} else {
		title.WriteString("XXXX")
		dPath.WriteString("x")
	}
	if month != 0 {
		title.WriteString(fmt.Sprintf("-%02d", month))
		dPath.WriteString(fmt.Sprintf("/%02d", month))
	} else if day != 0 {
		title.WriteString("-XX")
		dPath.WriteString("/x")
	}
	if day != 0 {
		title.WriteString(fmt.Sprintf("-%02d", day))
		dPath.WriteString(fmt.Sprintf("/%02d", day))
	}
	a.serveIndex(w, r.WithContext(context.WithValue(r.Context(), indexConfigKey, &indexConfig{
		path:  a.getRelativePath(r.Context().Value(blogContextKey).(string), dPath.String()),
		year:  year,
		month: month,
		day:   day,
		title: title.String(),
	})))
}

type indexConfig struct {
	blog             string
	path             string
	section          *section
	tax              *taxonomy
	taxValue         string
	parameter        string
	year, month, day int
	title            string
	description      string
	summaryTemplate  string
}

const indexConfigKey requestContextKey = "indexConfig"

func (a *goBlog) serveIndex(w http.ResponseWriter, r *http.Request) {
	ic := r.Context().Value(indexConfigKey).(*indexConfig)
	blog := ic.blog
	if blog == "" {
		blog, _ = r.Context().Value(blogContextKey).(string)
	}
	search := chi.URLParam(r, "search")
	if search != "" {
		search = searchDecode(search)
	}
	pageNoString := chi.URLParam(r, "page")
	pageNo, _ := strconv.Atoi(pageNoString)
	var sections []string
	if ic.section != nil {
		sections = []string{ic.section.Name}
	} else {
		for sectionKey := range a.cfg.Blogs[blog].Sections {
			sections = append(sections, sectionKey)
		}
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
		status:         statusPublished,
	}, db: a.db}, a.cfg.Blogs[blog].Pagination)
	p.SetPage(pageNo)
	var posts []*post
	t := servertiming.FromContext(r.Context()).NewMetric("gp").Start()
	err := p.Results(&posts)
	t.Stop()
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
		title = fmt.Sprintf("%s: %s", a.cfg.Blogs[blog].Search.Title, search)
	}
	// Clean title
	title = bluemonday.StrictPolicy().Sanitize(title)
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
		summaryTemplate = templateSummary
	}
	a.render(w, r, templateIndex, &renderData{
		BlogString: blog,
		Canonical:  a.getFullAddress(path),
		Data: map[string]interface{}{
			"Title":           title,
			"Description":     description,
			"Posts":           posts,
			"HasPrev":         hasPrev,
			"HasNext":         hasNext,
			"First":           path,
			"Prev":            prevPath,
			"Next":            nextPath,
			"SummaryTemplate": summaryTemplate,
		},
	})
}
