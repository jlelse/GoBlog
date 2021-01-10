package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cast"
	"gopkg.in/yaml.v3"
)

const micropubPath = "/micropub"

type micropubConfig struct {
	MediaEndpoint string `json:"media-endpoint,omitempty"`
}

func serveMicropubQuery(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Query().Get("q") {
	case "config":
		w.Header().Add(contentType, contentTypeJSONUTF8)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(&micropubConfig{
			MediaEndpoint: appConfig.Server.PublicAddress + micropubPath + micropubMediaSubPath,
		})
	case "source":
		var mf interface{}
		if urlString := r.URL.Query().Get("url"); urlString != "" {
			u, err := url.Parse(r.URL.Query().Get("url"))
			if err != nil {
				serveError(w, r, err.Error(), http.StatusBadRequest)
				return
			}
			p, err := getPost(u.Path)
			if err != nil {
				serveError(w, r, err.Error(), http.StatusBadRequest)
				return
			}
			mf = p.toMfItem()
		} else {
			limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
			offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
			posts, err := getPosts(&postsRequestConfig{
				limit:  limit,
				offset: offset,
			})
			if err != nil {
				serveError(w, r, err.Error(), http.StatusInternalServerError)
				return
			}
			list := map[string][]*microformatItem{}
			for _, p := range posts {
				list["items"] = append(list["items"], p.toMfItem())
			}
			mf = list
		}
		w.Header().Add(contentType, contentTypeJSONUTF8)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(mf)
	case "category":
		allCategories := []string{}
		for blog := range appConfig.Blogs {
			values, err := allTaxonomyValues(blog, appConfig.Micropub.CategoryParam)
			if err != nil {
				serveError(w, r, err.Error(), http.StatusInternalServerError)
				return
			}
			allCategories = append(allCategories, values...)
		}
		w.Header().Add(contentType, contentTypeJSONUTF8)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"categories": allCategories,
		})
	default:
		serve404(w, r)
	}
}

func (p *post) toMfItem() *microformatItem {
	params := p.Parameters
	params["path"] = []string{p.Path}
	params["section"] = []string{p.Section}
	params["blog"] = []string{p.Blog}
	params["published"] = []string{p.Published}
	params["updated"] = []string{p.Updated}
	pb, _ := yaml.Marshal(p.Parameters)
	content := fmt.Sprintf("---\n%s---\n%s", string(pb), p.Content)
	return &microformatItem{
		Type: []string{"h-entry"},
		Properties: &microformatProperties{
			Name:       p.Parameters["title"],
			Published:  []string{p.Published},
			Updated:    []string{p.Updated},
			Category:   p.Parameters[appConfig.Micropub.CategoryParam],
			Content:    []string{content},
			URL:        []string{p.fullURL()},
			InReplyTo:  p.Parameters[appConfig.Micropub.ReplyParam],
			LikeOf:     p.Parameters[appConfig.Micropub.LikeParam],
			BookmarkOf: p.Parameters[appConfig.Micropub.BookmarkParam],
			MpSlug:     []string{p.Slug},
			Audio:      p.Parameters[appConfig.Micropub.AudioParam],
			// TODO: Photos
		},
	}
}

