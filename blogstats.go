package main

import (
	"net/http"
)

func serveBlogStats(w http.ResponseWriter, r *http.Request) {
	blog := r.Context().Value(blogContextKey).(string)
	// Build query
	query, params := buildPostsQuery(&postsRequestConfig{
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
	if err = row.Scan(&totalCount); err != nil {
		serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	// Count posts per year
	rows, err := appDbQuery("select substr(published, 1, 4) as year, count(distinct path) as count from ("+query+") where published != '' group by year order by year desc", params...)
	if err != nil {
		serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	var years, counts []int
	for rows.Next() {
		var year, count int
		if err = rows.Scan(&year, &count); err == nil {
			years = append(years, year)
			counts = append(counts, count)
		}
	}
	render(w, r, templateBlogStats, &renderData{
		BlogString: blog,
		Canonical:  blogPath(blog) + appConfig.Blogs[blog].BlogStats.Path,
		Data: map[string]interface{}{
			"total":  totalCount,
			"years":  years,
			"counts": counts,
		},
	})
}
