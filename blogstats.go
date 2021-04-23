package main

import (
	"net/http"
	"strconv"
)

func serveBlogStats(w http.ResponseWriter, r *http.Request) {
	blog := r.Context().Value(blogContextKey).(string)
	// Build query
	prq := &postsRequestConfig{
		blog:   blog,
		status: statusPublished,
	}
	query, params := buildPostsQuery(prq)
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
	var years []stringPair
	for rows.Next() {
		var year, count string
		if err = rows.Scan(&year, &count); err == nil {
			years = append(years, stringPair{year, count})
		} else {
			serveError(w, r, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	// Count posts without date
	row, err = appDbQueryRow("select count(distinct path) from ("+query+") where published = ''", params...)
	if err != nil {
		serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	var noDateCount int
	if err = row.Scan(&noDateCount); err != nil {
		serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	// Count posts per month per year
	months := map[string][]stringPair{}
	for _, year := range years {
		prq.publishedYear, _ = strconv.Atoi(year.First)
		query, params = buildPostsQuery(prq)
		rows, err = appDbQuery("select substr(published, 6, 2) as month, count(distinct path) as count from ("+query+") where published != '' group by month order by month desc", params...)
		if err != nil {
			serveError(w, r, err.Error(), http.StatusInternalServerError)
			return
		}
		for rows.Next() {
			var month, count string
			if err = rows.Scan(&month, &count); err == nil {
				months[year.First] = append(months[year.First], stringPair{month, count})
			} else {
				serveError(w, r, err.Error(), http.StatusInternalServerError)
				return
			}
		}
	}
	render(w, r, templateBlogStats, &renderData{
		BlogString: blog,
		Canonical:  blogPath(blog) + appConfig.Blogs[blog].BlogStats.Path,
		Data: map[string]interface{}{
			"total":       totalCount,
			"years":       years,
			"withoutdate": noDateCount,
			"months":      months,
		},
	})
}
