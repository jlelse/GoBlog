package main

import "net/http"

func serveTaxonomy(blog string, tax *taxonomy) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		allValues, err := allTaxonomyValues(blog, tax.Name)
		if err != nil {
			serveError(w, r, err.Error(), http.StatusInternalServerError)
			return
		}
		render(w, templateTaxonomy, &renderData{
			BlogString: blog,
			Canonical:  appConfig.Server.PublicAddress + r.URL.Path,
			Data: map[string]interface{}{
				"Taxonomy":    tax,
				"ValueGroups": groupStrings(allValues),
			},
		})
	}
}
