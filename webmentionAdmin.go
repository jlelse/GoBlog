package main

import (
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/vcraescu/go-paginator/v2"
)

type webmentionPaginationAdapter struct {
	config  *webmentionsRequestConfig
	nums    int64
	getNums sync.Once
	db      *database
}

var _ paginator.Adapter = (*webmentionPaginationAdapter)(nil)

func (p *webmentionPaginationAdapter) Nums() (int64, error) {
	p.getNums.Do(func() {
		p.nums = int64(noError(p.db.countWebmentions(p.config)))
	})
	return p.nums, nil
}

func (p *webmentionPaginationAdapter) Slice(offset, length int, data any) error {
	modifiedConfig := *p.config
	modifiedConfig.offset = offset
	modifiedConfig.limit = length

	wms, err := p.db.getWebmentions(&modifiedConfig)
	reflect.ValueOf(data).Elem().Set(reflect.ValueOf(&wms).Elem())
	return err
}

func (a *goBlog) webmentionAdmin(w http.ResponseWriter, r *http.Request) {
	var status webmentionStatus = ""
	switch webmentionStatus(r.URL.Query().Get("status")) {
	case webmentionStatusVerified:
		status = webmentionStatusVerified
	case webmentionStatusApproved:
		status = webmentionStatusApproved
	}
	sourcelike := r.URL.Query().Get("source")
	p := paginator.New(&webmentionPaginationAdapter{config: &webmentionsRequestConfig{
		status:     status,
		sourcelike: sourcelike,
	}, db: a.db}, 5)
	p.SetPage(stringToInt(chi.URLParam(r, "page")))
	var mentions []*mention
	err := p.Results(&mentions)
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	// Navigation
	var hasPrev, hasNext bool
	var prevPage, currentPage, nextPage int
	var prevPath, currentPath, nextPath string
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
	currentPage, _ = p.Page()
	currentPath = fmt.Sprintf("%s/page/%d", webmentionPath, currentPage)
	hasNext, _ = p.HasNext()
	if hasNext {
		nextPage, _ = p.NextPage()
	} else {
		nextPage, _ = p.Page()
	}
	nextPath = fmt.Sprintf("%s/page/%d", webmentionPath, nextPage)
	// Query
	query := ""
	params := url.Values{}
	if status != "" {
		params.Add("status", string(status))
	}
	if sourcelike != "" {
		params.Add("source", sourcelike)
	}
	if len(params) > 0 {
		query = "?" + params.Encode()
	}
	// Render
	a.render(w, r, a.renderWebmentionAdmin, &renderData{
		Data: &webmentionRenderData{
			mentions: mentions,
			hasPrev:  hasPrev,
			hasNext:  hasNext,
			prev:     prevPath + query,
			current:  currentPath + query,
			next:     nextPath + query,
		},
	})
}

func (a *goBlog) webmentionAdminAction(w http.ResponseWriter, r *http.Request) {
	action := chi.URLParam(r, "action")
	if action != "delete" && action != "approve" && action != "reverify" {
		a.serveError(w, r, "Invalid action", http.StatusBadRequest)
		return
	}
	id, err := strconv.Atoi(r.FormValue("mentionid"))
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusBadRequest)
		return
	}
	switch action {
	case "delete":
		err = a.db.deleteWebmentionId(id)
	case "approve":
		err = a.db.approveWebmentionId(id)
	case "reverify":
		err = a.reverifyWebmentionId(id)
	}
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	if action == "delete" || action == "approve" {
		a.cache.purge()
	}
	redirectTo := r.FormValue("redir")
	if redirectTo == "" {
		redirectTo = "."
	}
	http.Redirect(w, r, redirectTo, http.StatusFound)
}
