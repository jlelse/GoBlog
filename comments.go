package main

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"

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
		Canonical: defaultIfEmpty(comment.Original, canonical),
		Data:      comment,
	})
}

func (a *goBlog) createCommentFromRequest(w http.ResponseWriter, r *http.Request) {
	target := r.FormValue("target")
	comment := r.FormValue("comment")
	name := r.FormValue("name")
	website := r.FormValue("website")
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
	updateId := -1
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
	name = defaultIfEmpty(cleanHTMLText(name), "Anonymous")
	website = cleanHTMLText(website)
	original = cleanHTMLText(original)
	if original != "" {
		// Check if comment already exists
		exists, id, err := a.db.commentIdByOriginal(original)
		if err != nil {
			return "", http.StatusInternalServerError, errors.New("failed to check the database")
		}
		if exists {
			updateId = id
		}
	}
	// Insert
	if updateId == -1 {
		result, err := a.db.Exec(
			"insert into comments (target, comment, name, website, original) values (@target, @comment, @name, @website, @original)",
			sql.Named("target", target), sql.Named("comment", comment), sql.Named("name", name), sql.Named("website", website), sql.Named("original", original),
		)
		if err != nil {
			return "", http.StatusInternalServerError, errors.New("failed to save comment to database")
		}
		if commentID, err := result.LastInsertId(); err != nil {
			return "", http.StatusInternalServerError, errors.New("failed to save comment to database")
		} else {
			commentAddress := bc.getRelativePath(fmt.Sprintf("%s/%d", commentPath, commentID))
			// Send webmention
			_ = a.createWebmention(a.getFullAddress(commentAddress), a.getFullAddress(target))
			// Return comment path
			return commentAddress, 0, nil
		}
	} else {
		if err := a.db.updateComment(updateId, comment, name, website); err != nil {
			return "", http.StatusInternalServerError, errors.New("failed to update comment in database")
		}
		commentAddress := bc.getRelativePath(fmt.Sprintf("%s/%d", commentPath, updateId))
		// Send webmention
		_ = a.createWebmention(a.getFullAddress(commentAddress), a.getFullAddress(target))
		// Return comment path
		return commentAddress, 0, nil
	}
}

func (a *goBlog) checkCommentTarget(target string) (string, int, error) {
	if target == "" {
		return "", http.StatusBadRequest, errors.New("no target specified")
	} else if !strings.HasPrefix(target, a.cfg.Server.PublicAddress) {
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
}

func buildCommentsQuery(config *commentsRequestConfig) (query string, args []any) {
	queryBuilder := builderpool.Get()
	defer builderpool.Put(queryBuilder)
	queryBuilder.WriteString("select id, target, name, website, comment, original from comments")
	if config.id != 0 {
		queryBuilder.WriteString(" where id = @id")
		args = append(args, sql.Named("id", config.id))
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
	for rows.Next() {
		c := &comment{}
		err = rows.Scan(&c.ID, &c.Target, &c.Name, &c.Website, &c.Comment, &c.Original)
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

func (db *database) commentIdByOriginal(original string) (bool, int, error) {
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

func (blog *configBlog) commentsEnabled() bool {
	return blog.Comments != nil && blog.Comments.Enabled
}

const commentsPostParam = "comments"

func (a *goBlog) commentsEnabledForPost(post *post) bool {
	return post != nil && a.getBlogFromPost(post).commentsEnabled() && post.firstParameter(commentsPostParam) != "false"
}
