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
			hideShareButton:       bc.hideShareButton,
			hideTranslateButton:   bc.hideTranslateButton,
			hideSpeakButton:       bc.hideSpeakButton,
			addReplyTitle:         bc.addReplyTitle,
			addReplyContext:       bc.addReplyContext,
			addLikeTitle:          bc.addLikeTitle,
			addLikeContext:        bc.addLikeContext,
			userNick:              a.cfg.User.Nick,
			userName:              a.cfg.User.Name,
		},
	})
}

func (a *goBlog) booleanBlogSettingHandler(settingName string, apply func(*configBlog, bool)) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		blog, bc := a.getBlog(r)
		// Read values
		settingValue := r.FormValue(settingName) == "on"
		// Update
		err := a.saveBooleanSettingValue(settingNameWithBlog(blog, settingName), settingValue)
		if err != nil {
			a.serveError(w, r, "Failed to update setting in database", http.StatusInternalServerError)
			return
		}
		// Apply
		apply(bc, settingValue)
		http.Redirect(w, r, bc.getRelativePath(settingsPath), http.StatusFound)
	})
}

func (a *goBlog) getBooleanSettingHandler(settingName string) http.HandlerFunc {
	var apply func(*configBlog, bool)
	switch settingName {
	case hideOldContentWarningSetting:
		apply = func(cb *configBlog, b bool) { cb.hideOldContentWarning = b; a.purgeCache() }
	case hideShareButtonSetting:
		apply = func(cb *configBlog, b bool) { cb.hideShareButton = b; a.purgeCache() }
	case hideTranslateButtonSetting:
		apply = func(cb *configBlog, b bool) { cb.hideTranslateButton = b; a.purgeCache() }
	case hideSpeakButtonSetting:
		apply = func(cb *configBlog, b bool) { cb.hideSpeakButton = b; a.purgeCache() }
	case addReplyTitleSetting:
		apply = func(cb *configBlog, b bool) { cb.addReplyTitle = b }
	case addReplyContextSetting:
		apply = func(cb *configBlog, b bool) { cb.addReplyContext = b }
	case addLikeTitleSetting:
		apply = func(cb *configBlog, b bool) { cb.addLikeTitle = b }
	case addLikeContextSetting:
		apply = func(cb *configBlog, b bool) { cb.addLikeContext = b }
	}
	return a.booleanBlogSettingHandler(settingName, apply)
}

const settingsDeleteSectionPath = "/deletesection"

func (a *goBlog) settingsDeleteSection(w http.ResponseWriter, r *http.Request) {
	blog, bc := a.getBlog(r)
	section := r.FormValue("sectionname")
	// Check if any post uses this section
	count, err := a.db.countPosts(&postsRequestConfig{
		blogs:    []string{blog},
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
	a.purgeCache()
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
	sectionHideOnStart := r.FormValue("sectionhideonstart") == "on"
	// Create section
	section := &configSection{
		Name:         sectionName,
		Title:        sectionTitle,
		Description:  sectionDescription,
		PathTemplate: sectionPathTemplate,
		ShowFull:     sectionShowFull,
		HideOnStart:  sectionHideOnStart,
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
	a.purgeCache()
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

func (a *goBlog) settingsHideOldContentWarning() http.HandlerFunc {
	return a.getBooleanSettingHandler(hideOldContentWarningSetting)
}

const settingsHideShareButtonPath = "/sharebutton"

func (a *goBlog) settingsHideShareButton() http.HandlerFunc {
	return a.getBooleanSettingHandler(hideShareButtonSetting)
}

const settingsHideTranslateButtonPath = "/translatebutton"

func (a *goBlog) settingsHideTranslateButton() http.HandlerFunc {
	return a.getBooleanSettingHandler(hideTranslateButtonSetting)
}

const settingsHideSpeakButtonPath = "/speakbutton"

func (a *goBlog) settingsHideSpeakButton() http.HandlerFunc {
	return a.getBooleanSettingHandler(hideSpeakButtonSetting)
}

const settingsAddReplyTitlePath = "/replytitle"

func (a *goBlog) settingsAddReplyTitle() http.HandlerFunc {
	return a.getBooleanSettingHandler(addReplyTitleSetting)
}

const settingsAddReplyContextPath = "/replycontext"

func (a *goBlog) settingsAddReplyContext() http.HandlerFunc {
	return a.getBooleanSettingHandler(addReplyContextSetting)
}

const settingsAddLikeTitlePath = "/liketitle"

func (a *goBlog) settingsAddLikeTitle() http.HandlerFunc {
	return a.getBooleanSettingHandler(addLikeTitleSetting)
}

const settingsAddLikeContextPath = "/likecontext"

func (a *goBlog) settingsAddLikeContext() http.HandlerFunc {
	return a.getBooleanSettingHandler(addLikeContextSetting)
}

const settingsUpdateUserPath = "/user"

func (a *goBlog) settingsUpdateUser(w http.ResponseWriter, r *http.Request) {
	_, bc := a.getBlog(r)
	// Read values
	userNick := r.FormValue(userNickSetting)
	userName := r.FormValue(userNameSetting)
	if userNick == "" || userName == "" {
		a.serveError(w, r, "Values must not be empty", http.StatusInternalServerError)
		return
	}
	// Update
	err := a.saveSettingValue(userNickSetting, userNick)
	if err != nil {
		a.serveError(w, r, "Failed to update user nick in database", http.StatusInternalServerError)
		return
	}
	err = a.saveSettingValue(userNameSetting, userName)
	if err != nil {
		a.serveError(w, r, "Failed to update user name in database", http.StatusInternalServerError)
		return
	}
	a.cfg.User.Nick = userNick
	a.cfg.User.Name = userName
	a.purgeCache()
	http.Redirect(w, r, bc.getRelativePath(settingsPath), http.StatusFound)
}
