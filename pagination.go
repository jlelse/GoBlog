package main

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/vcraescu/go-paginator/v2"
)

type pagination struct {
	page, total  int
	first, query string
}

func readPage(r *http.Request, p paginator.Paginator, first string, results any) (pagination, error) {
	p.SetPage(stringToInt(chi.URLParam(r, "page")))
	nav := pagination{first: first}
	var err error
	if nav.page, err = p.Page(); err != nil {
		return nav, err
	}
	if nav.total, err = p.PageNums(); err != nil {
		return nav, err
	}
	return nav, p.Results(results)
}

func (p pagination) url(page int) string {
	if page == 1 {
		return p.first + p.query
	}
	return fmt.Sprintf("%s/page/%d%s", strings.TrimSuffix(p.first, "/"), page, p.query)
}
