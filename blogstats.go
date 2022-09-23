package main

import (
	"bytes"
	"database/sql"
	"encoding/gob"
	"net/http"

	"go.goblog.app/app/pkgs/bufferpool"
)

const (
	defaultBlogStatsPath = "/statistics"
	blogStatsTablePath   = ".table.html"
)

func (a *goBlog) initBlogStats() {
	f := func(p *post) {
		a.db.resetBlogStats(p.Blog)
	}
	a.pPostHooks = append(a.pPostHooks, f)
	a.pUpdateHooks = append(a.pUpdateHooks, f)
	a.pDeleteHooks = append(a.pDeleteHooks, f)
	a.pUndeleteHooks = append(a.pUndeleteHooks, f)
}

func (a *goBlog) serveBlogStats(w http.ResponseWriter, r *http.Request) {
	_, bc := a.getBlog(r)
	canonical := bc.getRelativePath(defaultIfEmpty(bc.BlogStats.Path, defaultBlogStatsPath))
	a.render(w, r, a.renderBlogStats, &renderData{
		Canonical: a.getFullAddress(canonical),
		Data: &blogStatsRenderData{
			tableUrl: canonical + blogStatsTablePath,
		},
	})
}

func (a *goBlog) serveBlogStatsTable(w http.ResponseWriter, r *http.Request) {
	blog, _ := a.getBlog(r)
	data, err, _ := a.blogStatsCacheGroup.Do(blog, func() (any, error) {
		return a.db.getBlogStats(blog)
	})
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	// Render
	a.render(w, r, a.renderBlogStatsTable, &renderData{
		Data: data,
	})
}

const blogStatsSql = `
with filtered as (
    select
		path,
		pub,
		substr(pub, 1, 4) as year,
		substr(pub, 6, 2) as month,
		wordcount(content) as words,
		charcount(content) as chars
	from (
		select
			path,
			tolocal(published) as pub,
			mdtext(coalesce(content, '')) as content
		from posts
		where status = @status and visibility = @visibility and blog = @blog
	)
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

type blogStatsRow struct {
	Name, Posts, Chars, Words, WordsPerPost string
}

type blogStatsData struct {
	Total  blogStatsRow
	NoDate blogStatsRow
	Years  []blogStatsRow
	Months map[string][]blogStatsRow
}

func (db *database) getBlogStats(blog string) (data *blogStatsData, err error) {
	// Check cache
	if stats := db.loadBlogStatsCache(blog); stats != nil {
		return stats, nil
	}
	// Prevent creating posts while getting stats
	db.pcm.Lock()
	defer db.pcm.Unlock()
	// Scan objects
	currentStats := blogStatsRow{}
	var currentMonth, currentYear string
	// Data to later return
	data = &blogStatsData{
		Months: map[string][]blogStatsRow{},
	}
	// Query and scan
	rows, err := db.Query(blogStatsSql, sql.Named("status", statusPublished), sql.Named("visibility", visibilityPublic), sql.Named("blog", blog))
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		err = rows.Scan(&currentYear, &currentMonth, &currentStats.Posts, &currentStats.Words, &currentStats.Chars, &currentStats.WordsPerPost)
		if currentYear == "A" && currentMonth == "A" {
			data.Total = blogStatsRow{
				Posts:        currentStats.Posts,
				Words:        currentStats.Words,
				Chars:        currentStats.Chars,
				WordsPerPost: currentStats.WordsPerPost,
			}
		} else if currentYear == "N" && currentMonth == "N" {
			data.NoDate = blogStatsRow{
				Posts:        currentStats.Posts,
				Words:        currentStats.Words,
				Chars:        currentStats.Chars,
				WordsPerPost: currentStats.WordsPerPost,
			}
		} else if currentMonth == "A" {
			data.Years = append(data.Years, blogStatsRow{
				Name:         currentYear,
				Posts:        currentStats.Posts,
				Words:        currentStats.Words,
				Chars:        currentStats.Chars,
				WordsPerPost: currentStats.WordsPerPost,
			})
		} else {
			data.Months[currentYear] = append(data.Months[currentYear], blogStatsRow{
				Name:         currentMonth,
				Posts:        currentStats.Posts,
				Words:        currentStats.Words,
				Chars:        currentStats.Chars,
				WordsPerPost: currentStats.WordsPerPost,
			})
		}
	}
	db.cacheBlogStats(blog, data)
	return data, nil
}

func (db *database) cacheBlogStats(blog string, stats *blogStatsData) {
	buf := bufferpool.Get()
	_ = gob.NewEncoder(buf).Encode(stats)
	_ = db.cachePersistently("blogstats_"+blog, buf.Bytes())
	bufferpool.Put(buf)
}

func (db *database) loadBlogStatsCache(blog string) (stats *blogStatsData) {
	data, err := db.retrievePersistentCache("blogstats_" + blog)
	if err != nil || data == nil {
		return nil
	}
	stats = &blogStatsData{}
	err = gob.NewDecoder(bytes.NewReader(data)).Decode(stats)
	if err != nil {
		return nil
	}
	return stats
}

func (db *database) resetBlogStats(blog string) {
	_ = db.clearPersistentCache("blogstats_" + blog)
}
