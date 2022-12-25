package main

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/araddon/dateparse"
	"github.com/samber/lo"
	"go.goblog.app/app/pkgs/bufferpool"
)

func (a *goBlog) checkPost(p *post, new bool) (err error) {
	if p == nil {
		return errors.New("no post")
	}
	now := time.Now().Local()
	nowString := now.Format(time.RFC3339)
	// Maybe add blog
	if p.Blog == "" {
		p.Blog = a.cfg.DefaultBlog
	}
	// Check blog
	if _, ok := a.cfg.Blogs[p.Blog]; !ok {
		return errors.New("blog doesn't exist")
	}
	// Maybe add section
	if p.Path == "" && p.Section == "" {
		// Has no path or section -> default section
		p.Section = a.getBlogFromPost(p).DefaultSection
	}
	// Check section
	if p.Section != "" {
		if _, ok := a.getBlogFromPost(p).Sections[p.Section]; !ok {
			return errors.New("section doesn't exist")
		}
	}
	// Fix and check date strings
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
	// Maybe set published date
	if new && p.Published == "" && p.Section != "" {
		// Has no published date, but section -> published now
		p.Published = nowString
	}
	// Maybe set updated date
	if !new && p.Published != "" {
		if published, err := dateparse.ParseLocal(p.Published); err == nil && now.After(published) {
			// Has published date in the past, so add updated date
			p.Updated = nowString
		}
	}
	// Fix content
	p.Content = strings.TrimSuffix(strings.TrimPrefix(p.Content, "\n"), "\n")
	// Check status
	if p.Status == statusNil {
		p.Status = statusPublished
		if p.Published != "" {
			// If published time is in the future, set status to scheduled
			publishedTime, err := dateparse.ParseLocal(p.Published)
			if err != nil {
				return err
			}
			if publishedTime.After(now) {
				p.Status = statusScheduled
			}
		}
	}
	// Check visibility
	if p.Visibility == visibilityNil {
		p.Visibility = visibilityPublic
	}
	// Cleanup params
	for pk, pvs := range p.Parameters {
		pvs = lo.Filter(pvs, func(s string, _ int) bool { return s != "" })
		if len(pvs) == 0 {
			delete(p.Parameters, pk)
			continue
		}
		p.Parameters[pk] = pvs
	}
	// Automatically add reply title
	if replyLink := p.firstParameter(a.cfg.Micropub.ReplyParam); replyLink != "" && p.firstParameter(a.cfg.Micropub.ReplyTitleParam) == "" &&
		a.getBlogFromPost(p).addReplyTitle {
		// Is reply, but has no reply title
		if mf, err := a.parseMicroformats(replyLink, true); err == nil && mf.Title != "" {
			p.addParameter(a.cfg.Micropub.ReplyTitleParam, mf.Title)
		}
	}
	// Automatically add like title
	if likeLink := p.firstParameter(a.cfg.Micropub.LikeParam); likeLink != "" && p.firstParameter(a.cfg.Micropub.LikeTitleParam) == "" &&
		a.getBlogFromPost(p).addLikeTitle {
		// Is like, but has no like title
		if mf, err := a.parseMicroformats(likeLink, true); err == nil && mf.Title != "" {
			p.addParameter(a.cfg.Micropub.LikeTitleParam, mf.Title)
		}
	}
	// Check path
	if p.Path != "/" {
		p.Path = strings.TrimSuffix(p.Path, "/")
	}
	if p.Path == "" {
		published, parseErr := dateparse.ParseLocal(p.Published)
		if parseErr != nil {
			published = now
		}
		if p.Slug == "" {
			p.Slug = fmt.Sprintf("%v-%02d-%02d-%v", published.Year(), int(published.Month()), published.Day(), randomString(5))
		}
		pathTmplString := defaultIfEmpty(
			a.getBlogFromPost(p).Sections[p.Section].PathTemplate,
			"{{printf \""+a.getRelativePath(p.Blog, "/%v/%02d/%02d/%v")+"\" .Section .Year .Month .Slug}}",
		)
		pathTmpl, err := template.New("location").Parse(pathTmplString)
		if err != nil {
			return errors.New("failed to parse location template")
		}
		pathBuffer := bufferpool.Get()
		defer bufferpool.Put(pathBuffer)
		err = pathTmpl.Execute(pathBuffer, map[string]any{
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

func (a *goBlog) replacePost(p *post, oldPath string, oldStatus postStatus, oldVisibility postVisibility) error {
	return a.createOrReplacePost(p, &postCreationOptions{new: false, oldPath: oldPath, oldStatus: oldStatus, oldVisibility: oldVisibility})
}

type postCreationOptions struct {
	new           bool
	oldPath       string
	oldStatus     postStatus
	oldVisibility postVisibility
}

func (a *goBlog) createOrReplacePost(p *post, o *postCreationOptions) error {
	// Check post
	if err := a.checkPost(p, o.new); err != nil {
		return err
	}
	// Save to db
	if err := a.db.savePost(p, o); err != nil {
		return err
	}
	// Reload post from database
	p, err := a.getPost(p.Path)
	if err != nil {
		// Failed to reload post from database
		return err
	}
	// Trigger hooks
	if p.Status == statusPublished && (p.Visibility == visibilityPublic || p.Visibility == visibilityUnlisted) {
		if o.new || (o.oldStatus != statusPublished && o.oldVisibility != visibilityPublic && o.oldVisibility != visibilityUnlisted) {
			defer a.postPostHooks(p)
		} else {
			defer a.postUpdateHooks(p)
		}
	}
	// Purge cache
	a.cache.purge()
	a.deleteReactionsCache(p.Path)
	return nil
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
	sqlBuilder := bufferpool.Get()
	defer bufferpool.Put(sqlBuilder)
	var sqlArgs = []any{dbNoCache}
	// Start transaction
	sqlBuilder.WriteString("begin;")
	// Update or create post
	if o.new {
		// New post, create it
		sqlBuilder.WriteString("insert into posts (path, content, published, updated, blog, section, status, visibility, priority) values (?, ?, ?, ?, ?, ?, ?, ?, ?);")
		sqlArgs = append(sqlArgs, p.Path, p.Content, toUTCSafe(p.Published), toUTCSafe(p.Updated), p.Blog, p.Section, p.Status, p.Visibility, p.Priority)
	} else {
		// Delete post parameters
		sqlBuilder.WriteString("delete from post_parameters where path = ?;")
		sqlArgs = append(sqlArgs, o.oldPath)
		// Update old post
		sqlBuilder.WriteString("update posts set path = ?, content = ?, published = ?, updated = ?, blog = ?, section = ?, status = ?, visibility = ?, priority = ? where path = ?;")
		sqlArgs = append(sqlArgs, p.Path, p.Content, toUTCSafe(p.Published), toUTCSafe(p.Updated), p.Blog, p.Section, p.Status, p.Visibility, p.Priority, o.oldPath)
	}
	// Insert post parameters
	for param, value := range p.Parameters {
		for _, value := range lo.Filter(value, loStringNotEmpty) {
			sqlBuilder.WriteString("insert into post_parameters (path, parameter, value) values (?, ?, ?);")
			sqlArgs = append(sqlArgs, p.Path, param, value)
		}
	}
	// Commit transaction
	sqlBuilder.WriteString("commit;")
	// Execute
	if _, err := db.Exec(sqlBuilder.String(), sqlArgs...); err != nil {
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
	if path == "" {
		return errors.New("path required")
	}
	// Lock post creation
	a.db.pcm.Lock()
	defer a.db.pcm.Unlock()
	// Check if post exists
	p, err := a.getPost(path)
	if err != nil {
		return err
	}
	// Post exists, check if it's already marked as deleted
	if p.Deleted() {
		// Post is already marked as deleted, delete it from database
		if _, err = a.db.Exec(
			`begin;	delete from posts where path = ?; insert or ignore into deleted (path) values (?); commit;`,
			dbNoCache, p.Path, p.Path, p.Path,
		); err != nil {
			return err
		}
		// Rebuild FTS index
		a.db.rebuildFTSIndex()
		// Purge cache
		a.cache.purge()
		a.deleteReactionsCache(p.Path)
	} else {
		// Update post status
		p.Status = p.Status + statusDeletedSuffix
		// Add parameter
		deletedTime := utcNowString()
		if p.Parameters == nil {
			p.Parameters = map[string][]string{}
		}
		p.Parameters["deleted"] = []string{deletedTime}
		// Mark post as deleted
		if _, err = a.db.Exec(
			`begin;	update posts set status = ? where path = ?; delete from post_parameters where path = ? and parameter = 'deleted'; insert into post_parameters (path, parameter, value) values (?, 'deleted', ?); commit;`,
			dbNoCache, p.Status, p.Path, p.Path, p.Path, deletedTime,
		); err != nil {
			return err
		}
		// Rebuild FTS index
		a.db.rebuildFTSIndex()
		// Purge cache
		a.cache.purge()
		// Trigger hooks
		a.postDeleteHooks(p)
	}
	return nil
}

func (a *goBlog) undeletePost(path string) error {
	if path == "" {
		return errors.New("path required")
	}
	// Lock post creation
	a.db.pcm.Lock()
	defer a.db.pcm.Unlock()
	// Check if post exists
	p, err := a.getPost(path)
	if err != nil {
		return err
	}
	// Post exists, update status and parameters
	p.Status = postStatus(strings.TrimSuffix(string(p.Status), string(statusDeletedSuffix)))
	// Remove parameter
	p.Parameters["deleted"] = nil
	// Update database
	if _, err = a.db.Exec(
		`begin;	update posts set status = ? where path = ?; delete from post_parameters where path = ? and parameter = 'deleted'; commit;`,
		dbNoCache, p.Status, p.Path, p.Path,
	); err != nil {
		return err
	}
	// Rebuild FTS index
	a.db.rebuildFTSIndex()
	// Purge cache
	a.cache.purge()
	// Trigger hooks
	a.postUndeleteHooks(p)
	return nil
}

func (db *database) replacePostParam(path, param string, values []string) error {
	// Filter empty values
	values = lo.Filter(values, loStringNotEmpty)
	// Lock post creation
	db.pcm.Lock()
	defer db.pcm.Unlock()
	// Build SQL
	sqlBuilder := bufferpool.Get()
	var sqlArgs = []any{dbNoCache}
	// Start transaction
	sqlBuilder.WriteString("begin;")
	// Delete old post
	sqlBuilder.WriteString("delete from post_parameters where path = ? and parameter = ?;")
	sqlArgs = append(sqlArgs, path, param)
	// Insert new post parameters
	for _, value := range values {
		sqlBuilder.WriteString("insert into post_parameters (path, parameter, value) values (?, ?, ?);")
		sqlArgs = append(sqlArgs, path, param, value)
	}
	// Commit transaction
	sqlBuilder.WriteString("commit;")
	// Execute
	_, err := db.Exec(sqlBuilder.String(), sqlArgs...)
	bufferpool.Put(sqlBuilder)
	if err != nil {
		return err
	}
	// Update FTS index
	db.rebuildFTSIndex()
	return nil
}

type postsRequestConfig struct {
	search                                      string
	blog                                        string
	path                                        string
	limit                                       int
	offset                                      int
	sections                                    []string
	status                                      []postStatus
	visibility                                  []postVisibility
	taxonomy                                    *configTaxonomy
	taxonomyValue                               string
	parameters                                  []string // Ignores parameterValue
	parameter                                   string   // Ignores parameters
	parameterValue                              string
	excludeParameter                            string // exclude posts that have a certain parameter (with non-empty value)
	excludeParameterValue                       string // ... with exactly this value
	publishedYear, publishedMonth, publishedDay int
	publishedBefore                             time.Time
	randomOrder                                 bool
	priorityOrder                               bool
	withoutParameters                           bool
	withOnlyParameters                          []string
	withoutRenderedTitle                        bool
}

func buildPostsQuery(c *postsRequestConfig, selection string) (query string, args []any) {
	queryBuilder := bufferpool.Get()
	defer bufferpool.Put(queryBuilder)
	// Selection
	queryBuilder.WriteString("select ")
	queryBuilder.WriteString(selection)
	queryBuilder.WriteString(" from ")
	// Table
	if c.search != "" {
		queryBuilder.WriteString("(select p.* from posts_fts(@search) ps, posts p where ps.path = p.path)")
		args = append(args, sql.Named("search", c.search))
	} else {
		queryBuilder.WriteString("posts")
	}
	// Filter
	queryBuilder.WriteString(" where 1")
	if c.path != "" {
		queryBuilder.WriteString(" and path = @path")
		args = append(args, sql.Named("path", c.path))
	}
	if c.status != nil && len(c.status) > 0 {
		queryBuilder.WriteString(" and status in (")
		for i, status := range c.status {
			if i > 0 {
				queryBuilder.WriteString(", ")
			}
			named := "status" + strconv.Itoa(i)
			queryBuilder.WriteByte('@')
			queryBuilder.WriteString(named)
			args = append(args, sql.Named(named, status))
		}
		queryBuilder.WriteByte(')')
	}
	if c.visibility != nil && len(c.visibility) > 0 {
		queryBuilder.WriteString(" and visibility in (")
		for i, visibility := range c.visibility {
			if i > 0 {
				queryBuilder.WriteString(", ")
			}
			named := "visibility" + strconv.Itoa(i)
			queryBuilder.WriteByte('@')
			queryBuilder.WriteString(named)
			args = append(args, sql.Named(named, visibility))
		}
		queryBuilder.WriteByte(')')
	}
	if c.blog != "" {
		queryBuilder.WriteString(" and blog = @blog")
		args = append(args, sql.Named("blog", c.blog))
	}
	if c.parameter != "" {
		if c.parameterValue != "" {
			queryBuilder.WriteString(" and path in (select path from post_parameters where parameter = @param and value = @paramval)")
			args = append(args, sql.Named("param", c.parameter), sql.Named("paramval", c.parameterValue))
		} else {
			queryBuilder.WriteString(" and path in (select path from post_parameters where parameter = @param and length(coalesce(value, '')) > 0)")
			args = append(args, sql.Named("param", c.parameter))
		}
	} else if len(c.parameters) > 0 {
		queryBuilder.WriteString(" and path in (select path from post_parameters where parameter in (")
		for i, param := range c.parameters {
			if i > 0 {
				queryBuilder.WriteString(", ")
			}
			named := "param" + strconv.Itoa(i)
			queryBuilder.WriteByte('@')
			queryBuilder.WriteString(named)
			args = append(args, param)
		}
		queryBuilder.WriteString(") and length(coalesce(value, '')) > 0)")
	}
	if c.excludeParameter != "" {
		if c.excludeParameterValue != "" {
			queryBuilder.WriteString(" and path not in (select path from post_parameters where parameter = @param and value = @paramval)")
			args = append(args, sql.Named("param", c.excludeParameter), sql.Named("paramval", c.excludeParameterValue))
		} else {
			queryBuilder.WriteString(" and path not in (select path from post_parameters where parameter = @param and length(coalesce(value, '')) > 0)")
			args = append(args, sql.Named("param", c.excludeParameter))
		}
	}
	if c.taxonomy != nil && len(c.taxonomyValue) > 0 {
		queryBuilder.WriteString(" and path in (select path from post_parameters where parameter = @taxname and lowerx(value) = lowerx(@taxval))")
		args = append(args, sql.Named("taxname", c.taxonomy.Name), sql.Named("taxval", c.taxonomyValue))
	}
	if len(c.sections) > 0 {
		queryBuilder.WriteString(" and section in (")
		for i, section := range c.sections {
			if i > 0 {
				queryBuilder.WriteString(", ")
			}
			named := "section" + strconv.Itoa(i)
			queryBuilder.WriteByte('@')
			queryBuilder.WriteString(named)
			args = append(args, sql.Named(named, section))
		}
		queryBuilder.WriteByte(')')
	}
	if c.publishedYear != 0 {
		queryBuilder.WriteString(" and substr(tolocal(published), 1, 4) = @publishedyear")
		args = append(args, sql.Named("publishedyear", fmt.Sprintf("%0004d", c.publishedYear)))
	}
	if c.publishedMonth != 0 {
		queryBuilder.WriteString(" and substr(tolocal(published), 6, 2) = @publishedmonth")
		args = append(args, sql.Named("publishedmonth", fmt.Sprintf("%02d", c.publishedMonth)))
	}
	if c.publishedDay != 0 {
		queryBuilder.WriteString(" and substr(tolocal(published), 9, 2) = @publishedday")
		args = append(args, sql.Named("publishedday", fmt.Sprintf("%02d", c.publishedDay)))
	}
	if !c.publishedBefore.IsZero() {
		queryBuilder.WriteString(" and toutc(published) < @publishedbefore")
		args = append(args, sql.Named("publishedbefore", c.publishedBefore.UTC().Format(time.RFC3339)))
	}
	// Order
	queryBuilder.WriteString(" order by ")
	if c.randomOrder {
		queryBuilder.WriteString("random()")
	} else if c.priorityOrder {
		queryBuilder.WriteString("priority desc, published desc")
	} else {
		queryBuilder.WriteString("published desc")
	}
	// Limit & Offset
	if c.limit != 0 || c.offset != 0 {
		queryBuilder.WriteString(" limit @limit offset @offset")
		args = append(args, sql.Named("limit", c.limit), sql.Named("offset", c.offset))
	}
	return queryBuilder.String(), args
}

func (d *database) loadPostParameters(posts []*post, parameters ...string) (err error) {
	if len(posts) == 0 {
		return nil
	}
	// Build query
	sqlArgs := make([]any, 0)
	queryBuilder := bufferpool.Get()
	defer bufferpool.Put(queryBuilder)
	queryBuilder.WriteString("select path, parameter, value from post_parameters where")
	// Paths
	queryBuilder.WriteString(" path in (")
	for i, p := range posts {
		if i > 0 {
			queryBuilder.WriteString(", ")
		}
		named := "path" + strconv.Itoa(i)
		queryBuilder.WriteByte('@')
		queryBuilder.WriteString(named)
		sqlArgs = append(sqlArgs, sql.Named(named, p.Path))
	}
	queryBuilder.WriteByte(')')
	// Parameters
	if len(parameters) > 0 {
		queryBuilder.WriteString(" and parameter in (")
		for i, p := range parameters {
			if i > 0 {
				queryBuilder.WriteString(", ")
			}
			named := "param" + strconv.Itoa(i)
			queryBuilder.WriteByte('@')
			queryBuilder.WriteString(named)
			sqlArgs = append(sqlArgs, sql.Named(named, p))
		}
		queryBuilder.WriteByte(')')
	}
	// Order
	queryBuilder.WriteString(" order by id")
	// Query
	rows, err := d.Query(queryBuilder.String(), sqlArgs...)
	if err != nil {
		return err
	}
	// Result
	var path, name, value string
	params := map[string]map[string][]string{}
	for rows.Next() {
		if err = rows.Scan(&path, &name, &value); err != nil {
			return err
		}
		m, ok := params[path]
		if !ok {
			m = map[string][]string{}
		}
		m[name] = append(m[name], value)
		params[path] = m
	}
	// Add to posts
	for _, p := range posts {
		p.Parameters = params[p.Path]
	}
	return nil
}

func (a *goBlog) getPosts(config *postsRequestConfig) (posts []*post, err error) {
	// Query posts
	query, queryParams := buildPostsQuery(config, "path, coalesce(content, ''), coalesce(published, ''), coalesce(updated, ''), blog, coalesce(section, ''), status, visibility, priority")
	rows, err := a.db.Query(query, queryParams...)
	if err != nil {
		return nil, err
	}
	// Prepare row scanning
	var path, content, published, updated, blog, section, status, visibility string
	var priority int
	for rows.Next() {
		if err = rows.Scan(&path, &content, &published, &updated, &blog, &section, &status, &visibility, &priority); err != nil {
			return nil, err
		}
		// Create new post, fill and add to list
		p := &post{
			Path:       path,
			Content:    content,
			Published:  toLocalSafe(published),
			Updated:    toLocalSafe(updated),
			Blog:       blog,
			Section:    section,
			Status:     postStatus(status),
			Visibility: postVisibility(visibility),
			Priority:   priority,
		}
		posts = append(posts, p)
	}
	if !config.withoutParameters {
		err = a.db.loadPostParameters(posts, config.withOnlyParameters...)
		if err != nil {
			return nil, err
		}
	}
	// Render post title
	if !config.withoutRenderedTitle {
		for _, p := range posts {
			if t := p.Title(); t != "" {
				p.RenderedTitle = a.renderMdTitle(t)
			}
		}
	}
	return posts, nil
}

func (a *goBlog) getPost(path string) (*post, error) {
	posts, err := a.getPosts(&postsRequestConfig{path: path, limit: 1})
	if err != nil {
		return nil, err
	} else if len(posts) == 0 {
		return nil, errPostNotFound
	}
	return posts[0], nil
}

func (d *database) countPosts(config *postsRequestConfig) (count int, err error) {
	query, params := buildPostsQuery(config, "path")
	row, err := d.QueryRow("select count(distinct path) from ("+query+")", params...)
	if err != nil {
		return
	}
	err = row.Scan(&count)
	return
}

func (a *goBlog) getRandomPostPath(blog string) (path string, err error) {
	sections := lo.Keys(a.cfg.Blogs[blog].Sections)
	query, params := buildPostsQuery(&postsRequestConfig{randomOrder: true, limit: 1, blog: blog, sections: sections}, "path")
	row, err := a.db.QueryRow(query, params...)
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
	rows, err := d.Query(
		"select distinct value from post_parameters where parameter = @tax and length(coalesce(value, '')) > 0 and path in (select path from posts where blog = @blog and status = @status and visibility = @visibility) order by value",
		sql.Named("tax", taxonomy), sql.Named("blog", blog), sql.Named("status", statusPublished), sql.Named("visibility", visibilityPublic),
	)
	if err != nil {
		return nil, err
	}
	var values []string
	var value string
	for rows.Next() {
		if err = rows.Scan(&value); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}

const mediaUseSql = `
with mediafiles (name) as (values %s)
select name, count(path) as count from (
    select distinct m.name, p.path
    from mediafiles m, post_parameters p
    where instr(p.value, m.name) > 0
    union
    select distinct m.name, p.path
    from mediafiles m, posts_fts p
    where p.content match '"' || m.name || '"'
)
group by name;
`

func (db *database) usesOfMediaFile(names ...string) (counts []int, err error) {
	sqlArgs := []any{dbNoCache}
	nameValues := bufferpool.Get()
	for i, n := range names {
		if i > 0 {
			nameValues.WriteString(", ")
		}
		named := "name" + strconv.Itoa(i)
		nameValues.WriteString("(@")
		nameValues.WriteString(named)
		nameValues.WriteByte(')')
		sqlArgs = append(sqlArgs, sql.Named(named, n))
	}
	rows, err := db.Query(fmt.Sprintf(mediaUseSql, nameValues.String()), sqlArgs...)
	bufferpool.Put(nameValues)
	if err != nil {
		return nil, err
	}
	counts = make([]int, len(names))
	var name string
	var count int
	for rows.Next() {
		err = rows.Scan(&name, &count)
		if err != nil {
			return nil, err
		}
		for i, n := range names {
			if n == name {
				counts[i] = count
				break
			}
		}
	}
	return counts, nil
}
