package main

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/go-chi/chi"
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
	// Not persisted
	Slug     string `json:"slug"`
	rendered template.HTML
}

func servePost(w http.ResponseWriter, r *http.Request) {
	as := strings.HasSuffix(r.URL.Path, ".as")
	if as {
		r.URL.Path = strings.TrimSuffix(r.URL.Path, ".as")
	}
	p, err := getPost(r.URL.Path)
	if err == errPostNotFound {
		serve404(w, r)
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if as {
		p.serveActivityStreams(w)
		return
	}
	canonical := p.firstParameter("original")
	if canonical == "" {
		canonical = p.fullURL()
	}
	render(w, templatePost, &renderData{
		blogString: p.Blog,
		Canonical:  canonical,
		Data:       p,
	})
}

type postPaginationAdapter struct {
	config *postsRequestConfig
	nums   int
}

func (p *postPaginationAdapter) Nums() int {
	if p.nums == 0 {
		p.nums, _ = countPosts(p.config)
	}
	return p.nums
}

func (p *postPaginationAdapter) Slice(offset, length int, data interface{}) error {
	if reflect.TypeOf(data).Kind() != reflect.Ptr {
		panic("data has to be a pointer")
	}

	modifiedConfig := *p.config
	modifiedConfig.offset = offset
	modifiedConfig.limit = length

	posts, err := getPosts(&modifiedConfig)
	reflect.ValueOf(data).Elem().Set(reflect.ValueOf(&posts).Elem())
	return err
}

func serveHome(blog string, path string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		as := strings.HasSuffix(r.URL.Path, ".as")
		if as {
			appConfig.Blogs[blog].serveActivityStreams(blog, w)
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

func serveTaxonomy(blog string, tax *taxonomy) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		allValues, err := allTaxonomyValues(blog, tax.Name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		render(w, templateTaxonomy, &renderData{
			blogString: blog,
			Canonical:  appConfig.Server.PublicAddress + r.URL.Path,
			Data: struct {
				Taxonomy       *taxonomy
				TaxonomyValues []string
			}{
				Taxonomy:       tax,
				TaxonomyValues: allValues,
			},
		})
	}
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

type indexConfig struct {
	blog            string
	path            string
	section         *section
	tax             *taxonomy
	taxValue        string
	parameter       string
	title           string
	description     string
	summaryTemplate string
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
			blog:          ic.blog,
			sections:      sections,
			taxonomy:      ic.tax,
			taxonomyValue: ic.taxValue,
			parameter:     ic.parameter,
			search:        search,
		}}, appConfig.Blogs[ic.blog].Pagination)
		p.SetPage(pageNo)
		var posts []*post
		err := p.Results(&posts)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
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
		prevPage, err := p.PrevPage()
		if err == paginator.ErrNoPrevPage {
			prevPage = p.Page()
		}
		prevPath := fmt.Sprintf("%s/page/%d", path, prevPage)
		if prevPage < 2 {
			prevPath = path
		}
		nextPage, err := p.NextPage()
		if err == paginator.ErrNoNextPage {
			nextPage = p.Page()
		}
		nextPath := fmt.Sprintf("%s/page/%d", path, nextPage)
		summaryTemplate := ic.summaryTemplate
		if summaryTemplate == "" {
			summaryTemplate = templateSummary
		}
		render(w, templateIndex, &renderData{
			blogString: ic.blog,
			Canonical:  appConfig.Server.PublicAddress + path,
			Data: map[string]interface{}{
				"Title":           title,
				"Description":     description,
				"Posts":           posts,
				"HasPrev":         p.HasPrev(),
				"HasNext":         p.HasNext(),
				"First":           path,
				"Prev":            prevPath,
				"Next":            nextPath,
				"SummaryTemplate": summaryTemplate,
			},
		})
	}
}
