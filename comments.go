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
		render(w, templateComment, &renderData{
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

func commentsAdmin(w http.ResponseWriter, r *http.Request) {
	comments, err := getComments()
	if err != nil {
		serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	render(w, templateCommentsAdmin, &renderData{
		Data: comments,
	})
}

func getComments() ([]*comment, error) {
	comments := []*comment{}
	rows, err := appDbQuery("select id, target, name, website, comment from comments order by id desc")
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

func commentsAdminDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.FormValue("commentid"))
	if err != nil {
		serveError(w, r, err.Error(), http.StatusBadRequest)
		return
	}
	err = deleteComment(id)
	if err != nil {
		serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	purgeCache()
	http.Redirect(w, r, ".", http.StatusFound)
}

func deleteComment(id int) error {
	_, err := appDbExec("delete from comments where id = @id", sql.Named("id", id))
	return err
}
