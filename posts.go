package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/go-chi/chi"
	"github.com/vcraescu/go-paginator"
	"net/http"
	"reflect"
	"strconv"
)

var errPostNotFound = errors.New("post not found")

type Post struct {
	Path       string              `json:"path"`
	Content    string              `json:"content"`
	Published  string              `json:"published"`
	Updated    string              `json:"updated"`
	Parameters map[string][]string `json:"parameters"`
}

func servePost(w http.ResponseWriter, r *http.Request) {
	path := slashTrimmedPath(r)
	post, err := getPost(r.Context(), path)
	if err == errPostNotFound {
		serve404(w, r)
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	render(w, templatePost, post)
}

type indexTemplateData struct {
	Title       string
	Description string
	Posts       []*Post
	HasPrev     bool
	HasNext     bool
	First       string
	Prev        string
	Next        string
}

type postPaginationAdapter struct {
	context context.Context
	config  *postsRequestConfig
	nums    int
}

func (p *postPaginationAdapter) Nums() int {
	if p.nums == 0 {
		p.nums, _ = countPosts(p.context, p.config)
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

	posts, err := getPosts(p.context, &modifiedConfig)
	reflect.ValueOf(data).Elem().Set(reflect.ValueOf(&posts).Elem())
	return err
}

func serveHome(path string, ft feedType) func(w http.ResponseWriter, r *http.Request) {
	return serveIndex(&indexConfig{
		path: path,
		feed: ft,
	})
}

func serveSection(path string, section *section, ft feedType) func(w http.ResponseWriter, r *http.Request) {
	return serveIndex(&indexConfig{
		path:    path,
		section: section,
		feed:    ft,
	})
}

func serveTaxonomy(tax *taxonomy) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		allValues, err := allTaxonomyValues(tax.Name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		render(w, templateTaxonomy, struct {
			Taxonomy       *taxonomy
			TaxonomyValues []string
		}{
			Taxonomy:       tax,
			TaxonomyValues: allValues,
		})
	}
}

func serveTaxonomyValue(path string, tax *taxonomy, value string, ft feedType) func(w http.ResponseWriter, r *http.Request) {
	return serveIndex(&indexConfig{
		path:     path,
		tax:      tax,
		taxValue: value,
		feed:     ft,
	})
}

func servePhotos(path string) func(w http.ResponseWriter, r *http.Request) {
	return serveIndex(&indexConfig{
		path:              path,
		onlyWithParameter: appConfig.Blog.Photos.Parameter,
		template:          templatePhotos,
	})
}

type indexConfig struct {
	path              string
	section           *section
	tax               *taxonomy
	taxValue          string
	feed              feedType
	onlyWithParameter string
	template          string
}

func serveIndex(ic *indexConfig) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		pageNoString := chi.URLParam(r, "page")
		pageNo, _ := strconv.Atoi(pageNoString)
		sections := appConfig.Blog.Sections
		if ic.section != nil {
			sections = []*section{ic.section}
		}
		p := paginator.New(&postPaginationAdapter{context: r.Context(), config: &postsRequestConfig{
			sections:          sections,
			taxonomy:          ic.tax,
			taxonomyValue:     ic.taxValue,
			onlyWithParameter: ic.onlyWithParameter,
		}}, appConfig.Blog.Pagination)
		p.SetPage(pageNo)
		var posts []*Post
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
		if ic.feed != NONE {
			generateFeed(ic.feed, w, r, posts, title, description)
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
		render(w, template, &indexTemplateData{
			Title:       title,
			Description: description,
			Posts:       posts,
			HasPrev:     p.HasPrev(),
			HasNext:     p.HasNext(),
			First:       ic.path,
			Prev:        fmt.Sprintf("%s/page/%d", ic.path, prevPage),
			Next:        fmt.Sprintf("%s/page/%d", ic.path, nextPage),
		})
	}
}

func getPost(context context.Context, path string) (*Post, error) {
	posts, err := getPosts(context, &postsRequestConfig{path: path})
	if err != nil {
		return nil, err
	} else if len(posts) == 0 {
		return nil, errPostNotFound
	}
	return posts[0], nil
}

type postsRequestConfig struct {
	path              string
	limit             int
	offset            int
	sections          []*section
	taxonomy          *taxonomy
	taxonomyValue     string
	onlyWithParameter string
}

func getPosts(context context.Context, config *postsRequestConfig) (posts []*Post, err error) {
	paths := make(map[string]int)
	var rows *sql.Rows
	defaultSelection := "select p.path, coalesce(content, ''), coalesce(published, ''), coalesce(updated, ''), coalesce(parameter, ''), coalesce(value, '') "
	postsTable := "posts"
	if config.onlyWithParameter != "" {
		postsTable = "(select distinct p.* from " + postsTable + " p left outer join post_parameters pp on p.path = pp.path where pp.parameter = '" + config.onlyWithParameter + "' and length(coalesce(pp.value, '')) > 1)"
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
			postsTable += " section='" + section.Name + "'"
		}
		postsTable += ")"
	}
	defaultTables := " from " + postsTable + " p left outer join post_parameters pp on p.path = pp.path "
	defaultSorting := " order by p.published desc "
	if config.path != "" {
		query := defaultSelection + defaultTables + " where p.path=?" + defaultSorting
		rows, err = appDb.QueryContext(context, query, config.path)
	} else if config.limit != 0 || config.offset != 0 {
		query := defaultSelection + " from (select * from " + postsTable + " p " + defaultSorting + " limit ? offset ?) p left outer join post_parameters pp on p.path = pp.path "
		rows, err = appDb.QueryContext(context, query, config.limit, config.offset)
	} else {
		query := defaultSelection + defaultTables + defaultSorting
		rows, err = appDb.QueryContext(context, query)
	}
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	for rows.Next() {
		post := &Post{}
		var parameterName, parameterValue string
		err = rows.Scan(&post.Path, &post.Content, &post.Published, &post.Updated, &parameterName, &parameterValue)
		if err != nil {
			return nil, err
		}
		if paths[post.Path] == 0 {
			index := len(posts)
			paths[post.Path] = index + 1
			post.Parameters = make(map[string][]string)
			posts = append(posts, post)
		}
		if parameterName != "" && posts != nil {
			posts[paths[post.Path]-1].Parameters[parameterName] = append(posts[paths[post.Path]-1].Parameters[parameterName], parameterValue)
		}
	}
	return posts, nil
}

func countPosts(context context.Context, config *postsRequestConfig) (int, error) {
	posts, err := getPosts(context, config)
	return len(posts), err
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

func allTaxonomyValues(taxonomy string) ([]string, error) {
	var values []string
	rows, err := appDb.Query("select distinct value from post_parameters where parameter = ? and value not null and value != ''", taxonomy)
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
