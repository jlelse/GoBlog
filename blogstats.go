package main

import (
	"cmp"
	"database/sql"
	"net/http"
)

const defaultBlogStatsPath = "/statistics"

func (a *goBlog) serveBlogStats(w http.ResponseWriter, r *http.Request) {
	blog, bc := a.getBlog(r)
	canonical := bc.getRelativePath(cmp.Or(bc.BlogStats.Path, defaultBlogStatsPath))
	data, err := a.db.getBlogStats(blog)
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	a.render(w, r, a.renderBlogStats, &renderData{
		Canonical: a.getFullAddress(canonical),
		Data:      data,
	})
}

const blogStatsSql = `
with filtered as (
	select
		(CASE WHEN coalesce(pub, '') != '' THEN substr(pub, 1, 4) ELSE 'N' END) as year,
		(CASE WHEN coalesce(pub, '') != '' THEN substr(pub, 6, 2) ELSE 'N' END) as month,
		coalesce(wordcount, 0) as words,
		coalesce(charcount, 0) as chars
	from (
		select tolocal(published) as pub, wordcount, charcount
		from posts
		where status = @status and visibility = @visibility and blog = @blog
	)
), aggregated as (
	select
		year,
		month,
		count(*) as pc,
		coalesce(sum(words), 0) as wc,
		coalesce(sum(chars), 0) as cc,
		coalesce(round(avg(words), 0), 0) as wpp
	from filtered
	group by year, month
)
select * from (
	select year, 'A', sum(pc), sum(wc), sum(cc), round(sum(wc)/sum(pc), 0)
	from aggregated
	where year != 'N'
	group by year
	order by year desc
)
union all
select * from (
	select *
	from aggregated
	where year != 'N'
	order by year desc, month desc
)
union all
select *
from aggregated
where year == 'N'
union all
select 'A', 'A', sum(pc), sum(wc), sum(cc), round(sum(wc)/sum(pc), 0)
from aggregated;
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
	// Prevent creating posts while getting stats
	db.pcm.RLock()
	defer db.pcm.RUnlock()
	// Scan objects
	currentStats := blogStatsRow{}
	var currentMonth, currentYear string
	// Data to later return
	zeroBlogStatsRow := blogStatsRow{Posts: "0", Words: "0", Chars: "0", WordsPerPost: "0"}
	data = &blogStatsData{
		Total:  zeroBlogStatsRow,
		NoDate: zeroBlogStatsRow,
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
		row := blogStatsRow{
			Posts:        cmp.Or(currentStats.Posts, "0"),
			Words:        cmp.Or(currentStats.Words, "0"),
			Chars:        cmp.Or(currentStats.Chars, "0"),
			WordsPerPost: cmp.Or(currentStats.WordsPerPost, "0"),
		}
		if currentYear == "A" && currentMonth == "A" {
			data.Total = row
		} else if currentYear == "N" && currentMonth == "N" {
			data.NoDate = row
		} else if currentMonth == "A" {
			row.Name = currentYear
			data.Years = append(data.Years, row)
		} else {
			row.Name = currentMonth
			data.Months[currentYear] = append(data.Months[currentYear], row)
		}
	}
	return data, nil
}
