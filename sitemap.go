package main

import (
	"cmp"
	"database/sql"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"path"
	"time"

	"github.com/snabb/sitemap"
	"go.goblog.app/app/pkgs/contenttype"
)

const (
	sitemapPath             = "/sitemap.xml"
	sitemapBlogPath         = "/sitemap-blog.xml"
	sitemapBlogFeaturesPath = "/sitemap-blog-features.xml"
	sitemapBlogArchivesPath = "/sitemap-blog-archives.xml"
	sitemapBlogPostsPath    = "/sitemap-blog-posts.xml"
)

func (a *goBlog) serveSitemap(w http.ResponseWriter, r *http.Request) {
	// Create sitemap
	sm := sitemap.NewSitemapIndex()
	// Add blog sitemap indices
	now := time.Now().UTC()
	for _, bc := range a.cfg.Blogs {
		sm.Add(&sitemap.URL{
			Loc:     a.getFullAddress(bc.getRelativePath(sitemapBlogPath)),
			LastMod: &now,
		})
	}
	// Write sitemap
	a.writeSitemapXML(w, r, sm)
}

func (a *goBlog) serveSitemapBlog(w http.ResponseWriter, r *http.Request) {
	// Create sitemap
	sm := sitemap.NewSitemapIndex()
	// Add blog sitemaps
	_, bc := a.getBlog(r)
	now := time.Now().UTC()
	sm.Add(&sitemap.URL{
		Loc:     a.getFullAddress(bc.getRelativePath(sitemapBlogFeaturesPath)),
		LastMod: &now,
	})
	sm.Add(&sitemap.URL{
		Loc:     a.getFullAddress(bc.getRelativePath(sitemapBlogArchivesPath)),
		LastMod: &now,
	})
	sm.Add(&sitemap.URL{
		Loc:     a.getFullAddress(bc.getRelativePath(sitemapBlogPostsPath)),
		LastMod: &now,
	})
	// Write sitemap
	a.writeSitemapXML(w, r, sm)
}

func (a *goBlog) serveSitemapBlogFeatures(w http.ResponseWriter, r *http.Request) {
	// Create sitemap
	sm := sitemap.New()
	// Add features to sitemap
	blog, bc := a.getBlog(r)
	// Home: latest post in blog
	blogLastMod := a.sitemapLastMod(&postsRequestConfig{blogs: []string{blog}})
	sm.Add(&sitemap.URL{
		Loc:     a.getFullAddress(bc.getRelativePath("")),
		LastMod: blogLastMod,
	})
	// Photos
	if pc := bc.Photos; pc != nil && pc.Enabled {
		sm.Add(&sitemap.URL{
			Loc: a.getFullAddress(bc.getRelativePath(cmp.Or(pc.Path, defaultPhotosPath))),
			LastMod: a.sitemapLastMod(&postsRequestConfig{
				blogs:     []string{blog},
				parameter: a.cfg.Micropub.PhotoParam,
			}),
		})
	}
	// Search
	if bsc := bc.Search; bsc != nil && bsc.Enabled {
		sm.Add(&sitemap.URL{
			Loc: a.getFullAddress(bc.getRelativePath(cmp.Or(bsc.Path, defaultSearchPath))),
		})
	}
	// Stats
	if bsc := bc.BlogStats; bsc != nil && bsc.Enabled {
		sm.Add(&sitemap.URL{
			Loc:     a.getFullAddress(bc.getRelativePath(cmp.Or(bsc.Path, defaultBlogStatsPath))),
			LastMod: blogLastMod,
		})
	}
	// Blogroll
	if blogrollEnabled, blogrollPath := bc.getBlogrollPath(); blogrollEnabled {
		sm.Add(&sitemap.URL{
			Loc: a.getFullAddress(blogrollPath),
		})
	}
	// Geo map
	if mc := bc.Map; mc != nil && mc.Enabled {
		sm.Add(&sitemap.URL{
			Loc: a.getFullAddress(bc.getRelativePath(cmp.Or(mc.Path, defaultGeoMapPath))),
		})
	}
	// Contact
	if cc := bc.Contact; cc != nil && cc.Enabled {
		sm.Add(&sitemap.URL{
			Loc: a.getFullAddress(bc.getRelativePath(cmp.Or(cc.Path, defaultContactPath))),
		})
	}
	// Write sitemap
	a.writeSitemapXML(w, r, sm)
}

