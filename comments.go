package main

import (
	"database/sql"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

const commentPath = "/comment"

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
	blog := r.Context().Value(blogKey).(string)
	a.render(w, r, templateComment, &renderData{
		BlogString: blog,
		Canonical:  a.getFullAddress(a.getRelativePath(blog, path.Join(commentPath, strconv.Itoa(id)))),
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
	comment := cleanHTMLText(r.FormValue("comment"))
	if comment == "" {
		a.serveError(w, r, "Comment is empty", http.StatusBadRequest)
		return
	}
	name := defaultIfEmpty(cleanHTMLText(r.FormValue("name")), "Anonymous")
	website := cleanHTMLText(r.FormValue("website"))
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
		blog := r.Context().Value(blogKey).(string)
		commentAddress := a.getRelativePath(blog, path.Join(commentPath, strconv.Itoa(int(commentID))))
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
	var queryBuilder strings.Builder
	queryBuilder.WriteString("select id, target, name, website, comment from comments order by id desc")
	if config.limit != 0 || config.offset != 0 {
		queryBuilder.WriteString(" limit @limit offset @offset")
		args = append(args, sql.Named("limit", config.limit), sql.Named("offset", config.offset))
	}
	return queryBuilder.String(), args
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
