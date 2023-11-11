package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/samber/lo"
	"github.com/spf13/cast"
	"go.goblog.app/app/pkgs/contenttype"
	"gopkg.in/yaml.v3"
)

const micropubPath = "/micropub"

func (a *goBlog) serveMicropubQuery(w http.ResponseWriter, r *http.Request) {
	var result any
	switch query := r.URL.Query(); query.Get("q") {
	case "config":
		channels := a.getMicropubChannelsMap()
		result = map[string]any{
			"channels":       channels,
			"media-endpoint": a.getFullAddress(micropubPath + micropubMediaSubPath),
			"visibility":     []postVisibility{visibilityPublic, visibilityUnlisted, visibilityPrivate},
		}
	case "source":
		if urlString := query.Get("url"); urlString != "" {
			u, err := url.Parse(query.Get("url"))
			if err != nil {
				a.serveError(w, r, err.Error(), http.StatusBadRequest)
				return
			}
			p, err := a.getPost(u.Path)
			if err != nil {
				a.serveError(w, r, err.Error(), http.StatusBadRequest)
				return
			}
			result = a.postToMfItem(p)
		} else {
			posts, err := a.getPosts(&postsRequestConfig{
				limit:  stringToInt(query.Get("limit")),
				offset: stringToInt(query.Get("offset")),
			})
			if err != nil {
				a.serveError(w, r, err.Error(), http.StatusInternalServerError)
				return
			}
			list := map[string][]*microformatItem{}
			for _, p := range posts {
				list["items"] = append(list["items"], a.postToMfItem(p))
			}
			result = list
		}
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
		result = map[string]any{"categories": allCategories}
	case "channel":
		channels := a.getMicropubChannelsMap()
		result = map[string]any{"channels": channels}
	default:
		a.serve404(w, r)
		return
	}
	a.respondWithMinifiedJson(w, result)
}

func (a *goBlog) getMicropubChannelsMap() []map[string]any {
	channels := []map[string]any{}
	for b, bc := range a.cfg.Blogs {
		channels = append(channels, map[string]any{
			"name": fmt.Sprintf("%s: %s", b, bc.Title),
			"uid":  b,
		})
		for s, sc := range bc.Sections {
			channels = append(channels, map[string]any{
				"name": fmt.Sprintf("%s/%s: %s", b, s, sc.Name),
				"uid":  fmt.Sprintf("%s/%s", b, s),
			})
		}
	}
	return channels
}

