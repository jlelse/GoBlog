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
	path := slashTrimmedPath(r)
	p, err := getPost(path)
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
		canonical = appConfig.Server.PublicAddress + p.Path
	}
	render(w, templatePost, &renderData{
		blogString: p.Blog,
		Canonical:  canonical,
		Data:       p,
	})
}

type indexTemplateData struct {
	Blog        string
	Title       string
	Description string
	Posts       []*post
	HasPrev     bool
	HasNext     bool
	First       string
	Prev        string
	Next        string
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
			Canonical:  appConfig.Server.PublicAddress + slashTrimmedPath(r),
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

func servePhotos(blog string) func(w http.ResponseWriter, r *http.Request) {
	return serveIndex(&indexConfig{
		blog:      blog,
		path:      appConfig.Blogs[blog].Photos.Path,
		parameter: appConfig.Blogs[blog].Photos.Parameter,
		template:  templatePhotos,
	})
}

type indexConfig struct {
	blog      string
	path      string
	section   *section
	tax       *taxonomy
	taxValue  string
	parameter string
	template  string
}

func serveIndex(ic *indexConfig) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
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
		}}, appConfig.Blogs[ic.blog].Pagination)
		p.SetPage(pageNo)
		var posts []*post
		err := p.Results(&posts)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// Meta
		var title, description string
		if ic.tax != nil {
			title = fmt.Sprintf("%s: %s", ic.tax.Title, ic.taxValue)
		} else if ic.section != nil {
			title = ic.section.Title
			description = ic.section.Description
		}
		// Check if feed
		if ft := feedType(chi.URLParam(r, "feed")); ft != noFeed {
			generateFeed(ic.blog, ft, w, r, posts, title, description)
			return
		}
		// Navigation
		prevPage, err := p.PrevPage()
		if err == paginator.ErrNoPrevPage {
			prevPage = p.Page()
		}
		nextPage, err := p.NextPage()
		if err == paginator.ErrNoNextPage {
			nextPage = p.Page()
		}
		template := ic.template
		if len(template) == 0 {
			template = templateIndex
		}
		render(w, template, &renderData{
			blogString: ic.blog,
			Canonical:  appConfig.Server.PublicAddress + slashTrimmedPath(r),
			Data: &indexTemplateData{
				Title:       title,
				Description: description,
				Posts:       posts,
				HasPrev:     p.HasPrev(),
				HasNext:     p.HasNext(),
				First:       ic.path,
				Prev:        fmt.Sprintf("%s/page/%d", ic.path, prevPage),
				Next:        fmt.Sprintf("%s/page/%d", ic.path, nextPage),
			},
		})
	}
}

func getPost(path string) (*post, error) {
	posts, err := getPosts(&postsRequestConfig{path: path})
	if err != nil {
		return nil, err
	} else if len(posts) == 0 {
		return nil, errPostNotFound
	}
	return posts[0], nil
}

type postsRequestConfig struct {
	blog           string
	path           string
	limit          int
	offset         int
	sections       []string
	taxonomy       *taxonomy
	taxonomyValue  string
	parameter      string
	parameterValue string
}

func buildQuery(config *postsRequestConfig) (query string, params []interface{}) {
	defaultSelection := "select p.path as path, coalesce(content, ''), coalesce(published, ''), coalesce(updated, ''), coalesce(blog, ''), coalesce(section, ''), coalesce(parameter, ''), coalesce(value, '') "
	postsTable := "posts"
	if config.blog != "" {
		postsTable = "(select * from " + postsTable + " where blog = '" + config.blog + "')"
	}
	if config.parameter != "" {
		if config.parameterValue != "" {
			postsTable = "(select distinct p.* from " + postsTable + " p left outer join post_parameters pp on p.path = pp.path where pp.parameter = '" + config.parameter + "' and pp.value = '" + config.parameterValue + "')"
		} else {
			postsTable = "(select distinct p.* from " + postsTable + " p left outer join post_parameters pp on p.path = pp.path where pp.parameter = '" + config.parameter + "' and length(coalesce(pp.value, '')) > 1)"
		}
	}
	if config.taxonomy != nil && len(config.taxonomyValue) > 0 {
		postsTable = "(select distinct p.* from " + postsTable + " p left outer join post_parameters pp on p.path = pp.path where pp.parameter = '" + config.taxonomy.Name + "' and lower(pp.value) = lower('" + config.taxonomyValue + "'))"
	}
	if len(config.sections) > 0 {
		postsTable = "(select * from " + postsTable + " where"
		for i, section := range config.sections {
			if i > 0 {
				postsTable += " or"
			}
			postsTable += " section='" + section + "'"
		}
		postsTable += ")"
	}
	defaultTables := " from " + postsTable + " p left outer join post_parameters pp on p.path = pp.path "
	defaultSorting := " order by p.published desc "
	if config.path != "" {
		query = defaultSelection + defaultTables + " where p.path=?" + defaultSorting
		params = []interface{}{config.path}
	} else if config.limit != 0 || config.offset != 0 {
		query = defaultSelection + " from (select * from " + postsTable + " p " + defaultSorting + " limit ? offset ?) p left outer join post_parameters pp on p.path = pp.path "
		params = []interface{}{config.limit, config.offset}
	} else {
		query = defaultSelection + defaultTables + defaultSorting
	}
	return
}

func getPosts(config *postsRequestConfig) (posts []*post, err error) {
	query, queryParams := buildQuery(config)
	rows, err := appDb.Query(query, queryParams...)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	paths := make(map[string]int)
	for rows.Next() {
		p := &post{}
		var parameterName, parameterValue string
		err = rows.Scan(&p.Path, &p.Content, &p.Published, &p.Updated, &p.Blog, &p.Section, &parameterName, &parameterValue)
		if err != nil {
			return nil, err
		}
		if paths[p.Path] == 0 {
			index := len(posts)
			paths[p.Path] = index + 1
			p.Parameters = make(map[string][]string)
			posts = append(posts, p)
		}
		if parameterName != "" && posts != nil {
			posts[paths[p.Path]-1].Parameters[parameterName] = append(posts[paths[p.Path]-1].Parameters[parameterName], parameterValue)
		}
	}
	return posts, nil
}

func countPosts(config *postsRequestConfig) (count int, err error) {
	query, params := buildQuery(config)
	query = "select count(distinct path) from (" + query + ")"
	row := appDb.QueryRow(query, params...)
	err = row.Scan(&count)
	return
}

func allPostPaths() ([]string, error) {
	var postPaths []string
	rows, err := appDb.Query("select path from posts")
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var path string
		_ = rows.Scan(&path)
		postPaths = append(postPaths, path)
	}
	return postPaths, nil
}

func allTaxonomyValues(blog string, taxonomy string) ([]string, error) {
	var values []string
	rows, err := appDb.Query("select distinct pp.value from posts p left outer join post_parameters pp on p.path = pp.path where pp.parameter = ? and length(coalesce(pp.value, '')) > 1 and blog = ?", taxonomy, blog)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var value string
		_ = rows.Scan(&value)
		values = append(values, value)
	}
	return values, nil
}
