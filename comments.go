package main

import (
	"cmp"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strconv"

	"github.com/go-chi/chi/v5"
	"go.goblog.app/app/pkgs/builderpool"
)

const commentPath = "/comment"

type comment struct {
	ID       int
	Target   string
	Name     string
	Website  string
	Comment  string
	Original string
}

func (a *goBlog) serveComment(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		a.serveError(w, r, "id missing or wrong format", http.StatusBadRequest)
		return
	}
	comments, err := a.db.getComments(&commentsRequestConfig{id: id})
	if err != nil {
		a.serveError(w, r, "failed to query comments from database", http.StatusInternalServerError)
		return
	}
	if len(comments) < 1 {
		a.serve404(w, r)
		return
	}
	comment := comments[0]
	_, bc := a.getBlog(r)
	canonical := a.getFullAddress(bc.getRelativePath(path.Join(commentPath, strconv.Itoa(id))))
	a.render(w, r, a.renderComment, &renderData{
		Canonical: cmp.Or(comment.Original, canonical),
		Data:      comment,
	})
}

func (a *goBlog) createCommentFromRequest(w http.ResponseWriter, r *http.Request) {
	target := r.FormValue("target")   //nolint:gosec
	comment := r.FormValue("comment") //nolint:gosec
	name := r.FormValue("name")       //nolint:gosec
	website := r.FormValue("website") //nolint:gosec
	_, bc := a.getBlog(r)
	// Create comment
	result, errStatus, err := a.createComment(bc, target, comment, name, website, "")
	if err != nil {
		a.serveError(w, r, err.Error(), errStatus)
		return
	}
	// Redirect to comment
	http.Redirect(w, r, result, http.StatusFound)
}

func (a *goBlog) createComment(bc *configBlog, target, comment, name, website, original string) (string, int, error) {
	updateID := -1
	// Check target
	target, status, err := a.checkCommentTarget(target)
	if err != nil {
		return "", status, err
	}
	// Check and clean comment
	comment = cleanHTMLText(comment)
	if comment == "" {
		return "", http.StatusBadRequest, errors.New("comment is empty")
	}
	name = cmp.Or(cleanHTMLText(name), "Anonymous")
	website = cleanHTMLText(website)
	original = cleanHTMLText(original)
	if original != "" {
		// Check if comment already exists
		exists, id, err := a.db.commentIDByOriginal(original)
		if err != nil {
			return "", http.StatusInternalServerError, errors.New("failed to check the database")
		}
		if exists {
			updateID = id
		}
	}
	// Insert
	if updateID == -1 {
		result, err := a.db.Exec(
			"insert into comments (target, comment, name, website, original) values (@target, @comment, @name, @website, @original)",
			sql.Named("target", target), sql.Named("comment", comment), sql.Named("name", name), sql.Named("website", website), sql.Named("original", original),
		)
		if err != nil {
			return "", http.StatusInternalServerError, errors.New("failed to save comment to database")
		}
		commentID, err := result.LastInsertId()
		if err != nil {
			return "", http.StatusInternalServerError, errors.New("failed to save comment to database")
		}
		commentAddress := bc.getRelativePath(fmt.Sprintf("%s/%d", commentPath, commentID))
		// Send webmention
		_ = a.createWebmention(a.getFullAddress(commentAddress), a.getFullAddress(target))
		// Return comment path
		return commentAddress, 0, nil
	}
	if err := a.db.updateComment(updateID, comment, name, website); err != nil {
		return "", http.StatusInternalServerError, errors.New("failed to update comment in database")
	}
	commentAddress := bc.getRelativePath(fmt.Sprintf("%s/%d", commentPath, updateID))
	// Send webmention
	_ = a.createWebmention(a.getFullAddress(commentAddress), a.getFullAddress(target))
	// Return comment path
	return commentAddress, 0, nil
}

func (a *goBlog) checkCommentTarget(target string) (string, int, error) {
	if target == "" {
		return "", http.StatusBadRequest, errors.New("no target specified")
	} else if !a.isLocalURL(target) {
		return "", http.StatusBadRequest, errors.New("bad target")
	}
	targetURL, err := url.Parse(target)
	if err != nil {
		return "", http.StatusBadRequest, errors.New("failed to parse URL")
	}
	return targetURL.Path, 0, nil
}

type commentsRequestConfig struct {
	id, offset, limit int
	target            string
}

func buildCommentsQuery(config *commentsRequestConfig) (query string, args []any) {
	queryBuilder := builderpool.Get()
	defer builderpool.Put(queryBuilder)
	queryBuilder.WriteString("select id, target, name, website, comment, original from comments where 1")
	if config.id != 0 {
		queryBuilder.WriteString(" and id = @id")
		args = append(args, sql.Named("id", config.id))
	}
	if config.target != "" {
		queryBuilder.WriteString(" and target = @target")
		args = append(args, sql.Named("target", config.target))
	}
	queryBuilder.WriteString(" order by id desc")
	if config.limit != 0 || config.offset != 0 {
		queryBuilder.WriteString(" limit @limit offset @offset")
		args = append(args, sql.Named("limit", config.limit), sql.Named("offset", config.offset))
	}
	return queryBuilder.String(), args
}

func (db *database) getComments(config *commentsRequestConfig) ([]*comment, error) {
	var comments []*comment
	query, args := buildCommentsQuery(config)
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		c := &comment{}
		err = rows.Scan(&c.ID, &c.Target, &c.Name, &c.Website, &c.Comment, &c.Original)
		if err != nil {
			return nil, err
		}
		comments = append(comments, c)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return comments, nil
}

func (db *database) countComments(config *commentsRequestConfig) (count int, err error) {
	query, params := buildCommentsQuery(config)
	query = "select count(*) from (" + query + ")"
	row, err := db.QueryRow(query, params...)
	if err != nil {
		return
	}
	err = row.Scan(&count)
	return
}

func (db *database) updateComment(id int, comment, name, website string) error {
	_, err := db.Exec(
		"update comments set comment = @comment, name = @name, website = @website where id = @id",
		sql.Named("comment", comment), sql.Named("name", name), sql.Named("website", website), sql.Named("id", id),
	)
	return err
}

func (db *database) deleteComment(id int) error {
	_, err := db.Exec("delete from comments where id = @id", sql.Named("id", id))
	return err
}

func (db *database) commentIDByOriginal(original string) (bool, int, error) {
	var id int
	row, err := db.QueryRow("select id from comments where original = @original", sql.Named("original", original))
	if err != nil {
		return false, 0, err
	}
	if err := row.Scan(&id); err != nil && errors.Is(err, sql.ErrNoRows) {
		return false, 0, nil
	} else if err != nil {
		return false, 0, err
	}
	return true, id, nil
}

func (app *goBlog) commentsEnabled(blog *configBlog) bool {
	cc := blog.Comments
	wmDisabled := app.cfg.Webmention != nil && app.cfg.Webmention.DisableReceiving
	return cc != nil && cc.Enabled && !wmDisabled
}

const commentsPostParam = "comments"

func (a *goBlog) commentsEnabledForPost(post *post) bool {
	return post != nil && a.commentsEnabled(a.getBlogFromPost(post)) && post.firstParameter(commentsPostParam) != "false"
}
