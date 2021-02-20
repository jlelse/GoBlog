package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi"
	"github.com/microcosm-cc/bluemonday"
)

type comment struct {
	ID      int
	Target  string
	Name    string
	Website string
	Comment string
}

func serveComment(blog string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(chi.URLParam(r, "id"))
		if err != nil {
			serveError(w, r, err.Error(), http.StatusBadRequest)
			return
		}
		row, err := appDbQueryRow("select id, target, name, website, comment from comments where id = @id", sql.Named("id", id))
		if err != nil {
			serveError(w, r, err.Error(), http.StatusInternalServerError)
			return
		}
		comment := &comment{}
		if err = row.Scan(&comment.ID, &comment.Target, &comment.Name, &comment.Website, &comment.Comment); err == sql.ErrNoRows {
			serve404(w, r)
			return
		} else if err != nil {
			serveError(w, r, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("X-Robots-Tag", "noindex")
		render(w, r, templateComment, &renderData{
			BlogString: blog,
			Canonical:  appConfig.Server.PublicAddress + appConfig.Blogs[blog].getRelativePath(fmt.Sprintf("/comment/%d", id)),
			Data:       comment,
		})
	}
}

func createComment(blog, commentsPath string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check target
		target := checkCommentTarget(w, r)
		if target == "" {
			return
		}
		// Check and clean comment
		strict := bluemonday.StrictPolicy()
		comment := strings.TrimSpace(strict.Sanitize(r.FormValue("comment")))
		if comment == "" {
			serveError(w, r, "Comment is empty", http.StatusBadRequest)
			return
		}
		name := strings.TrimSpace(strict.Sanitize(r.FormValue("name")))
		if name == "" {
			name = "Anonymous"
		}
		website := strings.TrimSpace(strict.Sanitize(r.FormValue("website")))
		// Insert
		result, err := appDbExec("insert into comments (target, comment, name, website) values (@target, @comment, @name, @website)", sql.Named("target", target), sql.Named("comment", comment), sql.Named("name", name), sql.Named("website", website))
		if err != nil {
			serveError(w, r, err.Error(), http.StatusInternalServerError)
			return
		}
		if commentID, err := result.LastInsertId(); err != nil {
			// Serve error
			serveError(w, r, err.Error(), http.StatusInternalServerError)
		} else {
			commentAddress := fmt.Sprintf("%s/%d", commentsPath, commentID)
			// Send webmention
			_ = createWebmention(appConfig.Server.PublicAddress+commentAddress, appConfig.Server.PublicAddress+target)
			// Redirect to comment
			http.Redirect(w, r, commentAddress, http.StatusFound)
		}
	}
}

func checkCommentTarget(w http.ResponseWriter, r *http.Request) string {
	target := r.FormValue("target")
	if target == "" {
		serveError(w, r, "No target specified", http.StatusBadRequest)
		return ""
	} else if !strings.HasPrefix(target, appConfig.Server.PublicAddress) {
		serveError(w, r, "Bad target", http.StatusBadRequest)
		return ""
	}
	targetURL, err := url.Parse(target)
	if err != nil {
		serveError(w, r, err.Error(), http.StatusBadRequest)
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

func getComments(config *commentsRequestConfig) ([]*comment, error) {
	comments := []*comment{}
	query, args := buildCommentsQuery(config)
	rows, err := appDbQuery(query, args...)
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

func countComments(config *commentsRequestConfig) (count int, err error) {
	query, params := buildCommentsQuery(config)
	query = "select count(*) from (" + query + ")"
	row, err := appDbQueryRow(query, params...)
	if err != nil {
		return
	}
	err = row.Scan(&count)
	return
}

func deleteComment(id int) error {
	_, err := appDbExec("delete from comments where id = @id", sql.Named("id", id))
	return err
}
