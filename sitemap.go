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
		sm.Add(&sitemap.URL{
			Loc: a.getFullAddress(bc.getRelativePath("")),
		})
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
		// Year / month archives
		if dates, err := a.db.allPublishedDates(b); err == nil {
			already := map[string]bool{}
			for _, d := range dates {
				// Year
				yearPath := bc.getRelativePath("/" + fmt.Sprintf("%0004d", d.year))
				if !already[yearPath] {
					sm.Add(&sitemap.URL{
						Loc: a.getFullAddress(yearPath),
					})
					already[yearPath] = true
				}
				// Month
				monthPath := yearPath + "/" + fmt.Sprintf("%02d", d.month)
				if !already[monthPath] {
					sm.Add(&sitemap.URL{
						Loc: a.getFullAddress(monthPath),
					})
					already[monthPath] = true
				}
				// Day
				dayPath := monthPath + "/" + fmt.Sprintf("%02d", d.day)
				if !already[dayPath] {
					sm.Add(&sitemap.URL{
						Loc: a.getFullAddress(dayPath),
					})
					already[dayPath] = true
				}
				// XXXX-MM
				genericMonthPath := bc.getRelativePath("/x/" + fmt.Sprintf("%02d", d.month))
				if !already[genericMonthPath] {
					sm.Add(&sitemap.URL{
						Loc: a.getFullAddress(genericMonthPath),
					})
					already[genericMonthPath] = true
				}
				// XXXX-MM-DD
				genericMonthDayPath := genericMonthPath + "/" + fmt.Sprintf("%02d", d.day)
				if !already[genericMonthDayPath] {
					sm.Add(&sitemap.URL{
						Loc: a.getFullAddress(genericMonthDayPath),
					})
					already[genericMonthDayPath] = true
				}
				// XXXX-XX-DD
				genericDayPath := bc.getRelativePath("/x/x/" + fmt.Sprintf("%02d", d.day))
				if !already[genericDayPath] {
					sm.Add(&sitemap.URL{
						Loc: a.getFullAddress(genericDayPath),
					})
					already[genericDayPath] = true
				}
			}
		}
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
	}
	// Posts
	if posts, err := a.db.getPosts(&postsRequestConfig{status: statusPublished, withoutParameters: true}); err == nil {
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
