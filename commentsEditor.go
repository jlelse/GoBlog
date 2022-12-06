package main

import (
	"net/http"
	"path"
	"strconv"
)

const commentEditSubPath = "/edit"

func (a *goBlog) serveCommentsEditor(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.FormValue("id"))
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
	blog, bc := a.getBlog(r)
	if r.Method == http.MethodPost {
		name := r.FormValue("name")
		website := r.FormValue("website")
		commentText := r.FormValue("comment")
		if err := a.db.updateComment(id, commentText, name, website); err != nil {
			a.serveError(w, r, err.Error(), http.StatusInternalServerError)
			return
		}
		a.cache.purge()
		// Resend webmention
		commentAddress := bc.getRelativePath(path.Join(commentPath, strconv.Itoa(id)))
		_ = a.createWebmention(a.getFullAddress(commentAddress), a.getFullAddress(comment.Target))
		// Redirect to comment
		http.Redirect(w, r, commentAddress, http.StatusFound)
		return
	}
	a.render(w, r, a.renderCommentEditor, &renderData{
		Data:       comment,
		BlogString: blog,
	})
}
