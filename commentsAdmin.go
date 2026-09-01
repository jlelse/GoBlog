package main

import (
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/vcraescu/go-paginator/v2"
)

type commentsPaginationAdapter struct {
	config  *commentsRequestConfig
	getNums func() (int64, error)
	db      *database
}

func newCommentsPaginationAdapter(config *commentsRequestConfig, db *database) *commentsPaginationAdapter {
	p := &commentsPaginationAdapter{config: config, db: db}
	p.getNums = sync.OnceValues(func() (int64, error) {
		nums, err := p.db.countComments(p.config)
		return int64(nums), err
	})
	return p
}

func (p *commentsPaginationAdapter) Nums() (int64, error) {
	return p.getNums()
}

func (p *commentsPaginationAdapter) Slice(offset, length int, data any) error {
	modifiedConfig := *p.config
	modifiedConfig.offset = offset
	modifiedConfig.limit = length

	comments, err := p.db.getComments(&modifiedConfig)
	reflect.ValueOf(data).Elem().Set(reflect.ValueOf(&comments).Elem())
	return err
}

func (a *goBlog) commentsAdmin(w http.ResponseWriter, r *http.Request) {
	commentsPath := r.Context().Value(pathKey).(string)
	// Adapter
	p := paginator.New(newCommentsPaginationAdapter(&commentsRequestConfig{}, a.db), 5)
	p.SetPage(stringToInt(chi.URLParam(r, "page")))
	var comments []*comment
	err := p.Results(&comments)
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	// Navigation
	var hasPrev, hasNext bool
	var prevPage, nextPage int
	var prevPath, nextPath string
	hasPrev, _ = p.HasPrev()
	if hasPrev {
		prevPage, _ = p.PrevPage()
	} else {
		prevPage, _ = p.Page()
	}
	if prevPage < 2 {
		prevPath = commentsPath
	} else {
		prevPath = fmt.Sprintf("%s/page/%d", commentsPath, prevPage)
	}
	hasNext, _ = p.HasNext()
	if hasNext {
		nextPage, _ = p.NextPage()
	} else {
		nextPage, _ = p.Page()
	}
	nextPath = fmt.Sprintf("%s/page/%d", commentsPath, nextPage)
	// Render
	a.render(w, r, a.renderCommentsAdmin, &renderData{
		Data: &commentsRenderData{
			comments: comments,
			hasPrev:  hasPrev,
			hasNext:  hasNext,
			prev:     prevPath,
			next:     nextPath,
		},
	})
}

const commentDeleteSubPath = "/delete"

func (a *goBlog) commentsAdminDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.FormValue("commentid")) //nolint:gosec
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusBadRequest)
		return
	}
	err = a.db.deleteComment(id)
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	a.purgeCache()
	http.Redirect(w, r, ".", http.StatusFound)
}
