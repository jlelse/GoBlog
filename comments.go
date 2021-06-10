package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/microcosm-cc/bluemonday"
)

type comment struct {
	ID      int
	Target  string
	Name    string
	Website string
	Comment string
}

func (a *goBlog) serveComment(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusBadRequest)
		return
	}
	row, err := a.db.queryRow("select id, target, name, website, comment from comments where id = @id", sql.Named("id", id))
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	comment := &comment{}
	if err = row.Scan(&comment.ID, &comment.Target, &comment.Name, &comment.Website, &comment.Comment); err == sql.ErrNoRows {
		a.serve404(w, r)
		return
	} else if err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	blog := r.Context().Value(blogContextKey).(string)
	a.render(w, r, templateComment, &renderData{
		BlogString: blog,
		Canonical:  a.getFullAddress(a.cfg.Blogs[blog].getRelativePath(fmt.Sprintf("/comment/%d", id))),
		Data:       comment,
	})
}

func (a *goBlog) createComment(w http.ResponseWriter, r *http.Request) {
	// Check target
	target := a.checkCommentTarget(w, r)
	if target == "" {
		return
	}
	// Check and clean comment
	strict := bluemonday.StrictPolicy()
	comment := strings.TrimSpace(strict.Sanitize(r.FormValue("comment")))
	if comment == "" {
		a.serveError(w, r, "Comment is empty", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(strict.Sanitize(r.FormValue("name")))
	if name == "" {
		name = "Anonymous"
	}
	website := strings.TrimSpace(strict.Sanitize(r.FormValue("website")))
	// Insert
	result, err := a.db.exec("insert into comments (target, comment, name, website) values (@target, @comment, @name, @website)", sql.Named("target", target), sql.Named("comment", comment), sql.Named("name", name), sql.Named("website", website))
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	if commentID, err := result.LastInsertId(); err != nil {
		// Serve error
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
	} else {
		commentAddress := fmt.Sprintf("%s/%d", a.getRelativePath(r.Context().Value(blogContextKey).(string), "/comment"), commentID)
		// Send webmention
		_ = a.createWebmention(a.getFullAddress(commentAddress), a.getFullAddress(target))
		// Redirect to comment
		http.Redirect(w, r, commentAddress, http.StatusFound)
	}
}

func (a *goBlog) checkCommentTarget(w http.ResponseWriter, r *http.Request) string {
	target := r.FormValue("target")
	if target == "" {
		a.serveError(w, r, "No target specified", http.StatusBadRequest)
		return ""
	} else if !strings.HasPrefix(target, a.cfg.Server.PublicAddress) {
		a.serveError(w, r, "Bad target", http.StatusBadRequest)
		return ""
	}
	targetURL, err := url.Parse(target)
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusBadRequest)
		return ""
	}
	return targetURL.Path
}

type commentsRequestConfig struct {
	offset, limit int
}

func buildCommentsQuery(config *commentsRequestConfig) (query string, args []interface{}) {
	args = []interface{}{}
	query = "select id, target, name, website, comment from comments order by id desc"
	if config.limit != 0 || config.offset != 0 {
		query += " limit @limit offset @offset"
		args = append(args, sql.Named("limit", config.limit), sql.Named("offset", config.offset))
	}
	return
}

func (db *database) getComments(config *commentsRequestConfig) ([]*comment, error) {
	comments := []*comment{}
	query, args := buildCommentsQuery(config)
	rows, err := db.query(query, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		c := &comment{}
		err = rows.Scan(&c.ID, &c.Target, &c.Name, &c.Website, &c.Comment)
		if err != nil {
			return nil, err
		}
		comments = append(comments, c)
	}
	return comments, nil
}

func (db *database) countComments(config *commentsRequestConfig) (count int, err error) {
	query, params := buildCommentsQuery(config)
	query = "select count(*) from (" + query + ")"
	row, err := db.queryRow(query, params...)
	if err != nil {
		return
	}
	err = row.Scan(&count)
	return
}

func (db *database) deleteComment(id int) error {
	_, err := db.exec("delete from comments where id = @id", sql.Named("id", id))
	return err
}
