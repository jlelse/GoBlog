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
	Path       string            `json:"path"`
	Content    string            `json:"content"`
	Published  string            `json:"published"`
	Updated    string            `json:"updated"`
	Parameters map[string]string `json:"parameters"`
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

type indexTemplateDate struct {
	Posts   []*Post
	HasPrev bool
	HasNext bool
	Prev    string
	Next    string
}

type postPaginationAdapter struct {
	context context.Context
	nums    int
}

func (p *postPaginationAdapter) Nums() int {
	if p.nums == 0 {
		p.nums, _ = getPostsNum(p.context, &postsRequestConfig{})
	}
	return p.nums
}

func (p *postPaginationAdapter) Slice(offset, length int, data interface{}) error {
	if reflect.TypeOf(data).Kind() != reflect.Ptr {
		panic("data has to be a pointer")
	}

	posts, err := getPosts(p.context, &postsRequestConfig{
		offset: offset,
		limit:  length,
	})
	reflect.ValueOf(data).Elem().Set(reflect.ValueOf(&posts).Elem())
	return err
}

func serveIndex(path string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		pageNoString := chi.URLParam(r, "page")
		pageNo, _ := strconv.Atoi(pageNoString)
		p := paginator.New(&postPaginationAdapter{context: r.Context()}, appConfig.Blog.Pagination)
		p.SetPage(pageNo)
		var posts []*Post
		err := p.Results(&posts)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		prevPage, err := p.PrevPage()
		if err == paginator.ErrNoPrevPage {
			prevPage = p.Page()
		}
		nextPage, err := p.NextPage()
		if err == paginator.ErrNoNextPage {
			nextPage = p.Page()
		}
		render(w, templateIndex, &indexTemplateDate{
			Posts:   posts,
			HasPrev: p.HasPrev(),
			HasNext: p.HasNext(),
			Prev:    fmt.Sprintf("%s/page/%d", path, prevPage),
			Next:    fmt.Sprintf("%s/page/%d", path, nextPage),
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
	path   string
	limit  int
	offset int
}

func getPosts(context context.Context, config *postsRequestConfig) (posts []*Post, err error) {
	paths := make(map[string]int)
	var rows *sql.Rows
	if config.path != "" {
		rows, err = appDb.QueryContext(context, "select p.path, COALESCE(content, ''), COALESCE(published, ''), COALESCE(updated, ''), COALESCE(parameter, ''), COALESCE(value, '') from posts p left outer join post_parameters pp on p.path = pp.path where p.path=?", config.path)
	} else if config.limit != 0 || config.offset != 0 {
		rows, err = appDb.QueryContext(context, "select p.path, COALESCE(content, ''), COALESCE(published, ''), COALESCE(updated, ''), COALESCE(parameter, ''), COALESCE(value, '') from posts p left outer join post_parameters pp on p.path = pp.path limit ? offset ?", config.limit, config.offset)
	} else {
		rows, err = appDb.QueryContext(context, "select p.path, COALESCE(content, ''), COALESCE(published, ''), COALESCE(updated, ''), COALESCE(parameter, ''), COALESCE(value, '') from posts p left outer join post_parameters pp on p.path = pp.path")
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
			post.Parameters = make(map[string]string)
			posts = append(posts, post)
		}
		if parameterName != "" && posts != nil {
			posts[paths[post.Path]-1].Parameters[parameterName] = parameterValue
		}
	}
	return posts, nil
}

func getPostsNum(context context.Context, _ *postsRequestConfig) (num int, err error) {
	err = appDb.QueryRowContext(context, "select count(*) from posts").Scan(&num)
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
