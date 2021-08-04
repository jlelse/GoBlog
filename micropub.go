package main

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cast"
	"github.com/thoas/go-funk"
	"go.goblog.app/app/pkgs/contenttype"
	"gopkg.in/yaml.v3"
)

const micropubPath = "/micropub"

type micropubConfig struct {
	MediaEndpoint string `json:"media-endpoint,omitempty"`
}

func (a *goBlog) serveMicropubQuery(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Query().Get("q") {
	case "config":
		w.Header().Set(contentType, contenttype.JSONUTF8)
		w.WriteHeader(http.StatusOK)
		b, _ := json.Marshal(&micropubConfig{
			MediaEndpoint: a.getFullAddress(micropubPath + micropubMediaSubPath),
		})
		_, _ = a.min.Write(w, contenttype.JSON, b)
	case "source":
		var mf interface{}
		if urlString := r.URL.Query().Get("url"); urlString != "" {
			u, err := url.Parse(r.URL.Query().Get("url"))
			if err != nil {
				a.serveError(w, r, err.Error(), http.StatusBadRequest)
				return
			}
			p, err := a.db.getPost(u.Path)
			if err != nil {
				a.serveError(w, r, err.Error(), http.StatusBadRequest)
				return
			}
			mf = a.postToMfItem(p)
		} else {
			limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
			offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
			posts, err := a.db.getPosts(&postsRequestConfig{
				limit:  limit,
				offset: offset,
			})
			if err != nil {
				a.serveError(w, r, err.Error(), http.StatusInternalServerError)
				return
			}
			list := map[string][]*microformatItem{}
			for _, p := range posts {
				list["items"] = append(list["items"], a.postToMfItem(p))
			}
			mf = list
		}
		w.Header().Set(contentType, contenttype.JSONUTF8)
		w.WriteHeader(http.StatusOK)
		b, _ := json.Marshal(mf)
		_, _ = a.min.Write(w, contenttype.JSON, b)
	case "category":
		allCategories := []string{}
		for blog := range a.cfg.Blogs {
			values, err := a.db.allTaxonomyValues(blog, a.cfg.Micropub.CategoryParam)
			if err != nil {
				a.serveError(w, r, err.Error(), http.StatusInternalServerError)
				return
			}
			allCategories = append(allCategories, values...)
		}
		w.Header().Set(contentType, contenttype.JSONUTF8)
		w.WriteHeader(http.StatusOK)
		b, _ := json.Marshal(map[string]interface{}{
			"categories": allCategories,
		})
		_, _ = a.min.Write(w, contenttype.JSON, b)
	default:
		a.serve404(w, r)
	}
}

func (a *goBlog) serveMicropubPost(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	switch mt, _, _ := mime.ParseMediaType(r.Header.Get(contentType)); mt {
	case contenttype.WWWForm, contenttype.MultipartForm:
		_ = r.ParseForm()
		_ = r.ParseMultipartForm(0)
		if r.Form == nil {
			a.serveError(w, r, "Failed to parse form", http.StatusBadRequest)
			return
		}
		if action := micropubAction(r.Form.Get("action")); action != "" {
			switch action {
			case actionDelete:
				a.micropubDelete(w, r, r.Form.Get("url"))
			default:
				a.serveError(w, r, "Action not supported", http.StatusNotImplemented)
			}
			return
		}
		a.micropubCreatePostFromForm(w, r)
	case contenttype.JSON:
		parsedMfItem := &microformatItem{}
		b, _ := io.ReadAll(io.LimitReader(r.Body, 10000000)) // 10 MB
		if err := json.Unmarshal(b, parsedMfItem); err != nil {
			a.serveError(w, r, err.Error(), http.StatusBadRequest)
			return
		}
		if parsedMfItem.Action != "" {
			switch parsedMfItem.Action {
			case actionDelete:
				a.micropubDelete(w, r, parsedMfItem.URL)
			case actionUpdate:
				a.micropubUpdate(w, r, parsedMfItem.URL, parsedMfItem)
			default:
				a.serveError(w, r, "Action not supported", http.StatusNotImplemented)
			}
			return
		}
		a.micropubCreatePostFromJson(w, r, parsedMfItem)
	default:
		a.serveError(w, r, "wrong content type", http.StatusBadRequest)
	}
}

