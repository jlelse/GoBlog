package main

import (
	"fmt"
	"net/http"
	"reflect"
	"strconv"

	"github.com/go-chi/chi"
	"github.com/vcraescu/go-paginator"
)

type commentsPaginationAdapter struct {
	config *commentsRequestConfig
	nums   int64
}

func (p *commentsPaginationAdapter) Nums() (int64, error) {
	if p.nums == 0 {
		nums, _ := countComments(p.config)
		p.nums = int64(nums)
	}
	return p.nums, nil
}

func (p *commentsPaginationAdapter) Slice(offset, length int, data interface{}) error {
	modifiedConfig := *p.config
	modifiedConfig.offset = offset
	modifiedConfig.limit = length

	comments, err := getComments(&modifiedConfig)
	reflect.ValueOf(data).Elem().Set(reflect.ValueOf(&comments).Elem())
	return err
}

func commentsAdmin(blog, commentPath string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// Adapter
		pageNoString := chi.URLParam(r, "page")
		pageNo, _ := strconv.Atoi(pageNoString)
		p := paginator.New(&commentsPaginationAdapter{config: &commentsRequestConfig{}}, 5)
		p.SetPage(pageNo)
		var comments []*comment
		err := p.Results(&comments)
		if err != nil {
			serveError(w, r, err.Error(), http.StatusInternalServerError)
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
			prevPath = commentPath
		} else {
			prevPath = fmt.Sprintf("%s/page/%d", commentPath, prevPage)
		}
		hasNext, _ = p.HasNext()
		if hasNext {
			nextPage, _ = p.NextPage()
		} else {
			nextPage, _ = p.Page()
		}
		nextPath = fmt.Sprintf("%s/page/%d", commentPath, nextPage)
		// Render
		render(w, r, templateCommentsAdmin, &renderData{
			BlogString: blog,
			Data: map[string]interface{}{
				"Comments": comments,
				"HasPrev":  hasPrev,
				"HasNext":  hasNext,
				"Prev":     slashIfEmpty(prevPath),
				"Next":     slashIfEmpty(nextPath),
			},
		})
	}
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
