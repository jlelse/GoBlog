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
	"strings"
)

var errPostNotFound = errors.New("post not found")

type Post struct {
	Path       string              `json:"path"`
	Content    string              `json:"content"`
	Published  string              `json:"published"`
	Updated    string              `json:"updated"`
	Parameters map[string][]string `json:"parameters"`
}

func (p *Post) firstParameter(parameter string) (result string) {
	if pp := p.Parameters[parameter]; len(pp) > 0 {
		result = pp[0]
	}
	return
}

func (p *Post) title() string {
	return p.firstParameter("title")
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
	return serveIndex(path, nil, nil, "", ft)
}

func serveSection(path string, section *section, ft feedType) func(w http.ResponseWriter, r *http.Request) {
	return serveIndex(path, section, nil, "", ft)
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
	return serveIndex(path, nil, tax, value, ft)
}

func serveIndex(path string, sec *section, tax *taxonomy, taxonomyValue string, ft feedType) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		pageNoString := chi.URLParam(r, "page")
		pageNo, _ := strconv.Atoi(pageNoString)
		sections := appConfig.Blog.Sections
		if sec != nil {
			sections = []*section{sec}
		}
		p := paginator.New(&postPaginationAdapter{context: r.Context(), config: &postsRequestConfig{
			sections:      sections,
			taxonomy:      tax,
			taxonomyValue: taxonomyValue,
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
		if tax != nil {
			title = fmt.Sprintf("%s: %s", tax.Title, taxonomyValue)
		} else if sec != nil {
			title = sec.Title
			description = sec.Description
		} else if tax == nil && sec == nil {
			title = appConfig.Blog.Title
			description = appConfig.Blog.Description
		}
		// Check if feed
		if ft != NONE {
			generateFeed(ft, w, posts, title, description)
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
		render(w, templateIndex, &indexTemplateData{
			Title:       title,
			Description: description,
			Posts:       posts,
			HasPrev:     p.HasPrev(),
			HasNext:     p.HasNext(),
			First:       path,
			Prev:        fmt.Sprintf("%s/page/%d", path, prevPage),
			Next:        fmt.Sprintf("%s/page/%d", path, nextPage),
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
	path          string
	limit         int
	offset        int
	sections      []*section
	taxonomy      *taxonomy
	taxonomyValue string
}

func getPosts(context context.Context, config *postsRequestConfig) (posts []*Post, err error) {
	paths := make(map[string]int)
	var rows *sql.Rows
	defaultSelection := "select p.path, coalesce(content, ''), coalesce(published, ''), coalesce(updated, ''), coalesce(parameter, ''), coalesce(value, '') "
	postsTable := "posts"
	if config.taxonomy != nil && len(config.taxonomyValue) > 0 {
		postsTable = "(select distinct p.* from " + postsTable + " p left outer join post_parameters pp on p.path = pp.path where pp.parameter = '" + config.taxonomy.Name + "' and pp.value = '" + config.taxonomyValue + "')"
	}
	if len(config.sections) > 0 {
		postsTable = "(select * from " + postsTable + " where"
		for i, section := range config.sections {
			if i > 0 {
				postsTable += " or"
			}
			postsTable += " path like '/" + section.Name + "/%'"
		}
		postsTable += ")"
	}
	defaultTables := " from " + postsTable + " p left outer join post_parameters pp on p.path = pp.path "
	defaultSorting := " order by coalesce(p.updated, p.published) desc "
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

func checkPost(post *Post) error {
	if post == nil {
		return errors.New("no post")
	}
	if post.Path == "" || !strings.HasPrefix(post.Path, "/") {
		return errors.New("wrong path")
	}
	return nil
}

func createPost(post *Post) error {
	err := checkPost(post)
	if err != nil {
		return err
	}
	startWritingToDb()
	tx, err := appDb.Begin()
	if err != nil {
		return err
	}
	_, err = tx.Exec("insert into posts (path, content, published, updated) values (?, ?, ?, ?)", post.Path, post.Content, post.Published, post.Updated)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	for param, value := range post.Parameters {
		_, err = tx.Exec("insert into post_parameters (path, parameter, value) values (?, ?, ?)", post.Path, param, value)
		if err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	err = tx.Commit()
	if err != nil {
		return err
	}
	finishWritingToDb()
	go purgeCache(post.Path)
	return reloadRouter()
}

func deletePost(post *Post) error {
	err := checkPost(post)
	if err != nil {
		return err
	}
	startWritingToDb()
	tx, err := appDb.Begin()
	if err != nil {
		return err
	}
	_, err = tx.Exec("delete from posts where path=?", post.Path)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	_, err = tx.Exec("delete from post_parameters where path=?", post.Path)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	err = tx.Commit()
	if err != nil {
		return err
	}
	finishWritingToDb()
	go purgeCache(post.Path)
	return reloadRouter()
}
