package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
)

func (a *goBlog) initBlogStats() {
	f := func(p *post) {
		a.db.resetBlogStats(p.Blog)
	}
	a.pPostHooks = append(a.pPostHooks, f)
	a.pUpdateHooks = append(a.pUpdateHooks, f)
	a.pDeleteHooks = append(a.pDeleteHooks, f)
}

func (a *goBlog) serveBlogStats(w http.ResponseWriter, r *http.Request) {
	blog := r.Context().Value(blogContextKey).(string)
	bc := a.cfg.Blogs[blog]
	canonical := bc.getRelativePath(bc.BlogStats.Path)
	a.render(w, r, templateBlogStats, &renderData{
		BlogString: blog,
		Canonical:  a.getFullAddress(canonical),
		Data: map[string]interface{}{
			"TableUrl": canonical + ".table.html",
		},
	})
}

func (a *goBlog) serveBlogStatsTable(w http.ResponseWriter, r *http.Request) {
	blog := r.Context().Value(blogContextKey).(string)
	data, err, _ := a.blogStatsCacheGroup.Do(blog, func() (interface{}, error) {
		return a.db.getBlogStats(blog)
	})
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	// Render
	a.render(w, r, templateBlogStatsTable, &renderData{
		BlogString: blog,
		Data:       data,
	})
}

const blogStatsSql = `
with filtered as (
    select
		path,
		coalesce(published, '') as pub,
		substr(published, 1, 4) as year,
		substr(published, 6, 2) as month,
		wordcount(coalesce(content, '')) as words,
		charcount(coalesce(content, '')) as chars
	from posts
	where status = @status and blog = @blog
)
select *
from (
	select *
	from (
		select
			year,
			'A',
			coalesce(count(path), 0) as pc,
			coalesce(sum(words), 0) as wc,
			coalesce(sum(chars), 0) as cc,
			coalesce(round(sum(words)/count(path), 0), 0) as wpp
		from filtered
		where pub != ''
		group by year
		order by year desc
	)
	union all
	select *
	from (
		select
			year,
			month,
			coalesce(count(path), 0) as pc,
			coalesce(sum(words), 0) as wc,
			coalesce(sum(chars), 0) as cc,
			coalesce(round(sum(words)/count(path), 0), 0) as wpp
		from filtered
		where pub != ''
		group by year, month
		order by year desc, month desc
	)
	union all
	select *
	from (
		select
			'N',
			'N',
			coalesce(count(path), 0) as pc,
			coalesce(sum(words), 0) as wc,
			coalesce(sum(chars), 0) as cc,
			coalesce(round(sum(words)/count(path), 0), 0) as wpp
		from filtered
		where pub == ''
	)
	union all
	select *
	from (
		select
			'A',
			'A',
			coalesce(count(path), 0) as pc,
			coalesce(sum(words), 0) as wc,
			coalesce(sum(chars), 0) as cc,
			coalesce(round(sum(words)/count(path), 0), 0) as wpp
		from filtered
	)
);
`

func (db *database) getBlogStats(blog string) (data map[string]interface{}, err error) {
	// Check cache
	if stats := db.loadBlogStatsCache(blog); stats != nil {
		return stats, nil
	}
	// Prevent creating posts while getting stats
	db.pcm.Lock()
	defer db.pcm.Unlock()
	// Stats type to hold the stats data for a single row
	type statsTableType struct {
		Name, Posts, Chars, Words, WordsPerPost string
	}
	// Scan objects
	currentStats := statsTableType{}
	var currentMonth, currentYear string
	// Data to later return
	var total statsTableType
	var noDate statsTableType
	var years []statsTableType
	months := map[string][]statsTableType{}
	// Query and scan
	rows, err := db.query(blogStatsSql, sql.Named("status", statusPublished), sql.Named("blog", blog))
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		err = rows.Scan(&currentYear, &currentMonth, &currentStats.Posts, &currentStats.Words, &currentStats.Chars, &currentStats.WordsPerPost)
		if currentYear == "A" && currentMonth == "A" {
			total = statsTableType{
				Posts:        currentStats.Posts,
				Words:        currentStats.Words,
				Chars:        currentStats.Chars,
				WordsPerPost: currentStats.WordsPerPost,
			}
		} else if currentYear == "N" && currentMonth == "N" {
			noDate = statsTableType{
				Posts:        currentStats.Posts,
				Words:        currentStats.Words,
				Chars:        currentStats.Chars,
				WordsPerPost: currentStats.WordsPerPost,
			}
		} else if currentMonth == "A" {
			years = append(years, statsTableType{
				Name:         currentYear,
				Posts:        currentStats.Posts,
				Words:        currentStats.Words,
				Chars:        currentStats.Chars,
				WordsPerPost: currentStats.WordsPerPost,
			})
		} else {
			months[currentYear] = append(months[currentYear], statsTableType{
				Name:         currentMonth,
				Posts:        currentStats.Posts,
				Words:        currentStats.Words,
				Chars:        currentStats.Chars,
				WordsPerPost: currentStats.WordsPerPost,
			})
		}
	}
	data = map[string]interface{}{
		"total":       total,
		"years":       years,
		"withoutdate": noDate,
		"months":      months,
	}
	db.cacheBlogStats(blog, data)
	return data, nil
}

func (db *database) cacheBlogStats(blog string, stats map[string]interface{}) {
	jb, _ := json.Marshal(stats)
	_ = db.cachePersistently("blogstats_"+blog, jb)
}

func (db *database) loadBlogStatsCache(blog string) (stats map[string]interface{}) {
	data, err := db.retrievePersistentCache("blogstats_" + blog)
	if err != nil || data == nil {
		return nil
	}
	err = json.Unmarshal(data, &stats)
	if err != nil {
		log.Println(err)
	}
	return stats
}

func (db *database) resetBlogStats(blog string) {
	_ = db.clearPersistentCache("blogstats_" + blog)
}
