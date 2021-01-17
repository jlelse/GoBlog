package main

import (
	"net/http"
)

func serveBlogStats(blog, statsPath string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// Build query
		query, params := buildQuery(&postsRequestConfig{
			blog:   blog,
			status: statusPublished,
		})
		// Count total posts
		row, err := appDbQueryRow("select count(distinct path) from ("+query+")", params...)
		if err != nil {
			serveError(w, r, err.Error(), http.StatusInternalServerError)
			return
		}
		var totalCount int
		err = row.Scan(&totalCount)
		if err != nil {
			serveError(w, r, err.Error(), http.StatusInternalServerError)
			return
		}
		// Count posts per year
		rows, err := appDbQuery(`select substr(published, 1, 4) as year, count(distinct path) as count from (`+query+`) 
									where published != '' group by year order by year desc`, params...)
		if err != nil {
			serveError(w, r, err.Error(), http.StatusInternalServerError)
			return
		}
		var years, counts []int
		for rows.Next() {
			var year, count int
			rows.Scan(&year, &count)
			years = append(years, year)
			counts = append(counts, count)
		}
		render(w, templateBlogStats, &renderData{
			BlogString: blog,
			Canonical:  statsPath,
			Data: map[string]interface{}{
				"total":  totalCount,
				"years":  years,
				"counts": counts,
			},
		})
	}
}
