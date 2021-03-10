package main

import (
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

func servePost(w http.ResponseWriter, r *http.Request) {
	t := servertiming.FromContext(r.Context()).NewMetric("gp").Start()
	p, err := getPost(r.URL.Path)
	t.Stop()
	if err == errPostNotFound {
		serve404(w, r)
		return
	} else if err != nil {
		serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	if asRequest, ok := r.Context().Value(asRequestKey).(bool); ok && asRequest {
		p.serveActivityStreams(w)
		return
	}
	canonical := p.firstParameter("original")
	if canonical == "" {
		canonical = p.fullURL()
	}
	template := templatePost
	if p.Path == appConfig.Blogs[p.Blog].Path {
		template = templateStaticHome
	}
	w.Header().Add("Link", fmt.Sprintf("<%s>; rel=shortlink", p.shortURL()))
	render(w, r, template, &renderData{
		BlogString: p.Blog,
		Canonical:  canonical,
		Data:       p,
	})
}

func redirectToRandomPost(blog string) func(http.ResponseWriter, *http.Request) {
	return func(rw http.ResponseWriter, r *http.Request) {
		randomPath, err := getRandomPostPath(blog)
		if err != nil {
			serveError(rw, r, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(rw, r, randomPath, http.StatusFound)
	}
}

type postPaginationAdapter struct {
	config *postsRequestConfig
	nums   int64
}

func (p *postPaginationAdapter) Nums() (int64, error) {
	if p.nums == 0 {
		nums, _ := countPosts(p.config)
		p.nums = int64(nums)
	}
	return p.nums, nil
}

func (p *postPaginationAdapter) Slice(offset, length int, data interface{}) error {
	modifiedConfig := *p.config
	modifiedConfig.offset = offset
	modifiedConfig.limit = length

	posts, err := getPosts(&modifiedConfig)
	reflect.ValueOf(data).Elem().Set(reflect.ValueOf(&posts).Elem())
	return err
}

func serveHome(blog string, path string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if asRequest, ok := r.Context().Value(asRequestKey).(bool); ok && asRequest {
			appConfig.Blogs[blog].serveActivityStreams(blog, w, r)
			return
		}
		serveIndex(&indexConfig{
			blog: blog,
			path: path,
		})(w, r)
	}
}

func serveSection(blog string, path string, section *section) func(w http.ResponseWriter, r *http.Request) {
	return serveIndex(&indexConfig{
		blog:    blog,
		path:    path,
		section: section,
	})
}

func serveTaxonomyValue(blog string, path string, tax *taxonomy, value string) func(w http.ResponseWriter, r *http.Request) {
	return serveIndex(&indexConfig{
		blog:     blog,
		path:     path,
		tax:      tax,
		taxValue: value,
	})
}

func servePhotos(blog string, path string) func(w http.ResponseWriter, r *http.Request) {
	return serveIndex(&indexConfig{
		blog:            blog,
		path:            path,
		parameter:       appConfig.Blogs[blog].Photos.Parameter,
		title:           appConfig.Blogs[blog].Photos.Title,
		description:     appConfig.Blogs[blog].Photos.Description,
		summaryTemplate: templatePhotosSummary,
	})
}

func serveSearchResults(blog string, path string) func(w http.ResponseWriter, r *http.Request) {
	return serveIndex(&indexConfig{
		blog: blog,
		path: path,
	})
}

func serveDate(blog string, path string, year, month, day int) func(w http.ResponseWriter, r *http.Request) {
	var title strings.Builder
	if year != 0 {
		title.WriteString(fmt.Sprintf("%0004d", year))
	} else {
		title.WriteString("XXXX")
	}
	if month != 0 {
		title.WriteString(fmt.Sprintf("-%02d", month))
	} else if day != 0 {
		title.WriteString("-XX")
	}
	if day != 0 {
		title.WriteString(fmt.Sprintf("-%02d", day))
	}
	return serveIndex(&indexConfig{
		blog:  blog,
		path:  path,
		year:  year,
		month: month,
		day:   day,
		title: title.String(),
	})
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

func serveIndex(ic *indexConfig) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
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
			for sectionKey := range appConfig.Blogs[ic.blog].Sections {
				sections = append(sections, sectionKey)
			}
		}
		p := paginator.New(&postPaginationAdapter{config: &postsRequestConfig{
			blog:           ic.blog,
			sections:       sections,
			taxonomy:       ic.tax,
			taxonomyValue:  ic.taxValue,
			parameter:      ic.parameter,
			search:         search,
			publishedYear:  ic.year,
			publishedMonth: ic.month,
			publishedDay:   ic.day,
			status:         statusPublished,
		}}, appConfig.Blogs[ic.blog].Pagination)
		p.SetPage(pageNo)
		var posts []*post
		t := servertiming.FromContext(r.Context()).NewMetric("gp").Start()
		err := p.Results(&posts)
		t.Stop()
		if err != nil {
			serveError(w, r, err.Error(), http.StatusInternalServerError)
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
			title = fmt.Sprintf("%s: %s", appConfig.Blogs[ic.blog].Search.Title, search)
		}
		// Clean title
		title = bluemonday.StrictPolicy().Sanitize(title)
		// Check if feed
		if ft := feedType(chi.URLParam(r, "feed")); ft != noFeed {
			generateFeed(ic.blog, ft, w, r, posts, title, description)
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
			prevPath = fmt.Sprintf("%s/page/%d", path, prevPage)
		}
		hasNext, _ = p.HasNext()
		if hasNext {
			nextPage, _ = p.NextPage()
		} else {
			nextPage, _ = p.Page()
		}
		nextPath = fmt.Sprintf("%s/page/%d", path, nextPage)
		summaryTemplate := ic.summaryTemplate
		if summaryTemplate == "" {
			summaryTemplate = templateSummary
		}
		render(w, r, templateIndex, &renderData{
			BlogString: ic.blog,
			Canonical:  appConfig.Server.PublicAddress + path,
			Data: map[string]interface{}{
				"Title":           title,
				"Description":     description,
				"Posts":           posts,
				"HasPrev":         hasPrev,
				"HasNext":         hasNext,
				"First":           slashIfEmpty(path),
				"Prev":            slashIfEmpty(prevPath),
				"Next":            slashIfEmpty(nextPath),
				"SummaryTemplate": summaryTemplate,
			},
		})
	}
}
