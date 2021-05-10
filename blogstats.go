package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	"golang.org/x/sync/singleflight"
)

func initBlogStats() {
	f := func(p *post) {
		resetBlogStats(p.Blog)
	}
	postHooks[postPostHook] = append(postHooks[postPostHook], f)
	postHooks[postUpdateHook] = append(postHooks[postPostHook], f)
	postHooks[postDeleteHook] = append(postHooks[postPostHook], f)
}

func serveBlogStats(w http.ResponseWriter, r *http.Request) {
	blog := r.Context().Value(blogContextKey).(string)
	canonical := blogPath(blog) + appConfig.Blogs[blog].BlogStats.Path
	render(w, r, templateBlogStats, &renderData{
		BlogString: blog,
		Canonical:  canonical,
		Data: map[string]interface{}{
			"TableUrl": canonical + ".table.html",
		},
	})
}

var blogStatsCacheGroup singleflight.Group

func serveBlogStatsTable(w http.ResponseWriter, r *http.Request) {
	blog := r.Context().Value(blogContextKey).(string)
	data, err, _ := blogStatsCacheGroup.Do(blog, func() (interface{}, error) {
		return getBlogStats(blog)
	})
	if err != nil {
		serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	// Render
	render(w, r, templateBlogStatsTable, &renderData{
		BlogString: blog,
		Data:       data,
	})
}

func getBlogStats(blog string) (data map[string]interface{}, err error) {
	if stats := loadBlogStatsCache(blog); stats != nil {
		return stats, nil
	}
	// Build query
	prq := &postsRequestConfig{
		blog:   blog,
		status: statusPublished,
	}
	query, params := buildPostsQuery(prq)
	query = "select path, mdtext(content) as content, published, substr(published, 1, 4) as year, substr(published, 6, 2) as month from (" + query + ")"
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
		return nil, err
	}
	total := statsTableType{}
	if err = row.Scan(&total.Posts, &total.Chars, &total.Words, &total.WordsPerPost); err != nil {
		return nil, err
	}
	// Count posts per year
	rows, err := appDbQuery("select *, "+wordsPerPost+" from (select year, "+postCount+", "+charCount+", "+wordCount+" from ("+query+") where published != '' group by year order by year desc)", params...)
	if err != nil {
		return nil, err
	}
	var years []statsTableType
	year := statsTableType{}
	for rows.Next() {
		if err = rows.Scan(&year.Name, &year.Posts, &year.Chars, &year.Words, &year.WordsPerPost); err == nil {
			years = append(years, year)
		} else {
			return nil, err
		}
	}
	// Count posts without date
	row, err = appDbQueryRow("select *, "+wordsPerPost+" from (select "+postCount+", "+charCount+", "+wordCount+" from ("+query+") where published = '')", params...)
	if err != nil {
		return nil, err
	}
	noDate := statsTableType{}
	if err = row.Scan(&noDate.Posts, &noDate.Chars, &noDate.Words, &noDate.WordsPerPost); err != nil {
		return nil, err
	}
	// Count posts per month per year
	months := map[string][]statsTableType{}
	month := statsTableType{}
	for _, year := range years {
		rows, err = appDbQuery("select *, "+wordsPerPost+" from (select month, "+postCount+", "+charCount+", "+wordCount+" from ("+query+") where published != '' and year = @year group by month order by month desc)", append(params, sql.Named("year", year.Name))...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			if err = rows.Scan(&month.Name, &month.Posts, &month.Chars, &month.Words, &month.WordsPerPost); err == nil {
				months[year.Name] = append(months[year.Name], month)
			} else {
				return nil, err
			}
		}
	}
	data = map[string]interface{}{
		"total":       total,
		"years":       years,
		"withoutdate": noDate,
		"months":      months,
	}
	cacheBlogStats(blog, data)
	return data, nil
}

func cacheBlogStats(blog string, stats map[string]interface{}) {
	jb, _ := json.Marshal(stats)
	cachePersistently("blogstats_"+blog, jb)
}

func loadBlogStatsCache(blog string) (stats map[string]interface{}) {
	data, err := retrievePersistentCache("blogstats_" + blog)
	if err != nil || data == nil {
		return nil
	}
	err = json.Unmarshal(data, &stats)
	if err != nil {
		log.Println(err)
	}
	return stats
}

func resetBlogStats(blog string) {
	clearPersistentCache("blogstats_" + blog)
}
