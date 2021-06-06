package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/araddon/dateparse"
	"github.com/snabb/sitemap"
)

const sitemapPath = "/sitemap.xml"

func (a *goBlog) serveSitemap(w http.ResponseWriter, r *http.Request) {
	sm := sitemap.New()
	sm.Minify = true
	// Blogs
	for b, bc := range a.cfg.Blogs {
		// Blog
		blogPath := bc.Path
		if blogPath == "/" {
			blogPath = ""
		}
		sm.Add(&sitemap.URL{
			Loc: a.cfg.Server.PublicAddress + blogPath,
		})
		// Sections
		for _, section := range bc.Sections {
			if section.Name != "" {
				sm.Add(&sitemap.URL{
					Loc: a.cfg.Server.PublicAddress + bc.getRelativePath("/"+section.Name),
				})
			}
		}
		// Taxonomies
		for _, taxonomy := range bc.Taxonomies {
			if taxonomy.Name != "" {
				// Taxonomy
				taxPath := bc.getRelativePath("/" + taxonomy.Name)
				sm.Add(&sitemap.URL{
					Loc: a.cfg.Server.PublicAddress + taxPath,
				})
				// Values
				if taxValues, err := a.db.allTaxonomyValues(b, taxonomy.Name); err == nil {
					for _, tv := range taxValues {
						sm.Add(&sitemap.URL{
							Loc: a.cfg.Server.PublicAddress + taxPath + "/" + urlize(tv),
						})
					}
				}
			}
		}
		// Year / month archives
		if dates, err := a.db.allPublishedDates(b); err == nil {
			already := map[string]bool{}
			for _, d := range dates {
				// Year
				yearPath := bc.getRelativePath("/" + fmt.Sprintf("%0004d", d.year))
				if !already[yearPath] {
					sm.Add(&sitemap.URL{
						Loc: a.cfg.Server.PublicAddress + yearPath,
					})
					already[yearPath] = true
				}
				// Specific month
				monthPath := yearPath + "/" + fmt.Sprintf("%02d", d.month)
				if !already[monthPath] {
					sm.Add(&sitemap.URL{
						Loc: a.cfg.Server.PublicAddress + monthPath,
					})
					already[monthPath] = true
				}
				// Specific day
				dayPath := monthPath + "/" + fmt.Sprintf("%02d", d.day)
				if !already[dayPath] {
					sm.Add(&sitemap.URL{
						Loc: a.cfg.Server.PublicAddress + dayPath,
					})
					already[dayPath] = true
				}
				// Generic month
				genericMonthPath := blogPath + "/x/" + fmt.Sprintf("%02d", d.month)
				if !already[genericMonthPath] {
					sm.Add(&sitemap.URL{
						Loc: a.cfg.Server.PublicAddress + genericMonthPath,
					})
					already[genericMonthPath] = true
				}
				// Specific day
				genericMonthDayPath := genericMonthPath + "/" + fmt.Sprintf("%02d", d.day)
				if !already[genericMonthDayPath] {
					sm.Add(&sitemap.URL{
						Loc: a.cfg.Server.PublicAddress + genericMonthDayPath,
					})
					already[genericMonthDayPath] = true
				}
			}
		}
		// Photos
		if bc.Photos != nil && bc.Photos.Enabled {
			sm.Add(&sitemap.URL{
				Loc: a.cfg.Server.PublicAddress + bc.getRelativePath(bc.Photos.Path),
			})
		}
		// Search
		if bc.Search != nil && bc.Search.Enabled {
			sm.Add(&sitemap.URL{
				Loc: a.cfg.Server.PublicAddress + bc.getRelativePath(bc.Search.Path),
			})
		}
		// Stats
		if bc.BlogStats != nil && bc.BlogStats.Enabled {
			sm.Add(&sitemap.URL{
				Loc: a.cfg.Server.PublicAddress + bc.getRelativePath(bc.BlogStats.Path),
			})
		}
		// Blogroll
		if bc.Blogroll != nil && bc.Blogroll.Enabled {
			sm.Add(&sitemap.URL{
				Loc: a.cfg.Server.PublicAddress + bc.getRelativePath(bc.Blogroll.Path),
			})
		}
		// Custom pages
		for _, cp := range bc.CustomPages {
			sm.Add(&sitemap.URL{
				Loc: a.cfg.Server.PublicAddress + cp.Path,
			})
		}
	}
	// Posts
	if posts, err := a.db.getPosts(&postsRequestConfig{status: statusPublished}); err == nil {
		for _, p := range posts {
			item := &sitemap.URL{Loc: a.fullPostURL(p)}
			var lastMod time.Time
			if p.Updated != "" {
				lastMod, _ = dateparse.ParseLocal(p.Updated)
			}
			if p.Published != "" && lastMod.IsZero() {
				lastMod, _ = dateparse.ParseLocal(p.Published)
			}
			if !lastMod.IsZero() {
				item.LastMod = &lastMod
			}
			sm.Add(item)
		}
	}
	// Write...
	_, _ = sm.WriteTo(w) // Already minified
}