func serveMicropubPost(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var p *post
	if ct := r.Header.Get(contentType); strings.Contains(ct, contentTypeWWWForm) || strings.Contains(ct, contentTypeMultipartForm) {
		var err error
		if strings.Contains(ct, contentTypeMultipartForm) {
			err = r.ParseMultipartForm(0)
		} else {
			err = r.ParseForm()
		}
		if err != nil {
			serveError(w, r, err.Error(), http.StatusBadRequest)
			return
		}
		if action := micropubAction(r.Form.Get("action")); action != "" {
			u, err := url.Parse(r.Form.Get("url"))
			if err != nil {
				serveError(w, r, err.Error(), http.StatusBadRequest)
				return
			}
			if action == actionDelete {
				micropubDelete(w, r, u)
				return
			}
			serveError(w, r, "Action not supported", http.StatusNotImplemented)
			return
		}
		p, err = convertMPValueMapToPost(r.Form)
		if err != nil {
			serveError(w, r, err.Error(), http.StatusInternalServerError)
			return
		}
	} else if strings.Contains(ct, contentTypeJSON) {
		parsedMfItem := &microformatItem{}
		err := json.NewDecoder(r.Body).Decode(parsedMfItem)
		if err != nil {
			serveError(w, r, err.Error(), http.StatusInternalServerError)
			return
		}
		if parsedMfItem.Action != "" {
			u, err := url.Parse(parsedMfItem.URL)
			if err != nil {
				serveError(w, r, err.Error(), http.StatusBadRequest)
				return
			}
			if parsedMfItem.Action == actionDelete {
				micropubDelete(w, r, u)
				return
			}
			if parsedMfItem.Action == actionUpdate {
				micropubUpdate(w, r, u, parsedMfItem)
				return
			}
			serveError(w, r, "Action not supported", http.StatusNotImplemented)
			return
		}
		p, err = convertMPMfToPost(parsedMfItem)
		if err != nil {
			serveError(w, r, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		serveError(w, r, "wrong content type", http.StatusBadRequest)
		return
	}
	if !strings.Contains(r.Context().Value("scope").(string), "create") {
		serveError(w, r, "create scope missing", http.StatusForbidden)
		return
	}
	err := p.create()
	if err != nil {
		serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, p.fullURL(), http.StatusAccepted)
	return
}

func convertMPValueMapToPost(values map[string][]string) (*post, error) {
	if h, ok := values["h"]; ok && (len(h) != 1 || h[0] != "entry") {
		return nil, errors.New("only entry type is supported so far")
	}
	entry := &post{
		Parameters: map[string][]string{},
	}
	if content, ok := values["content"]; ok {
		entry.Content = content[0]
	}
	if published, ok := values["published"]; ok {
		entry.Published = published[0]
	}
	if updated, ok := values["updated"]; ok {
		entry.Updated = updated[0]
	}
	// Parameter
	if name, ok := values["name"]; ok {
		entry.Parameters["title"] = name
	}
	if category, ok := values["category"]; ok {
		entry.Parameters[appConfig.Micropub.CategoryParam] = category
	} else if categories, ok := values["category[]"]; ok {
		entry.Parameters[appConfig.Micropub.CategoryParam] = categories
	}
	if inReplyTo, ok := values["in-reply-to"]; ok {
		entry.Parameters[appConfig.Micropub.ReplyParam] = inReplyTo
	}
	if likeOf, ok := values["like-of"]; ok {
		entry.Parameters[appConfig.Micropub.LikeParam] = likeOf
	}
	if bookmarkOf, ok := values["bookmark-of"]; ok {
		entry.Parameters[appConfig.Micropub.BookmarkParam] = bookmarkOf
	}
	if audio, ok := values["audio"]; ok {
		entry.Parameters[appConfig.Micropub.AudioParam] = audio
	} else if audio, ok := values["audio[]"]; ok {
		entry.Parameters[appConfig.Micropub.AudioParam] = audio
	}
	if photo, ok := values["photo"]; ok {
		entry.Parameters[appConfig.Micropub.PhotoParam] = photo
	} else if photos, ok := values["photo[]"]; ok {
		entry.Parameters[appConfig.Micropub.PhotoParam] = photos
	}
	if photoAlt, ok := values["mp-photo-alt"]; ok {
		entry.Parameters[appConfig.Micropub.PhotoDescriptionParam] = photoAlt
	} else if photoAlts, ok := values["mp-photo-alt[]"]; ok {
		entry.Parameters[appConfig.Micropub.PhotoDescriptionParam] = photoAlts
	}
	if slug, ok := values["mp-slug"]; ok {
		entry.Slug = slug[0]
	}
	err := entry.computeExtraPostParameters()
	if err != nil {
		return nil, err
	}
	return entry, nil
}

type micropubAction string

const (
	actionUpdate micropubAction = "update"
	actionDelete                = "delete"
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

func convertMPMfToPost(mf *microformatItem) (*post, error) {
	if len(mf.Type) != 1 || mf.Type[0] != "h-entry" {
		return nil, errors.New("only entry type is supported so far")
	}
	entry := &post{
		Parameters: map[string][]string{},
	}
	// Content
	if mf.Properties != nil && len(mf.Properties.Content) == 1 && len(mf.Properties.Content[0]) > 0 {
		entry.Content = mf.Properties.Content[0]
	}
	if len(mf.Properties.Published) == 1 {
		entry.Published = mf.Properties.Published[0]
	}
	if len(mf.Properties.Updated) == 1 {
		entry.Updated = mf.Properties.Updated[0]
	}
	// Parameter
	if len(mf.Properties.Name) == 1 {
		entry.Parameters["title"] = mf.Properties.Name
	}
	if len(mf.Properties.Category) > 0 {
		entry.Parameters[appConfig.Micropub.CategoryParam] = mf.Properties.Category
	}
	if len(mf.Properties.InReplyTo) == 1 {
		entry.Parameters[appConfig.Micropub.ReplyParam] = mf.Properties.InReplyTo
	}
	if len(mf.Properties.LikeOf) == 1 {
		entry.Parameters[appConfig.Micropub.LikeParam] = mf.Properties.LikeOf
	}
	if len(mf.Properties.BookmarkOf) == 1 {
		entry.Parameters[appConfig.Micropub.BookmarkParam] = mf.Properties.BookmarkOf
	}
	if len(mf.Properties.Audio) > 0 {
		entry.Parameters[appConfig.Micropub.AudioParam] = mf.Properties.Audio
	}
	if len(mf.Properties.Photo) > 0 {
		for _, photo := range mf.Properties.Photo {
			if theString, justString := photo.(string); justString {
				entry.Parameters[appConfig.Micropub.PhotoParam] = append(entry.Parameters[appConfig.Micropub.PhotoParam], theString)
				entry.Parameters[appConfig.Micropub.PhotoDescriptionParam] = append(entry.Parameters[appConfig.Micropub.PhotoDescriptionParam], "")
			} else if thePhoto, isPhoto := photo.(map[string]interface{}); isPhoto {
				entry.Parameters[appConfig.Micropub.PhotoParam] = append(entry.Parameters[appConfig.Micropub.PhotoParam], cast.ToString(thePhoto["value"]))
				entry.Parameters[appConfig.Micropub.PhotoDescriptionParam] = append(entry.Parameters[appConfig.Micropub.PhotoDescriptionParam], cast.ToString(thePhoto["alt"]))
			}
		}
	}
	if len(mf.Properties.MpSlug) == 1 {
		entry.Slug = mf.Properties.MpSlug[0]
	}
	err := entry.computeExtraPostParameters()
	if err != nil {
		return nil, err
	}
	return entry, nil

}

func (p *post) computeExtraPostParameters() error {
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
		p.Blog = appConfig.DefaultBlog
	}
	if path := p.Parameters["path"]; len(path) == 1 && path[0] != "" {
		p.Path = path[0]
		delete(p.Parameters, "path")
	}
	if section := p.Parameters["section"]; len(section) == 1 && section[0] != "" {
		p.Section = section[0]
		delete(p.Parameters, "section")
	}
	if slug := p.Parameters["slug"]; len(slug) == 1 && slug[0] != "" {
		p.Slug = slug[0]
		delete(p.Parameters, "slug")
	}
	if published := p.Parameters["published"]; len(published) == 1 && published[0] != "" {
		p.Published = published[0]
		delete(p.Parameters, "published")
	}
	if updated := p.Parameters["updated"]; len(updated) == 1 && updated[0] != "" {
		p.Updated = updated[0]
		delete(p.Parameters, "updated")
	}
	if p.Path == "" && p.Section == "" {
		// Has no path or section -> default section
		p.Section = appConfig.Blogs[p.Blog].DefaultSection
	}
	if p.Published == "" && p.Section != "" {
		// Has no published date, but section -> published now
		p.Published = time.Now().Local().String()
	}
	// Add images not in content
	images := p.Parameters[appConfig.Micropub.PhotoParam]
	imageAlts := p.Parameters[appConfig.Micropub.PhotoDescriptionParam]
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

func micropubDelete(w http.ResponseWriter, r *http.Request, u *url.URL) {
	if !strings.Contains(r.Context().Value("scope").(string), "delete") {
		serveError(w, r, "delete scope missing", http.StatusForbidden)
		return
	}
	if err := deletePost(u.Path); err != nil {
		serveError(w, r, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, u.String(), http.StatusNoContent)
	return
}

func micropubUpdate(w http.ResponseWriter, r *http.Request, u *url.URL, mf *microformatItem) {
	if !strings.Contains(r.Context().Value("scope").(string), "update") {
		serveError(w, r, "update scope missing", http.StatusForbidden)
		return
	}
	p, err := getPost(u.Path)
	if err != nil {
		serveError(w, r, err.Error(), http.StatusBadRequest)
		return
	}
	if mf.Replace != nil {
		for key, value := range mf.Replace {
			switch key {
			case "content":
				p.Content = strings.TrimSpace(strings.Join(cast.ToStringSlice(value), " "))
			case "published":
				p.Published = strings.TrimSpace(strings.Join(cast.ToStringSlice(value), " "))
			case "updated":
				p.Updated = strings.TrimSpace(strings.Join(cast.ToStringSlice(value), " "))
			case "name":
				p.Parameters["title"] = cast.ToStringSlice(value)
			case "category":
				p.Parameters[appConfig.Micropub.CategoryParam] = cast.ToStringSlice(value)
			case "in-reply-to":
				p.Parameters[appConfig.Micropub.ReplyParam] = cast.ToStringSlice(value)
			case "like-of":
				p.Parameters[appConfig.Micropub.LikeParam] = cast.ToStringSlice(value)
			case "bookmark-of":
				p.Parameters[appConfig.Micropub.BookmarkParam] = cast.ToStringSlice(value)
			case "audio":
				p.Parameters[appConfig.Micropub.AudioParam] = cast.ToStringSlice(value)
				// TODO: photo
			}
		}
	}
	if mf.Add != nil {
		for key, value := range mf.Add {
			switch key {
			case "content":
				p.Content += strings.TrimSpace(strings.Join(cast.ToStringSlice(value), " "))
			case "published":
				p.Published = strings.TrimSpace(strings.Join(cast.ToStringSlice(value), " "))
			case "updated":
				p.Updated = strings.TrimSpace(strings.Join(cast.ToStringSlice(value), " "))
			case "category":
				category := p.Parameters[appConfig.Micropub.CategoryParam]
				if category == nil {
					category = []string{}
				}
				p.Parameters[appConfig.Micropub.CategoryParam] = append(category, cast.ToStringSlice(value)...)
			case "in-reply-to":
				p.Parameters[appConfig.Micropub.ReplyParam] = cast.ToStringSlice(value)
			case "like-of":
				p.Parameters[appConfig.Micropub.LikeParam] = cast.ToStringSlice(value)
			case "bookmark-of":
				p.Parameters[appConfig.Micropub.BookmarkParam] = cast.ToStringSlice(value)
			case "audio":
				audio := p.Parameters[appConfig.Micropub.CategoryParam]
				if audio == nil {
					audio = []string{}
				}
				p.Parameters[appConfig.Micropub.AudioParam] = append(audio, cast.ToStringSlice(value)...)
				// TODO: photo
			}
		}
	}
	if del := mf.Delete; del != nil {
		if reflect.TypeOf(del).Kind() == reflect.Slice {
			toDelete, ok := del.([]interface{})
			if ok {
				for _, key := range toDelete {
					switch key {
					case "content":
						p.Content = ""
					case "published":
						p.Published = ""
					case "updated":
						p.Updated = ""
					case "category":
						delete(p.Parameters, appConfig.Micropub.CategoryParam)
					case "in-reply-to":
						delete(p.Parameters, appConfig.Micropub.ReplyParam)
					case "like-of":
						delete(p.Parameters, appConfig.Micropub.LikeParam)
					case "bookmark-of":
						delete(p.Parameters, appConfig.Micropub.BookmarkParam)
					case "audio":
						delete(p.Parameters, appConfig.Micropub.AudioParam)
					case "photo":
						delete(p.Parameters, appConfig.Micropub.PhotoParam)
						delete(p.Parameters, appConfig.Micropub.PhotoDescriptionParam)
					}
				}
			}
		} else {
			toDelete, ok := del.(map[string]interface{})
			if ok {
				for key := range toDelete {
					if ok {
						switch key {
						case "content":
							p.Content = ""
						case "published":
							p.Published = ""
						case "updated":
							p.Updated = ""
						case "in-reply-to":
							delete(p.Parameters, appConfig.Micropub.ReplyParam)
						case "like-of":
							delete(p.Parameters, appConfig.Micropub.LikeParam)
						case "bookmark-of":
							delete(p.Parameters, appConfig.Micropub.BookmarkParam)
							// Use content to edit other parameters
						}
					}
				}
			}
		}
	}
	err = p.computeExtraPostParameters()
	if err != nil {
		serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	err = p.replace()
	if err != nil {
		serveError(w, r, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, p.fullURL(), http.StatusNoContent)
}
