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
	"github.com/thoas/go-funk"
)

func (a *goBlog) checkPost(p *post) (err error) {
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
		p.Blog = a.cfg.DefaultBlog
	}
	if _, ok := a.cfg.Blogs[p.Blog]; !ok {
		return errors.New("blog doesn't exist")
	}
	// Check if section exists
	if _, ok := a.cfg.Blogs[p.Blog].Sections[p.Section]; p.Section != "" && !ok {
		return errors.New("section doesn't exist")
	}
	// Check path
	if p.Path != "/" {
		p.Path = strings.TrimSuffix(p.Path, "/")
	}
	if p.Path == "" {
		if p.Section == "" {
			p.Section = a.cfg.Blogs[p.Blog].DefaultSection
		}
		if p.Slug == "" {
			random := generateRandomString(5)
			p.Slug = fmt.Sprintf("%v-%02d-%02d-%v", now.Year(), int(now.Month()), now.Day(), random)
		}
		published, _ := dateparse.ParseLocal(p.Published)
		pathTmplString := a.cfg.Blogs[p.Blog].Sections[p.Section].PathTemplate
		if pathTmplString == "" {
			return errors.New("path template empty")
		}
		pathTmpl, err := template.New("location").Parse(pathTmplString)
		if err != nil {
			return errors.New("failed to parse location template")
		}
		var pathBuffer bytes.Buffer
		err = pathTmpl.Execute(&pathBuffer, map[string]interface{}{
			"BlogPath": a.getRelativePath(p.Blog, ""),
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

func (a *goBlog) createPost(p *post) error {
	return a.createOrReplacePost(p, &postCreationOptions{new: true})
}

func (a *goBlog) replacePost(p *post, oldPath string, oldStatus postStatus) error {
	return a.createOrReplacePost(p, &postCreationOptions{new: false, oldPath: oldPath, oldStatus: oldStatus})
}

type postCreationOptions struct {
	new       bool
	oldPath   string
	oldStatus postStatus
}

func (a *goBlog) createOrReplacePost(p *post, o *postCreationOptions) error {
	// Check post
	if err := a.checkPost(p); err != nil {
		return err
	}
	// Save to db
	if err := a.db.savePost(p, o); err != nil {
		return err
	}
	// Trigger hooks
	if p.Status == statusPublished {
		if o.new || o.oldStatus == statusDraft {
			defer a.postPostHooks(p)
		} else {
			defer a.postUpdateHooks(p)
		}
	}
	// Reload router
	return a.reloadRouter()
}

// Save check post to database
func (db *database) savePost(p *post, o *postCreationOptions) error {
	// Check
	if !o.new && o.oldPath == "" {
		return errors.New("old path required")
	}
	// Lock post creation
	db.pcm.Lock()
	defer db.pcm.Unlock()
	// Build SQL
	var sqlBuilder strings.Builder
	var sqlArgs = []interface{}{dbNoCache}
	// Start transaction
	sqlBuilder.WriteString("begin;")
	// Delete old post
	if !o.new {
		sqlBuilder.WriteString("delete from posts where path = ?;delete from post_parameters where path = ?;")
		sqlArgs = append(sqlArgs, o.oldPath, o.oldPath)
	}
	// Insert new post
	sqlBuilder.WriteString("insert into posts (path, content, published, updated, blog, section, status, priority) values (?, ?, ?, ?, ?, ?, ?, ?);")
	sqlArgs = append(sqlArgs, p.Path, p.Content, p.Published, p.Updated, p.Blog, p.Section, p.Status, p.Priority)
	// Insert post parameters
	for param, value := range p.Parameters {
		for _, value := range value {
			if value != "" {
				sqlBuilder.WriteString("insert into post_parameters (path, parameter, value) values (?, ?, ?);")
				sqlArgs = append(sqlArgs, p.Path, param, value)
			}
		}
	}
	// Commit transaction
	sqlBuilder.WriteString("commit;")
	// Execute
	if _, err := db.exec(sqlBuilder.String(), sqlArgs...); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed: posts.path") {
			return errors.New("post already exists at given path")
		}
		return err
	}
	// Update FTS index
	db.rebuildFTSIndex()
	return nil
}

func (a *goBlog) deletePost(path string) error {
	p, err := a.db.deletePost(path)
	if err != nil || p == nil {
		return err
	}
	defer a.postDeleteHooks(p)
	return a.reloadRouter()
}

func (db *database) deletePost(path string) (*post, error) {
	if path == "" {
		return nil, nil
	}
	p, err := db.getPost(path)
	if err != nil {
		return nil, err
	}
	_, err = db.exec("begin;delete from posts where path = ?;delete from post_parameters where path = ?;commit;", dbNoCache, p.Path, p.Path)
	if err != nil {
		return nil, err
	}
	db.rebuildFTSIndex()
	return p, nil
}

type postsRequestConfig struct {
	search                                      string
	blog                                        string
	path                                        string
	limit                                       int
	offset                                      int
	sections                                    []string
	status                                      postStatus
	taxonomy                                    *configTaxonomy
	taxonomyValue                               string
	parameter                                   string
	parameterValue                              string
	publishedYear, publishedMonth, publishedDay int
	randomOrder                                 bool
	priorityOrder                               bool
	withoutParameters                           bool
	withOnlyParameters                          []string
}

func buildPostsQuery(c *postsRequestConfig, selection string) (query string, args []interface{}) {
	args = []interface{}{}
	table := "posts"
	if c.search != "" {
		table = "posts_fts(@search)"
		args = append(args, sql.Named("search", c.search))
	}
	var wheres []string
	if c.path != "" {
		wheres = append(wheres, "path = @path")
		args = append(args, sql.Named("path", c.path))
	}
	if c.status != "" && c.status != statusNil {
		wheres = append(wheres, "status = @status")
		args = append(args, sql.Named("status", c.status))
	}
	if c.blog != "" {
		wheres = append(wheres, "blog = @blog")
		args = append(args, sql.Named("blog", c.blog))
	}
	if c.parameter != "" {
		if c.parameterValue != "" {
			wheres = append(wheres, "path in (select path from post_parameters where parameter = @param and value = @paramval)")
			args = append(args, sql.Named("param", c.parameter), sql.Named("paramval", c.parameterValue))
		} else {
			wheres = append(wheres, "path in (select path from post_parameters where parameter = @param and length(coalesce(value, '')) > 0)")
			args = append(args, sql.Named("param", c.parameter))
		}
	}
	if c.taxonomy != nil && len(c.taxonomyValue) > 0 {
		wheres = append(wheres, "path in (select path from post_parameters where parameter = @taxname and lower(value) = lower(@taxval))")
		args = append(args, sql.Named("taxname", c.taxonomy.Name), sql.Named("taxval", c.taxonomyValue))
	}
	if len(c.sections) > 0 {
		ws := "section in ("
		for i, section := range c.sections {
			if i > 0 {
				ws += ", "
			}
			named := fmt.Sprintf("section%v", i)
			ws += "@" + named
			args = append(args, sql.Named(named, section))
		}
		ws += ")"
		wheres = append(wheres, ws)
	}
	if c.publishedYear != 0 {
		wheres = append(wheres, "substr(published, 1, 4) = @publishedyear")
		args = append(args, sql.Named("publishedyear", fmt.Sprintf("%0004d", c.publishedYear)))
	}
	if c.publishedMonth != 0 {
		wheres = append(wheres, "substr(published, 6, 2) = @publishedmonth")
		args = append(args, sql.Named("publishedmonth", fmt.Sprintf("%02d", c.publishedMonth)))
	}
	if c.publishedDay != 0 {
		wheres = append(wheres, "substr(published, 9, 2) = @publishedday")
		args = append(args, sql.Named("publishedday", fmt.Sprintf("%02d", c.publishedDay)))
	}
	if len(wheres) > 0 {
		table += " where " + strings.Join(wheres, " and ")
	}
	sorting := " order by published desc"
	if c.randomOrder {
		sorting = " order by random()"
	} else if c.priorityOrder {
		sorting = " order by priority desc, published desc"
	}
	table += sorting
	if c.limit != 0 || c.offset != 0 {
		table += " limit @limit offset @offset"
		args = append(args, sql.Named("limit", c.limit), sql.Named("offset", c.offset))
	}
	query = "select " + selection + " from " + table
	return query, args
}

func (d *database) getPostParameters(path string, parameters ...string) (params map[string][]string, err error) {
	var sqlArgs []interface{}
	// Parameter filter
	paramFilter := ""
	if len(parameters) > 0 {
		paramFilter = " and parameter in ("
		for i, p := range parameters {
			if i > 0 {
				paramFilter += ", "
			}
			named := fmt.Sprintf("param%v", i)
			paramFilter += "@" + named
			sqlArgs = append(sqlArgs, sql.Named(named, p))
		}
		paramFilter += ")"
	}
	// Query
	rows, err := d.query("select parameter, value from post_parameters where path = @path"+paramFilter+" order by id", append(sqlArgs, sql.Named("path", path))...)
	if err != nil {
		return nil, err
	}
	// Result
	var name, value string
	params = map[string][]string{}
	for rows.Next() {
		if err = rows.Scan(&name, &value); err != nil {
			return nil, err
		}
		params[name] = append(params[name], value)
	}
	return params, nil
}

func (d *database) getPosts(config *postsRequestConfig) (posts []*post, err error) {
	// Query posts
	query, queryParams := buildPostsQuery(config, "path, coalesce(content, ''), coalesce(published, ''), coalesce(updated, ''), blog, coalesce(section, ''), status, priority")
	rows, err := d.query(query, queryParams...)
	if err != nil {
		return nil, err
	}
	// Prepare row scanning
	var path, content, published, updated, blog, section, status string
	var priority int
	for rows.Next() {
		if err = rows.Scan(&path, &content, &published, &updated, &blog, &section, &status, &priority); err != nil {
			return nil, err
		}
		// Create new post, fill and add to list
		p := &post{
			Path:      path,
			Content:   content,
			Published: toLocalSafe(published),
			Updated:   toLocalSafe(updated),
			Blog:      blog,
			Section:   section,
			Status:    postStatus(status),
			Priority:  priority,
		}
		if !config.withoutParameters {
			if p.Parameters, err = d.getPostParameters(path, config.withOnlyParameters...); err != nil {
				return nil, err
			}
		}
		posts = append(posts, p)
	}
	return posts, nil
}

func (d *database) getPost(path string) (*post, error) {
	posts, err := d.getPosts(&postsRequestConfig{path: path, limit: 1})
	if err != nil {
		return nil, err
	} else if len(posts) == 0 {
		return nil, errPostNotFound
	}
	return posts[0], nil
}

func (d *database) getDrafts(blog string) []*post {
	ps, _ := d.getPosts(&postsRequestConfig{status: statusDraft, blog: blog})
	return ps
}

func (d *database) countPosts(config *postsRequestConfig) (count int, err error) {
	query, params := buildPostsQuery(config, "path")
	row, err := d.queryRow("select count(distinct path) from ("+query+")", params...)
	if err != nil {
		return
	}
	err = row.Scan(&count)
	return
}

func (d *database) allPostPaths(status postStatus) ([]string, error) {
	var postPaths []string
	rows, err := d.query("select path from posts where status = @status", sql.Named("status", status))
	if err != nil {
		return nil, err
	}
	var path string
	for rows.Next() {
		_ = rows.Scan(&path)
		if path != "" {
			postPaths = append(postPaths, path)
		}
	}
	return postPaths, nil
}

func (a *goBlog) getRandomPostPath(blog string) (path string, err error) {
	sections, ok := funk.Keys(a.cfg.Blogs[blog].Sections).([]string)
	if !ok {
		return "", errors.New("no sections")
	}
	query, params := buildPostsQuery(&postsRequestConfig{randomOrder: true, limit: 1, blog: blog, sections: sections}, "path")
	row, err := a.db.queryRow(query, params...)
	if err != nil {
		return
	}
	err = row.Scan(&path)
	if errors.Is(err, sql.ErrNoRows) {
		return "", errPostNotFound
	} else if err != nil {
		return "", err
	}
	return path, nil
}

func (d *database) allTaxonomyValues(blog string, taxonomy string) ([]string, error) {
	var values []string
	rows, err := d.query("select distinct value from post_parameters where parameter = @tax and length(coalesce(value, '')) > 0 and path in (select path from posts where blog = @blog and status = @status) order by value", sql.Named("tax", taxonomy), sql.Named("blog", blog), sql.Named("status", statusPublished))
	if err != nil {
		return nil, err
	}
	var value string
	for rows.Next() {
		if err = rows.Scan(&value); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}

type publishedDate struct {
	year, month, day int
}

func (d *database) allPublishedDates(blog string) (dates []publishedDate, err error) {
	rows, err := d.query("select distinct substr(published, 1, 4) as year, substr(published, 6, 2) as month, substr(published, 9, 2) as day from posts where blog = @blog and status = @status and year != '' and month != '' and day != ''", sql.Named("blog", blog), sql.Named("status", statusPublished))
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

func (db *database) usesOfMediaFile(name string) (count int, err error) {
	query := "select count(distinct path) from (select path from posts_fts where content match @fts_name union all select path from post_parameters where instr(value, @name) > 0)"
	row, err := db.queryRow(query, sql.Named("fts_name", fmt.Sprintf("\"%s\"", name)), sql.Named("name", name))
	if err != nil {
		return 0, err
	}
	err = row.Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}