func (a *goBlog) micropubParseValuePostParamsValueMap(entry *post, values map[string][]string) error {
	if h, ok := values["h"]; ok && (len(h) != 1 || h[0] != "entry") {
		return errors.New("only entry type is supported so far")
	}
	delete(values, "h")
	entry.Parameters = map[string][]string{}
	if content, ok := values["content"]; ok && len(content) > 0 {
		entry.Content = content[0]
		delete(values, "content")
	}
	if published, ok := values["published"]; ok && len(published) > 0 {
		entry.Published = published[0]
		delete(values, "published")
	}
	if updated, ok := values["updated"]; ok && len(updated) > 0 {
		entry.Updated = updated[0]
		delete(values, "updated")
	}
	if slug, ok := values["mp-slug"]; ok && len(slug) > 0 {
		entry.Slug = slug[0]
		delete(values, "mp-slug")
	}
	// Status
	statusStr := ""
	if status, ok := values["post-status"]; ok && len(status) > 0 {
		statusStr = status[0]
		delete(values, "post-status")
	}
	visibilityStr := ""
	if visibility, ok := values["visibility"]; ok && len(visibility) > 0 {
		visibilityStr = visibility[0]
		delete(values, "visibility")
	}
	if finalStatus := micropubStatus(statusNil, statusStr, visibilityStr); finalStatus != statusNil {
		entry.Status = finalStatus
	}
	// Parameter
	if name, ok := values["name"]; ok {
		entry.Parameters["title"] = name
		delete(values, "name")
	}
	if category, ok := values["category"]; ok {
		entry.Parameters[a.cfg.Micropub.CategoryParam] = category
		delete(values, "category")
	} else if categories, ok := values["category[]"]; ok {
		entry.Parameters[a.cfg.Micropub.CategoryParam] = categories
		delete(values, "category[]")
	}
	if inReplyTo, ok := values["in-reply-to"]; ok {
		entry.Parameters[a.cfg.Micropub.ReplyParam] = inReplyTo
		delete(values, "in-reply-to")
	}
	if likeOf, ok := values["like-of"]; ok {
		entry.Parameters[a.cfg.Micropub.LikeParam] = likeOf
		delete(values, "like-of")
	}
	if bookmarkOf, ok := values["bookmark-of"]; ok {
		entry.Parameters[a.cfg.Micropub.BookmarkParam] = bookmarkOf
		delete(values, "bookmark-of")
	}
	if audio, ok := values["audio"]; ok {
		entry.Parameters[a.cfg.Micropub.AudioParam] = audio
		delete(values, "audio")
	} else if audio, ok := values["audio[]"]; ok {
		entry.Parameters[a.cfg.Micropub.AudioParam] = audio
		delete(values, "audio[]")
	}
	if photo, ok := values["photo"]; ok {
		entry.Parameters[a.cfg.Micropub.PhotoParam] = photo
		delete(values, "photo")
	} else if photos, ok := values["photo[]"]; ok {
		entry.Parameters[a.cfg.Micropub.PhotoParam] = photos
		delete(values, "photo[]")
	}
	if photoAlt, ok := values["mp-photo-alt"]; ok {
		entry.Parameters[a.cfg.Micropub.PhotoDescriptionParam] = photoAlt
		delete(values, "mp-photo-alt")
	} else if photoAlts, ok := values["mp-photo-alt[]"]; ok {
		entry.Parameters[a.cfg.Micropub.PhotoDescriptionParam] = photoAlts
		delete(values, "mp-photo-alt[]")
	}
	if location, ok := values["location"]; ok {
		entry.Parameters[a.cfg.Micropub.LocationParam] = location
		delete(values, "location")
	}
	for n, p := range values {
		entry.Parameters[n] = append(entry.Parameters[n], p...)
	}
	return nil
}

type micropubAction string

const (
	actionUpdate micropubAction = "update"
	actionDelete micropubAction = "delete"
)

