package main

import "net/http"

const taxonomyContextKey = "taxonomy"

func serveTaxonomy(w http.ResponseWriter, r *http.Request) {
	blog := r.Context().Value(blogContextKey).(string)
	tax := r.Context().Value(taxonomyContextKey).(*taxonomy)
	allValues, err := allTaxonomyValues(blog, tax.Name)
	if err != nil {
		serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	render(w, r, templateTaxonomy, &renderData{
		BlogString: blog,
		Canonical:  appConfig.Server.PublicAddress + r.URL.Path,
		Data: map[string]interface{}{
			"Taxonomy":    tax,
			"ValueGroups": groupStrings(allValues),
		},
	})
}
