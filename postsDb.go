package main

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/araddon/dateparse"
)

func (p *post) checkPost() error {
	if p == nil {
		return errors.New("no post")
	}
	now := time.Now()
	// Fix content
	p.Content = strings.TrimSuffix(strings.TrimPrefix(p.Content, "\n"), "\n")
	// Fix date strings
	if p.Published != "" {
		d, err := dateparse.ParseIn(p.Published, time.Local)
		if err != nil {
			return err
		}
		p.Published = d.String()
	}
	if p.Updated != "" {
		d, err := dateparse.ParseIn(p.Updated, time.Local)
		if err != nil {
			return err
		}
		p.Updated = d.String()
	}
	// Cleanup params
	for key, value := range p.Parameters {
		if value == nil {
			delete(p.Parameters, key)
			continue
		}
		allValues := []string{}
		for _, v := range value {
			if v != "" {
				allValues = append(allValues, v)
			}
		}
		if len(allValues) >= 1 {
			p.Parameters[key] = allValues
		} else {
			delete(p.Parameters, key)
		}
	}
	// Check blog
	if p.Blog == "" {
		p.Blog = appConfig.DefaultBlog
	}
	// Check path
	p.Path = strings.TrimSuffix(p.Path, "/")
	if p.Path == "" {
		if p.Section == "" {
			p.Section = appConfig.Blogs[p.Blog].DefaultSection
		}
		if p.Slug == "" {
			random := generateRandomString(5)
			p.Slug = fmt.Sprintf("%v-%02d-%02d-%v", now.Year(), int(now.Month()), now.Day(), random)
		}
		published, _ := dateparse.ParseIn(p.Published, time.Local)
		pathTmplString := appConfig.Blogs[p.Blog].Sections[p.Section].PathTemplate
		if pathTmplString == "" {
			return errors.New("path template empty")
		}
		pathTmpl, err := template.New("location").Parse(pathTmplString)
		if err != nil {
			return errors.New("failed to parse location template")
		}
		var pathBuffer bytes.Buffer
		err = pathTmpl.Execute(&pathBuffer, map[string]interface{}{
			"BlogPath": appConfig.Blogs[p.Blog].Path,
			"Year":     published.Year(),
			"Month":    int(published.Month()),
			"Day":      published.Day(),
			"Slug":     p.Slug,
			"Section":  p.Section,
		})
		if err != nil {
			return errors.New("failed to execute location template")
		}
		p.Path = pathBuffer.String()
	}
	if p.Path != "" && !strings.HasPrefix(p.Path, "/") {
		return errors.New("wrong path")
	}
	return nil
}

func (p *post) create() error {
	return p.createOrReplace(true)
}

func (p *post) replace() error {
	return p.createOrReplace(false)
}

func (p *post) createOrReplace(new bool) error {
	err := p.checkPost()
	if err != nil {
		return err
	}
	startWritingToDb()
	postExists := postExists(p.Path)
	if postExists && new {
		finishWritingToDb()
		return errors.New("post already exists at given path")
	}
	tx, err := appDb.Begin()
	if err != nil {
		finishWritingToDb()
		return err
	}
	if postExists {
		_, err := tx.Exec("delete from posts where path = @path", sql.Named("path", p.Path))
		if err != nil {
			_ = tx.Rollback()
			finishWritingToDb()
			return err
		}
	}
	_, err = tx.Exec(
		"insert into posts (path, content, published, updated, blog, section) values (@path, @content, @published, @updated, @blog, @section)",
		sql.Named("path", p.Path), sql.Named("content", p.Content), sql.Named("published", p.Published), sql.Named("updated", p.Updated), sql.Named("blog", p.Blog), sql.Named("section", p.Section))
	if err != nil {
		_ = tx.Rollback()
		finishWritingToDb()
		return err
	}
	ppStmt, err := tx.Prepare("insert into post_parameters (path, parameter, value) values (@path, @parameter, @value)")
	if err != nil {
		_ = tx.Rollback()
		finishWritingToDb()
		return err
	}
	for param, value := range p.Parameters {
		for _, value := range value {
			if value != "" {
				_, err := ppStmt.Exec(sql.Named("path", p.Path), sql.Named("parameter", param), sql.Named("value", value))
				if err != nil {
					_ = tx.Rollback()
					finishWritingToDb()
					return err
				}
			}
		}
	}
	err = tx.Commit()
	if err != nil {
		finishWritingToDb()
		return err
	}
	finishWritingToDb()
	if !postExists {
		defer p.postPostHooks()
	} else {
		defer p.postUpdateHooks()
	}
	return reloadRouter()
}

