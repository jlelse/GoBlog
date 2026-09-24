package main

import (
	"cmp"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/samber/lo"

	"go.goblog.app/app/pkgs/contenttype"
)

const llmsTxtPath = "/llms.txt"

func (a *goBlog) serveLlmsTxt(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set(contentType, contenttype.TextUTF8)
	a.generateLlmsTxt(w)
}

func (a *goBlog) generateLlmsTxt(w http.ResponseWriter) {
	// Sort blog names; blog at the root path first, then alphabetically for deterministic output
	blogs := lo.Keys(a.cfg.Blogs)
	sort.Slice(blogs, func(i, j int) bool {
		rootOf := func(name string) bool {
			return a.cfg.Blogs[name].Path == "/" || a.cfg.Blogs[name].Path == ""
		}
		if rootOf(blogs[i]) != rootOf(blogs[j]) {
			return rootOf(blogs[i])
		}
		return blogs[i] < blogs[j]
	})
	// Generate for every blog
	for i, blog := range blogs {
		if i > 0 {
			fmt.Fprint(w, "\n---\n\n")
		}
		bc := a.cfg.Blogs[blog]
		// Title, language and descriptions as blockquote lines
		if title := a.renderMdTitle(bc.Title); title != "" {
			fmt.Fprintf(w, "# %s\n\n", title)
		}
		for _, quote := range []string{fmt.Sprintf("%s: %s", a.ts.GetTemplateStringVariant(bc.Lang, "llmslanguage"), bc.Lang), bc.Description, bc.LongDescription} {
			if quote == "" {
				continue
			}
			rendered := a.renderTextSafe(quote)
			if rendered == "" {
				continue
			}
			for line := range strings.SplitSeq(strings.TrimSpace(rendered), "\n") {
				line = strings.TrimSpace(line)
				if line != "" {
					fmt.Fprintf(w, "> %s\n", line)
				}
			}
			fmt.Fprint(w, "\n")
		}
		// Start page and sitemap
		fmt.Fprintf(w, "## %s\n\n", a.ts.GetTemplateStringVariant(bc.Lang, "llmsblog"))
		fmt.Fprintf(w, "- [%s](%s)%s\n", a.renderMdTitle(bc.Title), a.getFullAddress(bc.getRelativePath("")), a.listDesc(cmp.Or(bc.Description, bc.LongDescription)))
		fmt.Fprintf(w, "- [%s](%s)\n\n", a.ts.GetTemplateStringVariant(bc.Lang, "llmssitemap"), a.getFullAddress(bc.getRelativePath(sitemapBlogPath)))
		// Enabled features
		fmt.Fprintf(w, "## %s\n\n", a.ts.GetTemplateStringVariant(bc.Lang, "llmsfeatures"))
		fmt.Fprint(w, a.llmsFeatures(bc))
		fmt.Fprint(w, "\n")
		// Sections
		if len(bc.Sections) > 0 {
			fmt.Fprintf(w, "## %s\n\n", a.ts.GetTemplateStringVariant(bc.Lang, "llmssections"))
			sections := lo.Values(bc.Sections)
			sort.Slice(sections, func(i, j int) bool { return sections[i].Name < sections[j].Name })
			for _, section := range sections {
				if section.Name == "" {
					continue
				}
				fmt.Fprintf(w, "- [%s](%s)%s\n", a.renderMdTitle(section.Title), a.getFullAddress(bc.getRelativePath(section.Name)), a.listDesc(section.Description))
			}
			fmt.Fprint(w, "\n")
		}
		// Taxonomies and values
		for _, taxonomy := range bc.Taxonomies {
			if taxonomy.Name == "" {
				continue
			}
			title := cmp.Or(a.renderMdTitle(taxonomy.Title), taxonomy.Name)
			fmt.Fprintf(w, "## %s\n\n", title)
			fmt.Fprintf(w, "- [%s](%s)%s\n", title, a.getFullAddress(bc.getRelativePath("/"+taxonomy.Name)), a.listDesc(taxonomy.Description))
			if values, err := a.db.allTaxonomyValues(blog, taxonomy.Name); err == nil {
				for _, value := range values {
					fmt.Fprintf(w, "- [%s](%s)\n", value, a.getFullAddress(bc.getRelativePath("/"+taxonomy.Name+"/"+urlize(value))))
				}
			}
			fmt.Fprint(w, "\n")
		}
		// Menus
		menuNames := lo.Keys(bc.Menus)
		sort.Strings(menuNames)
		for _, menuName := range menuNames {
			fmt.Fprintf(w, "## %s\n\n", menuName)
			for _, item := range bc.Menus[menuName].Items {
				fmt.Fprintf(w, "- [%s](%s)\n", a.renderMdTitle(item.Title), item.Link)
			}
			fmt.Fprint(w, "\n")
		}
	}
}

