package main

import (
	"fmt"
	"net/http"
	"reflect"
	"strconv"

	"github.com/go-chi/chi"
	"github.com/vcraescu/go-paginator"
)

type webmentionPaginationAdapter struct {
	config *webmentionsRequestConfig
	nums   int64
}

func (p *webmentionPaginationAdapter) Nums() (int64, error) {
	if p.nums == 0 {
		nums, _ := countWebmentions(p.config)
		p.nums = int64(nums)
	}
	return p.nums, nil
}

func (p *webmentionPaginationAdapter) Slice(offset, length int, data interface{}) error {
	modifiedConfig := *p.config
	modifiedConfig.offset = offset
	modifiedConfig.limit = length

	wms, err := getWebmentions(&modifiedConfig)
	reflect.ValueOf(data).Elem().Set(reflect.ValueOf(&wms).Elem())
	return err
}

func webmentionAdmin(w http.ResponseWriter, r *http.Request) {
	pageNoString := chi.URLParam(r, "page")
	pageNo, _ := strconv.Atoi(pageNoString)
	p := paginator.New(&webmentionPaginationAdapter{config: &webmentionsRequestConfig{}}, 10)
	p.SetPage(pageNo)
	var mentions []*mention
	err := p.Results(&mentions)
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
		prevPath = webmentionPath
	} else {
		prevPath = fmt.Sprintf("%s/page/%d", webmentionPath, prevPage)
	}
	hasNext, _ = p.HasNext()
	if hasNext {
		nextPage, _ = p.NextPage()
	} else {
		nextPage, _ = p.Page()
	}
	nextPath = fmt.Sprintf("%s/page/%d", webmentionPath, nextPage)
	// Render
	render(w, r, templateWebmentionAdmin, &renderData{
		Data: map[string]interface{}{
			"Mentions": mentions,
			"HasPrev":  hasPrev,
			"HasNext":  hasNext,
			"Prev":     slashIfEmpty(prevPath),
			"Next":     slashIfEmpty(nextPath),
		},
	})
}

func webmentionAdminDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.FormValue("mentionid"))
	if err != nil {
		serveError(w, r, err.Error(), http.StatusBadRequest)
		return
	}
	err = deleteWebmention(id)
	if err != nil {
		serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	purgeCache()
	http.Redirect(w, r, ".", http.StatusFound)
}

func webmentionAdminApprove(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.FormValue("mentionid"))
	if err != nil {
		serveError(w, r, err.Error(), http.StatusBadRequest)
		return
	}
	err = approveWebmention(id)
	if err != nil {
		serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	purgeCache()
	http.Redirect(w, r, ".", http.StatusFound)
}