type microformatItem struct {
	Type       []string                 `json:"type,omitempty"`
	URL        string                   `json:"url,omitempty"`
	Action     micropubAction           `json:"action,omitempty"`
	Properties *microformatProperties   `json:"properties,omitempty"`
	Replace    map[string][]interface{} `json:"replace,omitempty"`
	Add        map[string][]interface{} `json:"add,omitempty"`
	Delete     interface{}              `json:"delete,omitempty"`
}

type microformatProperties struct {
	Name       []string      `json:"name,omitempty"`
	Published  []string      `json:"published,omitempty"`
	Updated    []string      `json:"updated,omitempty"`
	PostStatus []string      `json:"post-status,omitempty"`
	Visibility []string      `json:"visibility,omitempty"`
	Category   []string      `json:"category,omitempty"`
	Content    []string      `json:"content,omitempty"`
	URL        []string      `json:"url,omitempty"`
	InReplyTo  []string      `json:"in-reply-to,omitempty"`
	LikeOf     []string      `json:"like-of,omitempty"`
	BookmarkOf []string      `json:"bookmark-of,omitempty"`
	MpSlug     []string      `json:"mp-slug,omitempty"`
	Photo      []interface{} `json:"photo,omitempty"`
	Audio      []string      `json:"audio,omitempty"`
}

func (a *goBlog) micropubParsePostParamsMfItem(entry *post, mf *microformatItem) error {
	if len(mf.Type) != 1 || mf.Type[0] != "h-entry" {
		return errors.New("only entry type is supported so far")
	}
	entry.Parameters = map[string][]string{}
	if mf.Properties == nil {
		return nil
	}
	// Content
	if len(mf.Properties.Content) > 0 && mf.Properties.Content[0] != "" {
		entry.Content = mf.Properties.Content[0]
	}
	if len(mf.Properties.Published) > 0 {
		entry.Published = mf.Properties.Published[0]
	}
	if len(mf.Properties.Updated) > 0 {
		entry.Updated = mf.Properties.Updated[0]
	}
	if len(mf.Properties.MpSlug) > 0 {
		entry.Slug = mf.Properties.MpSlug[0]
	}
	// Status
	status := ""
	if len(mf.Properties.PostStatus) > 0 {
		status = mf.Properties.PostStatus[0]
	}
	visibility := ""
	if len(mf.Properties.Visibility) > 0 {
		visibility = mf.Properties.Visibility[0]
	}
	if finalStatus := micropubStatus(statusNil, status, visibility); finalStatus != statusNil {
		entry.Status = finalStatus
	}
	// Parameter
	if len(mf.Properties.Name) > 0 {
		entry.Parameters["title"] = mf.Properties.Name
	}
	if len(mf.Properties.Category) > 0 {
		entry.Parameters[a.cfg.Micropub.CategoryParam] = mf.Properties.Category
	}
	if len(mf.Properties.InReplyTo) > 0 {
		entry.Parameters[a.cfg.Micropub.ReplyParam] = mf.Properties.InReplyTo
	}
	if len(mf.Properties.LikeOf) > 0 {
		entry.Parameters[a.cfg.Micropub.LikeParam] = mf.Properties.LikeOf
	}
	if len(mf.Properties.BookmarkOf) > 0 {
		entry.Parameters[a.cfg.Micropub.BookmarkParam] = mf.Properties.BookmarkOf
	}
	if len(mf.Properties.Audio) > 0 {
		entry.Parameters[a.cfg.Micropub.AudioParam] = mf.Properties.Audio
	}
	if len(mf.Properties.Photo) > 0 {
		for _, photo := range mf.Properties.Photo {
			if theString, justString := photo.(string); justString {
				entry.Parameters[a.cfg.Micropub.PhotoParam] = append(entry.Parameters[a.cfg.Micropub.PhotoParam], theString)
				entry.Parameters[a.cfg.Micropub.PhotoDescriptionParam] = append(entry.Parameters[a.cfg.Micropub.PhotoDescriptionParam], "")
			} else if thePhoto, isPhoto := photo.(map[string]interface{}); isPhoto {
				entry.Parameters[a.cfg.Micropub.PhotoParam] = append(entry.Parameters[a.cfg.Micropub.PhotoParam], cast.ToString(thePhoto["value"]))
				entry.Parameters[a.cfg.Micropub.PhotoDescriptionParam] = append(entry.Parameters[a.cfg.Micropub.PhotoDescriptionParam], cast.ToString(thePhoto["alt"]))
			}
		}
	}
	return nil
}