// llmsFeatures generates the feature links for the given blog
func (a *goBlog) llmsFeatures(bc *configBlog) string {
	var sb strings.Builder
	feature := func(label string, path string) {
		fmt.Fprintf(&sb, "- [%s](%s)\n", label, a.getFullAddress(path))
	}
	if pc := bc.Photos; pc != nil && pc.Enabled {
		feature(cmp.Or(a.renderMdTitle(pc.Title), a.ts.GetTemplateStringVariant(bc.Lang, "photos")), bc.getRelativePath(cmp.Or(pc.Path, defaultPhotosPath)))
	}
	if sc := bc.Search; sc != nil && sc.Enabled {
		feature(cmp.Or(a.renderMdTitle(sc.Title), a.ts.GetTemplateStringVariant(bc.Lang, "search")), bc.getRelativePath(cmp.Or(sc.Path, defaultSearchPath)))
	}
	if stc := bc.BlogStats; stc != nil && stc.Enabled {
		feature(cmp.Or(a.renderMdTitle(stc.Title), a.ts.GetTemplateStringVariant(bc.Lang, "blogstats")), bc.getRelativePath(cmp.Or(stc.Path, defaultBlogStatsPath)))
	}
	if otd := bc.OnThisDay; otd != nil && otd.Enabled {
		feature(a.ts.GetTemplateStringVariant(bc.Lang, "onthisday"), bc.getRelativePath(cmp.Or(otd.Path, defaultOnThisDayPath)))
	}
	if rp := bc.RandomPost; rp != nil && rp.Enabled {
		feature(a.ts.GetTemplateStringVariant(bc.Lang, "randompost"), bc.getRelativePath(cmp.Or(rp.Path, defaultRandomPath)))
	}
	if blogrollEnabled, blogrollPath := bc.getBlogrollPath(); blogrollEnabled {
		feature(cmp.Or(a.renderMdTitle(bc.Blogroll.Title), a.ts.GetTemplateStringVariant(bc.Lang, "blogroll")), blogrollPath)
	}
	if mc := bc.Map; mc != nil && mc.Enabled {
		feature(cmp.Or(a.renderMdTitle(mc.Title), a.ts.GetTemplateStringVariant(bc.Lang, "geomap")), bc.getRelativePath(cmp.Or(mc.Path, defaultGeoMapPath)))
	}
	if cc := bc.Contact; cc != nil && cc.Enabled {
		feature(cmp.Or(a.renderMdTitle(cc.Title), a.ts.GetTemplateStringVariant(bc.Lang, "contact")), bc.getRelativePath(cmp.Or(cc.Path, defaultContactPath)))
	}
	return sb.String()
}

// listDesc returns a text, markdown-rendered as plain text and collapsed to a
// single line, prefixed with ": ", used for markdown list link descriptions
func (a *goBlog) listDesc(text string) string {
	if rendered := a.renderTextSafe(text); rendered != "" {
		return ": " + strings.Join(strings.Fields(rendered), " ")
	}
	return ""
}
