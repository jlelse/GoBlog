package main

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/araddon/dateparse"
)

func (p *post) checkPost() (err error) {
	if p == nil {
		return errors.New("no post")
	}
	now := time.Now()
	// Fix content
	p.Content = strings.TrimSuffix(strings.TrimPrefix(p.Content, "\n"), "\n")
	// Fix date strings
	if p.Published != "" {
		p.Published, err = toLocal(p.Published)
		if err != nil {
			return err
		}
	}
	if p.Updated != "" {
		p.Updated, err = toLocal(p.Updated)
		if err != nil {
			return err
		}
	}
	// Check status
	if p.Status == "" {
		p.Status = statusPublished
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
	if _, ok := appConfig.Blogs[p.Blog]; !ok {
		return errors.New("blog doesn't exist")
	}
	// Check if section exists
	if _, ok := appConfig.Blogs[p.Blog].Sections[p.Section]; p.Section != "" && !ok {
		return errors.New("section doesn't exist")
	}
	// Check path
	if p.Path != "/" {
		p.Path = strings.TrimSuffix(p.Path, "/")
	}
	if p.Path == "" {
		if p.Section == "" {
			p.Section = appConfig.Blogs[p.Blog].DefaultSection
		}
		if p.Slug == "" {
			random := generateRandomString(5)
			p.Slug = fmt.Sprintf("%v-%02d-%02d-%v", now.Year(), int(now.Month()), now.Day(), random)
		}
		published, _ := dateparse.ParseLocal(p.Published)
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
	return p.createOrReplace(&postCreationOptions{new: true})
}

func (p *post) replace(oldPath string, oldStatus postStatus) error {
	return p.createOrReplace(&postCreationOptions{new: false, oldPath: oldPath, oldStatus: oldStatus})
}

type postCreationOptions struct {
	new       bool
	oldPath   string
	oldStatus postStatus
}

var postCreationMutex sync.Mutex

func (p *post) createOrReplace(o *postCreationOptions) error {
	err := p.checkPost()
	if err != nil {
		return err
	}
	// Prevent bad things
	postCreationMutex.Lock()
	defer postCreationMutex.Unlock()
	// Check if path is already in use
	if o.new || (p.Path != o.oldPath) {
		// Post is new or post path was changed
		newPathExists := false
		row, err := appDb.queryRow("select exists(select 1 from posts where path = @path)", sql.Named("path", p.Path))
		if err != nil {
			return err
		}
		err = row.Scan(&newPathExists)
		if err != nil {
			return err
		}
		if newPathExists {
			// New path already exists
			return errors.New("post already exists at given path")
		}
	}
	// Build SQL
	var sqlBuilder strings.Builder
	var sqlArgs []interface{}
	// Delete old post
	if !o.new {
		sqlBuilder.WriteString("delete from posts where path = ?;")
		sqlArgs = append(sqlArgs, o.oldPath)
	}
	// Insert new post
	sqlBuilder.WriteString("insert into posts (path, content, published, updated, blog, section, status) values (?, ?, ?, ?, ?, ?, ?);")
	sqlArgs = append(sqlArgs, p.Path, p.Content, p.Published, p.Updated, p.Blog, p.Section, p.Status)
	// Insert post parameters
	for param, value := range p.Parameters {
		for _, value := range value {
			if value != "" {
				sqlBuilder.WriteString("insert into post_parameters (path, parameter, value) values (?, ?, ?);")
				sqlArgs = append(sqlArgs, p.Path, param, value)
			}
		}
	}
	// Execute
	_, err = appDb.execMulti(sqlBuilder.String(), sqlArgs...)
	if err != nil {
		return err
	}
	// Update FTS index, trigger hooks and reload router
	rebuildFTSIndex()
	if p.Status == statusPublished {
		if o.new || o.oldStatus == statusDraft {
			defer p.postPostHooks()
		} else {
			defer p.postUpdateHooks()
		}
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
	_, err = appDb.exec("delete from posts where path = @path", sql.Named("path", p.Path))
	if err != nil {
		return err
	}
	rebuildFTSIndex()
	defer p.postDeleteHooks()
	return reloadRouter()
}

func rebuildFTSIndex() {
	_, _ = appDb.exec("insert into posts_fts(posts_fts) values ('rebuild')")
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

func getRandomPostPath(blog string) (string, error) {
	var sections []string
	for sectionKey := range appConfig.Blogs[blog].Sections {
		sections = append(sections, sectionKey)
	}
	posts, err := getPosts(&postsRequestConfig{randomOrder: true, limit: 1, blog: blog, sections: sections})
	if err != nil {
		return "", err
	} else if len(posts) == 0 {
		return "", errPostNotFound
	}
	return posts[0].Path, nil
}

type postsRequestConfig struct {
	search                                      string
	blog                                        string
	path                                        string
	limit                                       int
	offset                                      int
	sections                                    []string
	status                                      postStatus
	taxonomy                                    *taxonomy
	taxonomyValue                               string
	parameter                                   string
	parameterValue                              string
	publishedYear, publishedMonth, publishedDay int
	randomOrder                                 bool
}

func buildPostsQuery(config *postsRequestConfig) (query string, args []interface{}) {
	args = []interface{}{}
	defaultSelection := "select p.path as path, coalesce(content, '') as content, coalesce(published, '') as published, coalesce(updated, '') as updated, coalesce(blog, '') as blog, coalesce(section, '') as section, coalesce(status, '') as status, coalesce(parameter, '') as parameter, coalesce(value, '') as value "
	postsTable := "posts"
	if config.search != "" {
		postsTable = "posts_fts(@search)"
		args = append(args, sql.Named("search", config.search))
	}
	if config.status != "" && config.status != statusNil {
		postsTable = "(select * from " + postsTable + " where status = @status)"
		args = append(args, sql.Named("status", config.status))
	}
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
	if config.publishedYear != 0 {
		postsTable = "(select * from " + postsTable + " p where substr(p.published, 1, 4) = @publishedyear)"
		args = append(args, sql.Named("publishedyear", fmt.Sprintf("%0004d", config.publishedYear)))
	}
	if config.publishedMonth != 0 {
		postsTable = "(select * from " + postsTable + " p where substr(p.published, 6, 2) = @publishedmonth)"
		args = append(args, sql.Named("publishedmonth", fmt.Sprintf("%02d", config.publishedMonth)))
	}
	if config.publishedDay != 0 {
		postsTable = "(select * from " + postsTable + " p where substr(p.published, 9, 2) = @publishedday)"
		args = append(args, sql.Named("publishedday", fmt.Sprintf("%02d", config.publishedDay)))
	}
	defaultTables := " from " + postsTable + " p left outer join post_parameters pp on p.path = pp.path "
	defaultSorting := " order by p.published desc "
	if config.randomOrder {
		defaultSorting = " order by random() "
	}
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
	query, queryParams := buildPostsQuery(config)
	rows, err := appDb.query(query, queryParams...)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	paths := map[string]int{}
	for rows.Next() {
		p := &post{}
		var parameterName, parameterValue string
		err = rows.Scan(&p.Path, &p.Content, &p.Published, &p.Updated, &p.Blog, &p.Section, &p.Status, &parameterName, &parameterValue)
		if err != nil {
			return nil, err
		}
		if paths[p.Path] == 0 {
			index := len(posts)
			paths[p.Path] = index + 1
			p.Parameters = map[string][]string{}
			// Fix dates
			p.Published = toLocalSafe(p.Published)
			p.Updated = toLocalSafe(p.Updated)
			// Append
			posts = append(posts, p)
		}
		if parameterName != "" && posts != nil {
			posts[paths[p.Path]-1].Parameters[parameterName] = append(posts[paths[p.Path]-1].Parameters[parameterName], parameterValue)
		}
	}
	return posts, nil
}

func countPosts(config *postsRequestConfig) (count int, err error) {
	query, params := buildPostsQuery(config)
	query = "select count(distinct path) from (" + query + ")"
	row, err := appDb.queryRow(query, params...)
	if err != nil {
		return
	}
	err = row.Scan(&count)
	return
}

func allPostPaths(status postStatus) ([]string, error) {
	var postPaths []string
	rows, err := appDb.query("select path from posts where status = @status", sql.Named("status", status))
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var path string
		_ = rows.Scan(&path)
		if path != "" {
			postPaths = append(postPaths, path)
		}
	}
	return postPaths, nil
}

func allTaxonomyValues(blog string, taxonomy string) ([]string, error) {
	var values []string
	rows, err := appDb.query("select distinct pp.value from posts p left outer join post_parameters pp on p.path = pp.path where pp.parameter = @tax and length(coalesce(pp.value, '')) > 1 and blog = @blog and status = @status", sql.Named("tax", taxonomy), sql.Named("blog", blog), sql.Named("status", statusPublished))
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

type publishedDate struct {
	year, month, day int
}

func allPublishedDates(blog string) (dates []publishedDate, err error) {
	rows, err := appDb.query("select distinct substr(published, 1, 4) as year, substr(published, 6, 2) as month, substr(published, 9, 2) as day from posts where blog = @blog and status = @status and year != '' and month != '' and day != ''", sql.Named("blog", blog), sql.Named("status", statusPublished))
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var year, month, day int
		err = rows.Scan(&year, &month, &day)
		if err != nil {
			return nil, err
		}
		dates = append(dates, publishedDate{year, month, day})
	}
	return
}
