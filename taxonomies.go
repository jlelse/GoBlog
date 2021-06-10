package main

import "net/http"

const taxonomyContextKey = "taxonomy"

func (a *goBlog) serveTaxonomy(w http.ResponseWriter, r *http.Request) {
	blog := r.Context().Value(blogContextKey).(string)
	tax := r.Context().Value(taxonomyContextKey).(*taxonomy)
	allValues, err := a.db.allTaxonomyValues(blog, tax.Name)
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	a.render(w, r, templateTaxonomy, &renderData{
		BlogString: blog,
		Canonical:  a.getFullAddress(r.URL.Path),
		Data: map[string]interface{}{
			"Taxonomy":    tax,
			"ValueGroups": groupStrings(allValues),
		},
	})
}
