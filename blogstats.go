package main

import (
	"database/sql"
	"net/http"
)

func serveBlogStats(blog, statsPath string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var totalCount int
		row, err := appDbQueryRow("select count(path) as count from posts where blog = @blog", sql.Named("blog", blog))
		if err != nil {
			serveError(w, r, err.Error(), http.StatusInternalServerError)
			return
		}
		err = row.Scan(&totalCount)
		if err != nil {
			serveError(w, r, err.Error(), http.StatusInternalServerError)
			return
		}
		var years, counts []int
		rows, err := appDbQuery("select substr(published, 1, 4) as year, count(path) as count from posts where blog = @blog and coalesce(published, '') != '' group by year order by year desc", sql.Named("blog", blog))
		if err != nil {
			serveError(w, r, err.Error(), http.StatusInternalServerError)
			return
		}
		for rows.Next() {
			var year, count int
			rows.Scan(&year, &count)
			years = append(years, year)
			counts = append(counts, count)
		}
		render(w, templateBlogStats, &renderData{
			blogString: blog,
			Canonical:  statsPath,
			Data: map[string]interface{}{
				"total":  totalCount,
				"years":  years,
				"counts": counts,
			},
		})
	}
}
