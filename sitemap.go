package main

import (
	"bytes"
	"database/sql"
	"encoding/xml"
	"net/http"
	"time"

	"github.com/araddon/dateparse"
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
	a.writeSitemapXML(w, sm)
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
	a.writeSitemapXML(w, sm)
}

func (a *goBlog) serveSitemapBlogFeatures(w http.ResponseWriter, r *http.Request) {
	// Create sitemap
	sm := sitemap.New()
	// Add features to sitemap
	_, bc := a.getBlog(r)
	// Home
	sm.Add(&sitemap.URL{
		Loc: a.getFullAddress(bc.getRelativePath("")),
	})
	// Photos
	if pc := bc.Photos; pc != nil && pc.Enabled {
		sm.Add(&sitemap.URL{
			Loc: a.getFullAddress(bc.getRelativePath(defaultIfEmpty(pc.Path, defaultPhotosPath))),
		})
	}
	// Search
	if bsc := bc.Search; bsc != nil && bsc.Enabled {
		sm.Add(&sitemap.URL{
			Loc: a.getFullAddress(bc.getRelativePath(defaultIfEmpty(bsc.Path, defaultSearchPath))),
		})
	}
	// Stats
	if bsc := bc.BlogStats; bsc != nil && bsc.Enabled {
		sm.Add(&sitemap.URL{
			Loc: a.getFullAddress(bc.getRelativePath(defaultIfEmpty(bsc.Path, defaultBlogStatsPath))),
		})
	}
	// Blogroll
	if brc := bc.Blogroll; brc != nil && brc.Enabled {
		sm.Add(&sitemap.URL{
			Loc: a.getFullAddress(bc.getRelativePath(defaultIfEmpty(brc.Path, defaultBlogrollPath))),
		})
	}
	// Geo map
	if mc := bc.Map; mc != nil && mc.Enabled {
		sm.Add(&sitemap.URL{
			Loc: a.getFullAddress(bc.getRelativePath(defaultIfEmpty(mc.Path, defaultGeoMapPath))),
		})
	}
	// Contact
	if cc := bc.Contact; cc != nil && cc.Enabled {
		sm.Add(&sitemap.URL{
			Loc: a.getFullAddress(bc.getRelativePath(defaultIfEmpty(cc.Path, defaultContactPath))),
		})
	}
	// Custom pages
	for _, cp := range bc.CustomPages {
		sm.Add(&sitemap.URL{
			Loc: a.getFullAddress(cp.Path),
		})
	}
	// Write sitemap
	a.writeSitemapXML(w, sm)
}

func (a *goBlog) serveSitemapBlogArchives(w http.ResponseWriter, r *http.Request) {
	// Create sitemap
	sm := sitemap.New()
	// Add archives to sitemap
	b, bc := a.getBlog(r)
	// Sections
	for _, section := range bc.Sections {
		if section.Name != "" {
			sm.Add(&sitemap.URL{
				Loc: a.getFullAddress(bc.getRelativePath(section.Name)),
			})
		}
	}
	// Taxonomies
	for _, taxonomy := range bc.Taxonomies {
		if taxonomy.Name != "" {
			// Taxonomy
			taxPath := bc.getRelativePath("/" + taxonomy.Name)
			sm.Add(&sitemap.URL{
				Loc: a.getFullAddress(taxPath),
			})
			// Values
			if taxValues, err := a.db.allTaxonomyValues(b, taxonomy.Name); err == nil {
				for _, tv := range taxValues {
					sm.Add(&sitemap.URL{
						Loc: a.getFullAddress(taxPath + "/" + urlize(tv)),
					})
				}
			}
		}
	}
	// Date based archives
	datePaths, _ := a.sitemapDatePaths(b)
	for _, p := range datePaths {
		sm.Add(&sitemap.URL{
			Loc: a.getFullAddress(bc.getRelativePath(p)),
		})
	}
	// Write sitemap
	a.writeSitemapXML(w, sm)
}

// Serve sitemap with all the blog's posts
func (a *goBlog) serveSitemapBlogPosts(w http.ResponseWriter, r *http.Request) {
	// Create sitemap
	sm := sitemap.New()
	// Request posts
	blog, _ := a.getBlog(r)
	posts, _ := a.getPosts(&postsRequestConfig{
		status:            statusPublished,
		blog:              blog,
		withoutParameters: true,
	})
	// Add posts to sitemap
	for _, p := range posts {
		item := &sitemap.URL{Loc: a.fullPostURL(p)}
		lastMod := timeNoErr(dateparse.ParseLocal(defaultIfEmpty(p.Updated, p.Published)))
		if !lastMod.IsZero() {
			item.LastMod = &lastMod
		}
		sm.Add(item)
	}
	// Write sitemap
	a.writeSitemapXML(w, sm)
}

func (a *goBlog) writeSitemapXML(w http.ResponseWriter, sm interface{}) {
	w.Header().Set(contentType, contenttype.XMLUTF8)
	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	buf.WriteString(`<?xml-stylesheet type="text/xsl" href="`)
	buf.WriteString(a.assetFileName("sitemap.xsl"))
	buf.WriteString(`" ?>`)
	_ = xml.NewEncoder(&buf).Encode(sm)
	_ = a.min.Minify(contenttype.XML, w, &buf)
}

const sitemapDatePathsSql = `
with alldates as (
    select distinct 
        substr(published, 1, 4) as year,
        substr(published, 6, 2) as month,
        substr(published, 9, 2) as day
    from (
            select tolocal(coalesce(published, '')) as published
            from posts
            where blog = @blog and status = @status and published != ''
        )
)
select distinct '/' || year from alldates
union
select distinct '/' || year || '/' || month from alldates
union
select distinct '/' || year || '/' || month || '/' || day from alldates
union
select distinct '/x/' || month from alldates
union
select distinct '/x/' || month || '/' || day from alldates
union
select distinct '/x/x/' || day from alldates;
`

func (a *goBlog) sitemapDatePaths(blog string) (paths []string, err error) {
	rows, err := a.db.query(sitemapDatePathsSql, sql.Named("blog", blog), sql.Named("status", statusPublished))
	if err != nil {
		return nil, err
	}
	var path string
	for rows.Next() {
		err = rows.Scan(&path)
		if err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return
}