func deletePost(path string) error {
	if path == "" {
		return nil
	}
	p, err := getPost(path)
	if err != nil {
		return err
	}
	_, err = appDbExec("delete from posts where path = @path", sql.Named("path", p.Path))
	defer p.postDeleteHooks()
	return reloadRouter()
}

func postExists(path string) bool {
	result := 0
	row, err := appDbQueryRow("select exists(select 1 from posts where path = @path)", sql.Named("path", path))
	if err != nil {
		return false
	}
	if err = row.Scan(&result); err != nil {
		return false
	}
	return result == 1
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

func buildQuery(config *postsRequestConfig) (query string, args []interface{}) {
	args = []interface{}{}
	defaultSelection := "select p.path as path, coalesce(content, ''), coalesce(published, ''), coalesce(updated, ''), coalesce(blog, ''), coalesce(section, ''), coalesce(parameter, ''), coalesce(value, '') "
	postsTable := "posts"
	if config.blog != "" {
		postsTable = "(select * from " + postsTable + " where blog = @blog)"
		args = append(args, sql.Named("blog", config.blog))
	}
	if config.parameter != "" {
		postsTable = "(select distinct p.* from " + postsTable + " p left outer join post_parameters pp on p.path = pp.path where pp.parameter = @param "
		args = append(args, sql.Named("param", config.parameter))
		if config.parameterValue != "" {
			postsTable += "and pp.value = @paramval)"
			args = append(args, sql.Named("paramval", config.parameterValue))
		} else {
			postsTable += "and length(coalesce(pp.value, '')) > 1)"
		}
	}
	if config.taxonomy != nil && len(config.taxonomyValue) > 0 {
		postsTable = "(select distinct p.* from " + postsTable + " p left outer join post_parameters pp on p.path = pp.path where pp.parameter = @taxname and lower(pp.value) = lower(@taxval))"
		args = append(args, sql.Named("taxname", config.taxonomy.Name), sql.Named("taxval", config.taxonomyValue))
	}
	if len(config.sections) > 0 {
		postsTable = "(select * from " + postsTable + " where"
		for i, section := range config.sections {
			if i > 0 {
				postsTable += " or"
			}
			named := fmt.Sprintf("section%v", i)
			postsTable += fmt.Sprintf(" section = @%v", named)
			args = append(args, sql.Named(named, section))
		}
		postsTable += ")"
	}
	defaultTables := " from " + postsTable + " p left outer join post_parameters pp on p.path = pp.path "
	defaultSorting := " order by p.published desc "
	if config.path != "" {
		query = defaultSelection + defaultTables + " where p.path = @path" + defaultSorting
		args = append(args, sql.Named("path", config.path))
	} else if config.limit != 0 || config.offset != 0 {
		query = defaultSelection + " from (select * from " + postsTable + " p " + defaultSorting + " limit @limit offset @offset) p left outer join post_parameters pp on p.path = pp.path "
		args = append(args, sql.Named("limit", config.limit), sql.Named("offset", config.offset))
	} else {
		query = defaultSelection + defaultTables + defaultSorting
	}
	return
}

func getPosts(config *postsRequestConfig) (posts []*post, err error) {
	query, queryParams := buildQuery(config)
	rows, err := appDbQuery(query, queryParams...)
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
	row, err := appDbQueryRow(query, params...)
	if err != nil {
		return
	}
	err = row.Scan(&count)
	return
}

func allPostPaths() ([]string, error) {
	var postPaths []string
	rows, err := appDbQuery("select path from posts")
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
	rows, err := appDbQuery("select distinct pp.value from posts p left outer join post_parameters pp on p.path = pp.path where pp.parameter = @tax and length(coalesce(pp.value, '')) > 1 and blog = @blog", sql.Named("tax", taxonomy), sql.Named("blog", blog))
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
