package main

import (
	"cmp"
	"database/sql"
	"encoding/xml"
	"io"
	"net/http"
	"time"

	"github.com/snabb/sitemap"
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

func (a *goBlog) writeSitemapXML(w http.ResponseWriter, r *http.Request, sm any) {
	script, err := a.newXMLViewScript("js/xml-view.js").marshal()
	if err != nil {
		a.error("Sitemap: Failed to encode script", "err", err)
		a.serveError(w, r, "", http.StatusInternalServerError)
		return
	}
	a.writeXMLWithViewScript(w, script, func(w io.Writer) error {
		return xml.NewEncoder(w).Encode(sm)
	})
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