func (a *goBlog) computeExtraPostParameters(p *post) error {
	if p.Parameters == nil {
		p.Parameters = map[string][]string{}
	}
	p.Content = regexp.MustCompile("\r\n").ReplaceAllString(p.Content, "\n")
	if split := strings.Split(p.Content, "---\n"); len(split) >= 3 && len(strings.TrimSpace(split[0])) == 0 {
		// Contains frontmatter
		fm := split[1]
		meta := map[string]interface{}{}
		err := yaml.Unmarshal([]byte(fm), &meta)
		if err != nil {
			return err
		}
		// Find section and copy frontmatter to params
		for key, value := range meta {
			// Delete existing content - replace
			p.Parameters[key] = []string{}
			if a, ok := value.([]interface{}); ok {
				for _, ae := range a {
					p.Parameters[key] = append(p.Parameters[key], cast.ToString(ae))
				}
			} else {
				p.Parameters[key] = append(p.Parameters[key], cast.ToString(value))
			}
		}
		// Remove frontmatter from content
		p.Content = strings.Join(split[2:], "---\n")
	}
	// Check settings
	if blog := p.Parameters["blog"]; len(blog) == 1 && blog[0] != "" {
		p.Blog = blog[0]
		delete(p.Parameters, "blog")
	} else {
		p.Blog = a.cfg.DefaultBlog
	}
	if path := p.Parameters["path"]; len(path) == 1 {
		p.Path = path[0]
		delete(p.Parameters, "path")
	}
	if section := p.Parameters["section"]; len(section) == 1 {
		p.Section = section[0]
		delete(p.Parameters, "section")
	}
	if slug := p.Parameters["slug"]; len(slug) == 1 {
		p.Slug = slug[0]
		delete(p.Parameters, "slug")
	}
	if published := p.Parameters["published"]; len(published) == 1 {
		p.Published = published[0]
		delete(p.Parameters, "published")
	}
	if updated := p.Parameters["updated"]; len(updated) == 1 {
		p.Updated = updated[0]
		delete(p.Parameters, "updated")
	}
	if status := p.Parameters["status"]; len(status) == 1 {
		p.Status = postStatus(status[0])
		delete(p.Parameters, "status")
	}
	if priority := p.Parameters["priority"]; len(priority) == 1 {
		p.Priority = cast.ToInt(priority[0])
		delete(p.Parameters, "priority")
	}
	if p.Path == "" && p.Section == "" {
		// Has no path or section -> default section
		p.Section = a.cfg.Blogs[p.Blog].DefaultSection
	}
	if p.Published == "" && p.Section != "" {
		// Has no published date, but section -> published now
		p.Published = localNowString()
	}
	// Add images not in content
	images := p.Parameters[a.cfg.Micropub.PhotoParam]
	imageAlts := p.Parameters[a.cfg.Micropub.PhotoDescriptionParam]
	useAlts := len(images) == len(imageAlts)
	for i, image := range images {
		if !strings.Contains(p.Content, image) {
			if useAlts && len(imageAlts[i]) > 0 {
				p.Content += "\n\n![" + imageAlts[i] + "](" + image + " \"" + imageAlts[i] + "\")"
			} else {
				p.Content += "\n\n![](" + image + ")"
			}
		}
	}
	return nil
}

func (a *goBlog) micropubCreatePostFromForm(w http.ResponseWriter, r *http.Request) {
	p := &post{}
	err := a.micropubParseValuePostParamsValueMap(p, r.Form)
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusBadRequest)
		return
	}
	a.micropubCreate(w, r, p)
}

func (a *goBlog) micropubCreatePostFromJson(w http.ResponseWriter, r *http.Request, parsedMfItem *microformatItem) {
	p := &post{}
	err := a.micropubParsePostParamsMfItem(p, parsedMfItem)
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusBadRequest)
		return
	}
	a.micropubCreate(w, r, p)
}

