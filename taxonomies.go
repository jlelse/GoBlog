package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
)

const taxonomyContextKey = "taxonomy"

func (a *goBlog) serveTaxonomy(w http.ResponseWriter, r *http.Request) {
	blog, _ := a.getBlog(r)
	tax := r.Context().Value(taxonomyContextKey).(*configTaxonomy)
	allValues, err := a.db.allTaxonomyValues(blog, tax.Name)
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	a.render(w, r, a.renderTaxonomy, &renderData{
		Canonical: a.getFullAddress(r.URL.Path),
		Data: &taxonomyRenderData{
			taxonomy:    tax,
			valueGroups: groupStrings(allValues),
		},
	})
}

func (a *goBlog) serveTaxonomyValue(w http.ResponseWriter, r *http.Request) {
	_, bc := a.getBlog(r)
	tax := r.Context().Value(taxonomyContextKey).(*configTaxonomy)
	taxValueParam := chi.URLParam(r, "taxValue")
	if taxValueParam == "" {
		a.serve404(w, r)
		return
	}
	// Get value from DB
	row, err := a.db.QueryRow(
		"select value from post_parameters where parameter = @tax and urlize(value) = @taxValue limit 1",
		sql.Named("tax", tax.Name), sql.Named("taxValue", taxValueParam),
	)
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	var taxValue string
	err = row.Scan(&taxValue)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			a.serve404(w, r)
			return
		}
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	// Serve index
	a.serveIndex(w, r.WithContext(context.WithValue(r.Context(), indexConfigKey, &indexConfig{
		path:     bc.getRelativePath(fmt.Sprintf("/%s/%s", tax.Name, taxValueParam)),
		tax:      tax,
		taxValue: taxValue,
	})))
}
