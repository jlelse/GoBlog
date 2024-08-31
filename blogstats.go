package main

import (
	"bytes"
	"cmp"
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
	canonical := bc.getRelativePath(cmp.Or(bc.BlogStats.Path, defaultBlogStatsPath))
	a.render(w, r, a.renderBlogStats, &renderData{
		Canonical: a.getFullAddress(canonical),
		Data: &blogStatsRenderData{
			tableUrl: canonical + blogStatsTablePath,
		},
	})
}

func (a *goBlog) serveBlogStatsTable(w http.ResponseWriter, r *http.Request) {
	blog, _ := a.getBlog(r)
	data, err, _ := a.blogStatsCacheGroup.Do(blog, func() (*blogStatsData, error) {
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
		(CASE WHEN coalesce(pub, '') != '' THEN substr(pub, 1, 4) ELSE 'N' END) as year,
		(CASE WHEN coalesce(pub, '') != '' THEN substr(pub, 6, 2) ELSE 'N' END) as month,
		wordcount(content) as words,
		charcount(content) as chars
	from (
		select
			tolocal(published) as pub,
			mdtext(coalesce(content, '')) as content
		from posts
		where status = @status and visibility = @visibility and blog = @blog
	)
), aggregated as (
	select
        year,
        month,
        coalesce(count(*), 0) as pc,
        coalesce(sum(words), 0) as wc,
        coalesce(sum(chars), 0) as cc,
        coalesce(round(avg(words), 0), 0) as wpp
    from filtered
    group by year, month
)
select *
from (
	select *
	from (
		select year, 'A', sum(pc), sum(wc), sum(cc), round(sum(wc)/sum(pc), 0)
		from aggregated
		where year != 'N'
		group by year
		order by year desc
	)
	union all
	select *
	from (
		select *
		from aggregated
		where year != 'N'
		order by year desc, month desc
	)
	union all
	select *
	from (
		select *
		from aggregated
		where year == 'N'
	)
	union all
	select *
	from (
		select 'A', 'A', sum(pc), sum(wc), sum(cc), round(sum(wc)/sum(pc), 0)
		from aggregated
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
	db.pcm.RLock()
	defer db.pcm.RUnlock()
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
	defer rows.Close()
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
