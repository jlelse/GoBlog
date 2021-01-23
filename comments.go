package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"

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
		render(w, templateComment, &renderData{
			BlogString: blog,
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
		// Check comment
		comment := r.FormValue("comment")
		if comment == "" {
			serveError(w, r, "Comment is empty", http.StatusBadRequest)
			return
		}
		name := r.FormValue("name")
		if name == "" {
			name = "Anonymous"
		}
		website := r.FormValue("website")
		// Clean
		strict := bluemonday.StrictPolicy()
		name = strict.Sanitize(name)
		website = strict.Sanitize(website)
		comment = strict.Sanitize(comment)
		// Insert
		result, err := appDbExec("insert into comments (target, comment, name, website) values (@target, @comment, @name, @website)", sql.Named("target", target), sql.Named("comment", comment), sql.Named("name", name), sql.Named("website", website))
		if err != nil {
			serveError(w, r, err.Error(), http.StatusInternalServerError)
			return
		}
		commentID, err := result.LastInsertId()
		commentAddress := fmt.Sprintf("%s/%d", commentsPath, commentID)
		// Send webmention
		createWebmention(appConfig.Server.PublicAddress+commentAddress, appConfig.Server.PublicAddress+target)
		// Redirect to comment
		http.Redirect(w, r, commentAddress, http.StatusFound)
	}
}

func checkCommentTarget(w http.ResponseWriter, r *http.Request) string {
	target := r.FormValue("target")
	if target == "" {
		serveError(w, r, "No target specified", http.StatusBadRequest)
		return ""
	}
	postExists := 0
	row, err := appDbQueryRow("select exists(select 1 from posts where path = @path)", sql.Named("path", target))
	if err != nil {
		serveError(w, r, err.Error(), http.StatusInternalServerError)
		return ""
	}
	if err = row.Scan(&postExists); err != nil {
		serveError(w, r, err.Error(), http.StatusInternalServerError)
		return ""
	}
	if postExists != 1 {
		serveError(w, r, "Post does not exist", http.StatusBadRequest)
		return ""
	}
	return target
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
	rows, err := appDbQuery("select id, target, name, website, comment from comments")
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
	return
}

func deleteComment(id int) error {
	_, err := appDbExec("delete from comments where id = @id", sql.Named("id", id))
	return err
}