func (a *goBlog) serveMicropubPost(w http.ResponseWriter, r *http.Request) {
	blog, _ := a.getBlog(r)
	p := &post{Blog: blog}
	switch mt, _, _ := mime.ParseMediaType(r.Header.Get(contentType)); mt {
	case contenttype.WWWForm, contenttype.MultipartForm:
		_ = r.ParseMultipartForm(0)
		if r.Form == nil {
			a.serveError(w, r, "Failed to parse form", http.StatusBadRequest)
			return
		}
		if action := micropubAction(r.Form.Get("action")); action != "" {
			switch action {
			case actionDelete:
				a.micropubDelete(w, r, r.Form.Get("url"))
			case actionUndelete:
				a.micropubUndelete(w, r, r.Form.Get("url"))
			default:
				a.serveError(w, r, "Action not supported", http.StatusNotImplemented)
			}
			return
		}
		a.micropubCreatePostFromForm(w, r, p)
	case contenttype.JSON:
		parsedMfItem := &microformatItem{}
		err := json.NewDecoder(r.Body).Decode(parsedMfItem)
		if err != nil {
			a.serveError(w, r, err.Error(), http.StatusBadRequest)
			return
		}
		if parsedMfItem.Action != "" {
			switch parsedMfItem.Action {
			case actionDelete:
				a.micropubDelete(w, r, parsedMfItem.URL)
			case actionUndelete:
				a.micropubUndelete(w, r, parsedMfItem.URL)
			case actionUpdate:
				a.micropubUpdate(w, r, parsedMfItem.URL, parsedMfItem)
			default:
				a.serveError(w, r, "Action not supported", http.StatusNotImplemented)
			}
			return
		}
		a.micropubCreatePostFromJson(w, r, p, parsedMfItem)
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
	if channel, ok := values["mp-channel"]; ok && len(channel) > 0 {
		entry.setChannel(channel[0])
		delete(values, "mp-channel")
	}
	// Status
	if status, ok := values["post-status"]; ok && len(status) > 0 {
		statusStr := status[0]
		entry.Status = micropubStatus(statusStr)
		delete(values, "post-status")
	}
	// Visibility
	if visibility, ok := values["visibility"]; ok && len(visibility) > 0 {
		visibilityStr := visibility[0]
		entry.Visibility = micropubVisibility(visibilityStr)
		delete(values, "visibility")
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
	actionUpdate   micropubAction = "update"
	actionDelete   micropubAction = "delete"
	actionUndelete micropubAction = "undelete"
)

type microformatItem struct {
	Type       []string               `json:"type,omitempty"`
	URL        string                 `json:"url,omitempty"`
	Action     micropubAction         `json:"action,omitempty"`
	Properties *microformatProperties `json:"properties,omitempty"`
	Replace    map[string][]any       `json:"replace,omitempty"`
	Add        map[string][]any       `json:"add,omitempty"`
	Delete     any                    `json:"delete,omitempty"`
}

type microformatProperties struct {
	Name       []string `json:"name,omitempty"`
	Published  []string `json:"published,omitempty"`
	Updated    []string `json:"updated,omitempty"`
	PostStatus []string `json:"post-status,omitempty"`
	Visibility []string `json:"visibility,omitempty"`
	Category   []string `json:"category,omitempty"`
	Content    []string `json:"content,omitempty"`
	URL        []string `json:"url,omitempty"`
	InReplyTo  []string `json:"in-reply-to,omitempty"`
	LikeOf     []string `json:"like-of,omitempty"`
	BookmarkOf []string `json:"bookmark-of,omitempty"`
	MpSlug     []string `json:"mp-slug,omitempty"`
	Photo      []any    `json:"photo,omitempty"`
	Audio      []string `json:"audio,omitempty"`
	MpChannel  []string `json:"mp-channel,omitempty"`
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
	if len(mf.Properties.MpChannel) > 0 {
		entry.setChannel(mf.Properties.MpChannel[0])
	}
	// Status
	if len(mf.Properties.PostStatus) > 0 {
		status := mf.Properties.PostStatus[0]
		entry.Status = micropubStatus(status)
	}
	// Visibility
	if len(mf.Properties.Visibility) > 0 {
		visibility := mf.Properties.Visibility[0]
		entry.Visibility = micropubVisibility(visibility)
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
			} else if thePhoto, isPhoto := photo.(map[string]any); isPhoto {
				entry.Parameters[a.cfg.Micropub.PhotoParam] = append(entry.Parameters[a.cfg.Micropub.PhotoParam], cast.ToString(thePhoto["value"]))
				entry.Parameters[a.cfg.Micropub.PhotoDescriptionParam] = append(entry.Parameters[a.cfg.Micropub.PhotoDescriptionParam], cast.ToString(thePhoto["alt"]))
			}
		}
	}
	return nil
}