func (a *goBlog) micropubCreate(w http.ResponseWriter, r *http.Request, p *post) {
	if !strings.Contains(r.Context().Value(indieAuthScope).(string), "create") {
		a.serveError(w, r, "create scope missing", http.StatusForbidden)
		return
	}
	if err := a.computeExtraPostParameters(p); err != nil {
		a.serveError(w, r, err.Error(), http.StatusBadRequest)
		return
	}
	if err := a.createPost(p); err != nil {
		a.serveError(w, r, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, a.fullPostURL(p), http.StatusAccepted)
}

func (a *goBlog) micropubDelete(w http.ResponseWriter, r *http.Request, u string) {
	if !strings.Contains(r.Context().Value(indieAuthScope).(string), "delete") {
		a.serveError(w, r, "delete scope missing", http.StatusForbidden)
		return
	}
	uu, err := url.Parse(u)
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusBadRequest)
		return
	}
	if err := a.deletePost(uu.Path); err != nil {
		a.serveError(w, r, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, uu.String(), http.StatusNoContent)
}

func (a *goBlog) micropubUpdate(w http.ResponseWriter, r *http.Request, u string, mf *microformatItem) {
	if !strings.Contains(r.Context().Value(indieAuthScope).(string), "update") {
		a.serveError(w, r, "update scope missing", http.StatusForbidden)
		return
	}
	uu, err := url.Parse(u)
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusBadRequest)
		return
	}
	p, err := a.db.getPost(uu.Path)
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusBadRequest)
		return
	}
	oldPath := p.Path
	oldStatus := p.Status
	a.micropubUpdateReplace(p, mf.Replace)
	a.micropubUpdateAdd(p, mf.Add)
	a.micropubUpdateDelete(p, mf.Delete)
	err = a.computeExtraPostParameters(p)
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	err = a.replacePost(p, oldPath, oldStatus)
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, a.fullPostURL(p), http.StatusNoContent)
}

func (a *goBlog) micropubUpdateReplace(p *post, replace map[string][]interface{}) {
	if content, ok := replace["content"]; ok && len(content) > 0 {
		p.Content = cast.ToStringSlice(content)[0]
	}
	if published, ok := replace["published"]; ok && len(published) > 0 {
		p.Published = cast.ToStringSlice(published)[0]
	}
	if updated, ok := replace["updated"]; ok && len(updated) > 0 {
		p.Updated = cast.ToStringSlice(updated)[0]
	}
	// Status
	statusStr := ""
	if status, ok := replace["post-status"]; ok && len(status) > 0 {
		statusStr = cast.ToStringSlice(status)[0]
	}
	visibilityStr := ""
	if visibility, ok := replace["visibility"]; ok && len(visibility) > 0 {
		visibilityStr = cast.ToStringSlice(visibility)[0]
	}
	if finalStatus := micropubStatus(p.Status, statusStr, visibilityStr); finalStatus != statusNil {
		p.Status = finalStatus
	}
	// Parameters
	if name, ok := replace["name"]; ok && name != nil {
		p.Parameters["title"] = cast.ToStringSlice(name)
	}
	if category, ok := replace["category"]; ok && category != nil {
		p.Parameters[a.cfg.Micropub.CategoryParam] = cast.ToStringSlice(category)
	}
	if reply, ok := replace["in-reply-to"]; ok && reply != nil {
		p.Parameters[a.cfg.Micropub.ReplyParam] = cast.ToStringSlice(reply)
	}
	if like, ok := replace["like-of"]; ok && like != nil {
		p.Parameters[a.cfg.Micropub.LikeParam] = cast.ToStringSlice(like)
	}
	if bookmark, ok := replace["bookmark-of"]; ok && bookmark != nil {
		p.Parameters[a.cfg.Micropub.BookmarkParam] = cast.ToStringSlice(bookmark)
	}
	if audio, ok := replace["audio"]; ok && audio != nil {
		p.Parameters[a.cfg.Micropub.AudioParam] = cast.ToStringSlice(audio)
	}
	// TODO: photos
}

