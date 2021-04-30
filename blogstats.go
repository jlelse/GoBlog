package main

import (
	"database/sql"
	"net/http"

	servertiming "github.com/mitchellh/go-server-timing"
)

func serveBlogStats(w http.ResponseWriter, r *http.Request) {
	blog := r.Context().Value(blogContextKey).(string)
	// Start timing
	t := servertiming.FromContext(r.Context()).NewMetric("sq").Start()
	// Build query
	prq := &postsRequestConfig{
		blog:   blog,
		status: statusPublished,
	}
	query, params := buildPostsQuery(prq)
	postCount := "coalesce(count(distinct path), 0) as postcount"
	charCount := "coalesce(sum(coalesce(length(distinct content), 0)), 0)"
	wordCount := "coalesce(sum(wordcount(distinct content)), 0) as wordcount"
	wordsPerPost := "coalesce(round(wordcount/postcount,0), 0)"
	type statsTableType struct {
		Name, Posts, Chars, Words, WordsPerPost string
	}
	// Count total posts
	row, err := appDbQueryRow("select *, "+wordsPerPost+" from (select "+postCount+", "+charCount+", "+wordCount+" from ("+query+"))", params...)
	if err != nil {
		serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	total := statsTableType{}
	if err = row.Scan(&total.Posts, &total.Chars, &total.Words, &total.WordsPerPost); err != nil {
		serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	// Count posts per year
	rows, err := appDbQuery("select *, "+wordsPerPost+" from (select substr(published, 1, 4) as year, "+postCount+", "+charCount+", "+wordCount+" from ("+query+") where published != '' group by year order by year desc)", params...)
	if err != nil {
		serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	var years []statsTableType
	year := statsTableType{}
	for rows.Next() {
		if err = rows.Scan(&year.Name, &year.Posts, &year.Chars, &year.Words, &year.WordsPerPost); err == nil {
			years = append(years, year)
		} else {
			serveError(w, r, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	// Count posts without date
	row, err = appDbQueryRow("select *, "+wordsPerPost+" from (select "+postCount+", "+charCount+", "+wordCount+" from ("+query+") where published = '')", params...)
	if err != nil {
		serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	noDate := statsTableType{}
	if err = row.Scan(&noDate.Posts, &noDate.Chars, &noDate.Words, &noDate.WordsPerPost); err != nil {
		serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	// Count posts per month per year
	months := map[string][]statsTableType{}
	month := statsTableType{}
	for _, year := range years {
		rows, err = appDbQuery("select *, "+wordsPerPost+" from (select substr(published, 6, 2) as month, "+postCount+", "+charCount+", "+wordCount+" from ("+query+") where published != '' and substr(published, 1, 4) = @year group by month order by month desc)", append(params, sql.Named("year", year.Name))...)
		if err != nil {
			serveError(w, r, err.Error(), http.StatusInternalServerError)
			return
		}
		for rows.Next() {
			if err = rows.Scan(&month.Name, &month.Posts, &month.Chars, &month.Words, &month.WordsPerPost); err == nil {
				months[year.Name] = append(months[year.Name], month)
			} else {
				serveError(w, r, err.Error(), http.StatusInternalServerError)
				return
			}
		}
	}
	// Stop timing
	t.Stop()
	// Render
	render(w, r, templateBlogStats, &renderData{
		BlogString: blog,
		Canonical:  blogPath(blog) + appConfig.Blogs[blog].BlogStats.Path,
		Data: map[string]interface{}{
			"total":       total,
			"years":       years,
			"withoutdate": noDate,
			"months":      months,
		},
	})
}