func (a *goBlog) serveSitemapBlogArchives(w http.ResponseWriter, r *http.Request) {
	// Create sitemap
	sm := sitemap.New()
	// Add archives to sitemap
	b, bc := a.getBlog(r)
	// Sections
	for _, section := range bc.Sections {
		if section.Name != "" {
			sectionLastMod := a.sitemapLastMod(&postsRequestConfig{
				blogs:    []string{b},
				sections: []string{section.Name},
			})
			sm.Add(&sitemap.URL{
				Loc:     a.getFullAddress(bc.getRelativePath(section.Name)),
				LastMod: sectionLastMod,
			})
			datePaths, _ := a.sitemapDatePaths(b, []string{section.Name})
			for _, dp := range datePaths {
				sm.Add(&sitemap.URL{
					Loc:     a.getFullAddress(bc.getRelativePath(path.Join(section.Name, dp.path))),
					LastMod: dp.lastModPtr(),
				})
			}
		}
	}
	// Taxonomies
	blogLastMod := a.sitemapLastMod(&postsRequestConfig{blogs: []string{b}})
	for _, taxonomy := range bc.Taxonomies {
		if taxonomy.Name != "" {
			// Taxonomy
			taxPath := bc.getRelativePath("/" + taxonomy.Name)
			sm.Add(&sitemap.URL{
				Loc:     a.getFullAddress(taxPath),
				LastMod: blogLastMod,
			})
			// Values
			if taxValues, err := a.db.allTaxonomyValues(b, taxonomy.Name); err == nil {
				for _, tv := range taxValues {
					sm.Add(&sitemap.URL{
						Loc: a.getFullAddress(taxPath + "/" + urlize(tv)),
						LastMod: a.sitemapLastMod(&postsRequestConfig{
							blogs:         []string{b},
							taxonomy:      taxonomy,
							taxonomyValue: tv,
						}),
					})
				}
			}
		}
	}
	// Date based archives
	datePaths, _ := a.sitemapDatePaths(b, nil)
	for _, dp := range datePaths {
		sm.Add(&sitemap.URL{
			Loc:     a.getFullAddress(bc.getRelativePath(dp.path)),
			LastMod: dp.lastModPtr(),
		})
	}
	// Write sitemap
	a.writeSitemapXML(w, r, sm)
}

// Serve sitemap with all the blog's posts
func (a *goBlog) serveSitemapBlogPosts(w http.ResponseWriter, r *http.Request) {
	// Create sitemap
	sm := sitemap.New()
	// Request posts
	blog, _ := a.getBlog(r)
	posts, _ := a.getPosts(&postsRequestConfig{
		status:             []postStatus{statusPublished},
		visibility:         []postVisibility{visibilityPublic},
		blogs:              []string{blog},
		fetchWithoutParams: true,
	})
	// Add posts to sitemap
	for _, p := range posts {
		item := &sitemap.URL{Loc: a.fullPostURL(p)}
		lastMod := toLocalTime(cmp.Or(p.Updated, p.Published))
		if !lastMod.IsZero() {
			item.LastMod = &lastMod
		}
		sm.Add(item)
	}
	// Write sitemap
	a.writeSitemapXML(w, r, sm)
}

func (a *goBlog) writeSitemapXML(w http.ResponseWriter, _ *http.Request, sm any) {
	pr, pw := io.Pipe()
	go func() {
		_, _ = io.WriteString(pw, xml.Header)
		_, _ = io.WriteString(pw, `<?xml-stylesheet type="text/xsl" href="`)
		_, _ = io.WriteString(pw, a.assetFileName("sitemap.xsl"))
		_, _ = io.WriteString(pw, `" ?>`)
		_ = pw.CloseWithError(xml.NewEncoder(pw).Encode(sm))
	}()
	w.Header().Set(contentType, contenttype.XMLUTF8)
	_ = pr.CloseWithError(a.min.Get().Minify(contenttype.XML, w, pr))
}

func (a *goBlog) sitemapLastMod(config *postsRequestConfig) *time.Time {
	// Limit to published posts with public visibility, since those are the only ones that would be included in the sitemap
	config.status = []postStatus{statusPublished}
	config.visibility = []postVisibility{visibilityPublic}
	// Get max of updated and published dates for all posts matching the config
	query, args, err := buildPostsQuery(config, "coalesce(nullif(updated, ''), published) as lm")
	if err != nil {
		return nil
	}
	row, err := a.db.QueryRow("select max(lm) from ("+query+") where lm is not null and lm != ''", args...)
	if err != nil {
		return nil
	}
	var lmStr sql.NullString
	if err := row.Scan(&lmStr); err != nil || !lmStr.Valid {
		return nil
	}
	if lm := toLocalTime(lmStr.String); !lm.IsZero() {
		return &lm
	}
	return nil
}

type sitemapDatePath struct {
	path    string
	lastMod time.Time
}

func (d sitemapDatePath) lastModPtr() *time.Time {
	if d.lastMod.IsZero() {
		return nil
	}
	return &d.lastMod
}

const sitemapDatePathsSQL = `
with filteredposts as ( %s ),
alldates as (
    select
        substr(p, 1, 4) as year,
        substr(p, 6, 2) as month,
        substr(p, 9, 2) as day,
        lm
    from (
        select tolocal(published) as p, coalesce(nullif(updated, ''), published) as lm
        from filteredposts
        where coalesce(published, '') != ''
    )
)
select '/' || year, max(lm) from alldates group by year
union all
select '/' || year || '/' || month, max(lm) from alldates group by year, month
union all
select '/' || year || '/' || month || '/' || day, max(lm) from alldates group by year, month, day
union all
select '/x/' || month, max(lm) from alldates group by month
union all
select '/x/' || month || '/' || day, max(lm) from alldates group by month, day
union all
select '/x/x/' || day, max(lm) from alldates group by day;
`

func (a *goBlog) sitemapDatePaths(blog string, sections []string) (paths []sitemapDatePath, err error) {
	query, args, err := buildPostsQuery(&postsRequestConfig{
		blogs:      []string{blog},
		sections:   sections,
		status:     []postStatus{statusPublished},
		visibility: []postVisibility{visibilityPublic},
	}, "published, updated")
	if err != nil {
		return
	}
	rows, err := a.db.Query(fmt.Sprintf(sitemapDatePathsSQL, query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var p, lmStr string
	for rows.Next() {
		if err = rows.Scan(&p, &lmStr); err != nil {
			return nil, err
		}
		paths = append(paths, sitemapDatePath{path: p, lastMod: toLocalTime(lmStr)})
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return
}
