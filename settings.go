package main

import (
	"net/http"
	"sort"

	"github.com/samber/lo"
)

const settingsPath = "/settings"

func (a *goBlog) serveSettings(w http.ResponseWriter, r *http.Request) {
	blog, bc := a.getBlog(r)

	sections := lo.Values(bc.Sections)
	sort.Slice(sections, func(i, j int) bool { return sections[i].Name < sections[j].Name })

	a.render(w, r, a.renderSettings, &renderData{
		Data: &settingsRenderData{
			blog:                  blog,
			sections:              sections,
			defaultSection:        bc.DefaultSection,
			hideOldContentWarning: bc.hideOldContentWarning,
		},
	})
}

const settingsDeleteSectionPath = "/deletesection"

func (a *goBlog) settingsDeleteSection(w http.ResponseWriter, r *http.Request) {
	blog, bc := a.getBlog(r)
	section := r.FormValue("sectionname")
	// Check if any post uses this section
	count, err := a.db.countPosts(&postsRequestConfig{
		blog:     blog,
		sections: []string{section},
	})
	if err != nil {
		a.serveError(w, r, "Failed to check if section is still used", http.StatusInternalServerError)
		return
	}
	if count > 0 {
		a.serveError(w, r, "Section is still used", http.StatusBadRequest)
		return
	}
	// Delete section
	err = a.deleteSection(blog, section)
	if err != nil {
		a.serveError(w, r, "Failed to delete section from the database", http.StatusInternalServerError)
		return
	}
	// Reload sections
	err = a.loadSections()
	if err != nil {
		a.serveError(w, r, "Failed to reload section configuration from the database", http.StatusInternalServerError)
		return
	}
	a.reloadRouter()
	a.cache.purge()
	http.Redirect(w, r, bc.getRelativePath(settingsPath), http.StatusFound)
}

const settingsCreateSectionPath = "/createsection"

func (a *goBlog) settingsCreateSection(w http.ResponseWriter, r *http.Request) {
	blog, bc := a.getBlog(r)
	// Read values
	sectionName := r.FormValue("sectionname")
	sectionTitle := r.FormValue("sectiontitle")
	if sectionName == "" || sectionTitle == "" {
		a.serveError(w, r, "Missing values for name or title", http.StatusBadRequest)
		return
	}
	// Create section
	section := &configSection{
		Name:  sectionName,
		Title: sectionTitle,
	}
	err := a.saveSection(blog, section)
	if err != nil {
		a.serveError(w, r, "Failed to insert section into database", http.StatusInternalServerError)
		return
	}
	// Reload sections
	err = a.loadSections()
	if err != nil {
		a.serveError(w, r, "Failed to reload section configuration from the database", http.StatusInternalServerError)
		return
	}
	a.reloadRouter()
	a.cache.purge()
	http.Redirect(w, r, bc.getRelativePath(settingsPath), http.StatusFound)
}

const settingsUpdateSectionPath = "/updatesection"

func (a *goBlog) settingsUpdateSection(w http.ResponseWriter, r *http.Request) {
	blog, bc := a.getBlog(r)
	// Read values
	sectionName := r.FormValue("sectionname")
	sectionTitle := r.FormValue("sectiontitle")
	if sectionName == "" || sectionTitle == "" {
		a.serveError(w, r, "Missing values for name or title", http.StatusBadRequest)
		return
	}
	sectionDescription := r.FormValue("sectiondescription")
	sectionPathTemplate := r.FormValue("sectionpathtemplate")
	sectionShowFull := r.FormValue("sectionshowfull") == "on"
	// Create section
	section := &configSection{
		Name:         sectionName,
		Title:        sectionTitle,
		Description:  sectionDescription,
		PathTemplate: sectionPathTemplate,
		ShowFull:     sectionShowFull,
	}
	err := a.saveSection(blog, section)
	if err != nil {
		a.serveError(w, r, "Failed to update section in database", http.StatusInternalServerError)
		return
	}
	// Reload sections
	err = a.loadSections()
	if err != nil {
		a.serveError(w, r, "Failed to reload section configuration from the database", http.StatusInternalServerError)
		return
	}
	a.reloadRouter()
	a.cache.purge()
	http.Redirect(w, r, bc.getRelativePath(settingsPath), http.StatusFound)
}

const settingsUpdateDefaultSectionPath = "/updatedefaultsection"

func (a *goBlog) settingsUpdateDefaultSection(w http.ResponseWriter, r *http.Request) {
	blog, bc := a.getBlog(r)
	// Read values
	newDefaultSection := r.FormValue("defaultsection")
	// Check plausibility
	if _, ok := bc.Sections[newDefaultSection]; !ok {
		a.serveError(w, r, "Section unknown", http.StatusBadRequest)
		return
	}
	// Update
	err := a.saveSettingValue(settingNameWithBlog(blog, defaultSectionSetting), newDefaultSection)
	if err != nil {
		a.serveError(w, r, "Failed to update default section in database", http.StatusInternalServerError)
		return
	}
	bc.DefaultSection = newDefaultSection
	http.Redirect(w, r, bc.getRelativePath(settingsPath), http.StatusFound)
}

const settingsHideOldContentWarningPath = "/oldcontentwarning"

func (a *goBlog) settingsHideOldContentWarning(w http.ResponseWriter, r *http.Request) {
	blog, bc := a.getBlog(r)
	// Read values
	hideOldContentWarning := r.FormValue(hideOldContentWarningSetting) == "on"
	// Update
	err := a.saveBooleanSettingValue(settingNameWithBlog(blog, hideOldContentWarningSetting), hideOldContentWarning)
	if err != nil {
		a.serveError(w, r, "Failed to update default section in database", http.StatusInternalServerError)
		return
	}
	bc.hideOldContentWarning = hideOldContentWarning
	a.cache.purge()
	http.Redirect(w, r, bc.getRelativePath(settingsPath), http.StatusFound)
}
