package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"net/http"

	"github.com/go-chi/chi/v5"
)

const taxonomyContextKey = "taxonomy"

func taxonomyWeight(count, least, most int) int {
	if least == most {
		return 0
	}
	// A square-root scale keeps mid-frequency tags distinct even with a very popular outlier.
	low, high := math.Sqrt(float64(least)), math.Sqrt(float64(most))
	return int(math.Round(6 * (math.Sqrt(float64(count)) - low) / (high - low)))
}

func (a *goBlog) serveTaxonomy(w http.ResponseWriter, r *http.Request) {
	blog, _ := a.getBlog(r)
	tax := r.Context().Value(taxonomyContextKey).(*configTaxonomy)
	values, err := a.db.taxonomyCounts(blog, tax.Name)
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	a.render(w, r, a.renderTaxonomy, &renderData{
		Canonical: a.getFullAddress(r.URL.Path),
		Data: &taxonomyRenderData{
			taxonomy: tax,
			values:   values,
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