func (a *goBlog) micropubUpdateAdd(p *post, add map[string][]interface{}) {
	for key, value := range add {
		switch key {
		case "content":
			p.Content += strings.TrimSpace(strings.Join(cast.ToStringSlice(value), " "))
		case "published":
			p.Published = strings.TrimSpace(strings.Join(cast.ToStringSlice(value), " "))
		case "updated":
			p.Updated = strings.TrimSpace(strings.Join(cast.ToStringSlice(value), " "))
		case "category":
			p.Parameters[a.cfg.Micropub.CategoryParam] = append(p.Parameters[a.cfg.Micropub.CategoryParam], cast.ToStringSlice(value)...)
		case "in-reply-to":
			p.Parameters[a.cfg.Micropub.ReplyParam] = cast.ToStringSlice(value)
		case "like-of":
			p.Parameters[a.cfg.Micropub.LikeParam] = cast.ToStringSlice(value)
		case "bookmark-of":
			p.Parameters[a.cfg.Micropub.BookmarkParam] = cast.ToStringSlice(value)
		case "audio":
			p.Parameters[a.cfg.Micropub.AudioParam] = append(p.Parameters[a.cfg.Micropub.AudioParam], cast.ToStringSlice(value)...)
			// TODO: photo
		}
	}
}

func (a *goBlog) micropubUpdateDelete(p *post, del interface{}) {
	if del == nil {
		return
	}
	deleteProperties, ok := del.([]interface{})
	if ok {
		// Completely remove properties
		for _, prop := range deleteProperties {
			switch prop {
			case "content":
				p.Content = ""
			case "published":
				p.Published = ""
			case "updated":
				p.Updated = ""
			case "category":
				delete(p.Parameters, a.cfg.Micropub.CategoryParam)
			case "in-reply-to":
				delete(p.Parameters, a.cfg.Micropub.ReplyParam)
				delete(p.Parameters, a.cfg.Micropub.ReplyTitleParam)
			case "like-of":
				delete(p.Parameters, a.cfg.Micropub.LikeParam)
				delete(p.Parameters, a.cfg.Micropub.LikeTitleParam)
			case "bookmark-of":
				delete(p.Parameters, a.cfg.Micropub.BookmarkParam)
			case "audio":
				delete(p.Parameters, a.cfg.Micropub.AudioParam)
			case "photo":
				delete(p.Parameters, a.cfg.Micropub.PhotoParam)
				delete(p.Parameters, a.cfg.Micropub.PhotoDescriptionParam)
			}
		}
		// Return
		return
	}
	toDelete, ok := del.(map[string]interface{})
	if ok {
		// Only delete parts of properties
		for key, values := range toDelete {
			switch key {
			// Properties to completely delete
			case "content":
				p.Content = ""
			case "published":
				p.Published = ""
			case "updated":
				p.Updated = ""
			case "in-reply-to":
				delete(p.Parameters, a.cfg.Micropub.ReplyParam)
				delete(p.Parameters, a.cfg.Micropub.ReplyTitleParam)
			case "like-of":
				delete(p.Parameters, a.cfg.Micropub.LikeParam)
				delete(p.Parameters, a.cfg.Micropub.LikeTitleParam)
			case "bookmark-of":
				delete(p.Parameters, a.cfg.Micropub.BookmarkParam)
			// Properties to delete part of
			// TODO: Support partial deletes of more properties
			case "category":
				delValues := cast.ToStringSlice(values)
				p.Parameters[a.cfg.Micropub.CategoryParam] = funk.FilterString(p.Parameters[a.cfg.Micropub.CategoryParam], func(s string) bool {
					return !funk.ContainsString(delValues, s)
				})
			}
		}
	}
}

func micropubStatus(defaultStatus postStatus, status string, visibility string) (final postStatus) {
	final = defaultStatus
	switch status {
	case "published":
		final = statusPublished
	case "draft":
		final = statusDraft
	}
	if final != statusDraft {
		// Only override status if it's not a draft
		switch visibility {
		case "public":
			final = statusPublished
		case "unlisted":
			final = statusUnlisted
		case "private":
			final = statusPrivate
		}
	}
	return final
}