func (a *goBlog) extractParamsFromContent(p *post) error {
	if p.Parameters == nil {
		p.Parameters = map[string][]string{}
	}
	p.Content = regexp.MustCompile("\r\n").ReplaceAllString(p.Content, "\n")
	if split := strings.Split(p.Content, "---\n"); len(split) >= 3 && strings.TrimSpace(split[0]) == "" {
		// Contains frontmatter
		fm := split[1]
		meta := map[string]any{}
		err := yaml.Unmarshal([]byte(fm), &meta)
		if err != nil {
			return err
		}
		// Find section and copy frontmatter to params
		for key, value := range meta {
			// Delete existing content - replace
			p.Parameters[key] = []string{}
			if a, ok := value.([]any); ok {
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
	if visibility := p.Parameters["visibility"]; len(visibility) == 1 {
		p.Visibility = postVisibility(visibility[0])
		delete(p.Parameters, "visibility")
	}
	if priority := p.Parameters["priority"]; len(priority) == 1 {
		p.Priority = cast.ToInt(priority[0])
		delete(p.Parameters, "priority")
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

func (a *goBlog) micropubCreatePostFromForm(w http.ResponseWriter, r *http.Request, p *post) {
	err := a.micropubParseValuePostParamsValueMap(p, r.Form)
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusBadRequest)
		return
	}
	a.micropubCreate(w, r, p)
}

func (a *goBlog) micropubCreatePostFromJson(w http.ResponseWriter, r *http.Request, p *post, parsedMfItem *microformatItem) {
	err := a.micropubParsePostParamsMfItem(p, parsedMfItem)
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusBadRequest)
		return
	}
	a.micropubCreate(w, r, p)
}

func (a *goBlog) micropubCheckScope(w http.ResponseWriter, r *http.Request, required string) bool {
	if !strings.Contains(r.Context().Value(indieAuthScope).(string), required) {
		a.serveError(w, r, required+" scope missing", http.StatusForbidden)
		return false
	}
	return true
}

func (a *goBlog) micropubCreate(w http.ResponseWriter, r *http.Request, p *post) {
	if !a.micropubCheckScope(w, r, "create") {
		return
	}
	if err := a.extractParamsFromContent(p); err != nil {
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
	if !a.micropubCheckScope(w, r, "delete") {
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

func (a *goBlog) micropubUndelete(w http.ResponseWriter, r *http.Request, u string) {
	if !a.micropubCheckScope(w, r, "undelete") {
		return
	}
	uu, err := url.Parse(u)
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusBadRequest)
		return
	}
	if err := a.undeletePost(uu.Path); err != nil {
		a.serveError(w, r, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, uu.String(), http.StatusNoContent)
}

func (a *goBlog) micropubUpdate(w http.ResponseWriter, r *http.Request, u string, mf *microformatItem) {
	if !a.micropubCheckScope(w, r, "update") {
		return
	}
	uu, err := url.Parse(u)
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusBadRequest)
		return
	}
	ppath := uu.Path
	if ppath == "" {
		// Probably homepage "/"
		ppath = "/"
	}
	p, err := a.getPost(ppath)
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusBadRequest)
		return
	}
	// Check if post is marked as deleted
	if p.Deleted() {
		a.serveError(w, r, "post is marked as deleted, undelete it first", http.StatusBadRequest)
		return
	}
	// Update post
	oldPath := p.Path
	oldStatus := p.Status
	oldVisibility := p.Visibility
	a.micropubUpdateReplace(p, mf.Replace)
	a.micropubUpdateAdd(p, mf.Add)
	a.micropubUpdateDelete(p, mf.Delete)
	err = a.extractParamsFromContent(p)
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	err = a.replacePost(p, oldPath, oldStatus, oldVisibility)
	if err != nil {
		a.serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, a.fullPostURL(p), http.StatusNoContent)
}

func (a *goBlog) micropubUpdateReplace(p *post, replace map[string][]any) {
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
	if status, ok := replace["post-status"]; ok && len(status) > 0 {
		statusStr := cast.ToStringSlice(status)[0]
		p.Status = micropubStatus(statusStr)
	}
	// Visibility
	if visibility, ok := replace["visibility"]; ok && len(visibility) > 0 {
		visibilityStr := cast.ToStringSlice(visibility)[0]
		p.Visibility = micropubVisibility(visibilityStr)
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

func (a *goBlog) micropubUpdateAdd(p *post, add map[string][]any) {
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

func (a *goBlog) micropubUpdateDelete(p *post, del any) {
	if del == nil {
		return
	}
	deleteProperties, ok := del.([]any)
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
				delete(p.Parameters, a.cfg.Micropub.ReplyContextParam)
			case "like-of":
				delete(p.Parameters, a.cfg.Micropub.LikeParam)
				delete(p.Parameters, a.cfg.Micropub.LikeTitleParam)
				delete(p.Parameters, a.cfg.Micropub.LikeContextParam)
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
	toDelete, ok := del.(map[string]any)
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
				p.Parameters[a.cfg.Micropub.CategoryParam] = lo.Filter(p.Parameters[a.cfg.Micropub.CategoryParam], func(s string, _ int) bool {
					return !lo.Contains(delValues, s)
				})
			}
		}
	}
}

func micropubStatus(status string) postStatus {
	switch status {
	case "draft":
		return statusDraft
	default:
		return statusPublished
	}
}

func micropubVisibility(visibility string) postVisibility {
	switch visibility {
	case "unlisted":
		return visibilityUnlisted
	case "private":
		return visibilityPrivate
	default:
		return visibilityPublic
	}
}
