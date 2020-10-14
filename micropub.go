package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cast"
	"gopkg.in/yaml.v3"
)

const micropubPath = "/micropub"
const micropubMediaSubPath = "/media"

type micropubConfig struct {
	MediaEndpoint string `json:"media-endpoint,omitempty"`
}

func serveMicropubQuery(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Query().Get("q") {
	case "config":
		w.Header().Add(contentType, contentTypeJSON)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(&micropubConfig{
			// TODO: Uncomment when media endpoint implemented
			// MediaEndpoint: appConfig.Server.PublicAddress + micropubMediaPath,
		})
	case "source":
		var mf interface{}
		if urlString := r.URL.Query().Get("url"); urlString != "" {
			u, err := url.Parse(r.URL.Query().Get("url"))
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			post, err := getPost(r.Context(), u.Path)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			mf = post.toMfItem()
		} else {
			posts, err := getPosts(r.Context(), &postsRequestConfig{})
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			list := map[string][]*microformatItem{}
			for _, post := range posts {
				list["items"] = append(list["items"], post.toMfItem())
			}
			mf = list
		}
		w.Header().Add(contentType, contentTypeJSON)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(mf)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (post *Post) toMfItem() *microformatItem {
	params := post.Parameters
	params["path"] = []string{post.Path}
	params["section"] = []string{post.Section}
	params["blog"] = []string{post.Blog}
	pb, _ := yaml.Marshal(post.Parameters)
	content := fmt.Sprintf("---\n%s---\n%s", string(pb), post.Content)
	return &microformatItem{
		Type: []string{"h-entry"},
		Properties: &microformatProperties{
			Name:      post.Parameters["title"],
			Published: []string{post.Published},
			Updated:   []string{post.Updated},
			Content:   []string{content},
			MpSlug:    []string{post.Slug},
			Category:  post.Parameters[appConfig.Micropub.CategoryParam],
		},
	}
}

func serveMicropubPost(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var post *Post
	if ct := r.Header.Get(contentType); strings.Contains(ct, contentTypeWWWForm) || strings.Contains(ct, contentTypeMultipartForm) {
		var err error
		r.ParseForm()
		if strings.Contains(ct, contentTypeMultipartForm) {
			err := r.ParseMultipartForm(0)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		if action := micropubAction(r.Form.Get("action")); action != "" {
			u, err := url.Parse(r.Form.Get("url"))
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
			}
			if action == actionDelete {
				micropubDelete(w, r, u)
				return
			}
			http.Error(w, "Action not supported", http.StatusNotImplemented)
			return
		}
		post, err = convertMPValueMapToPost(r.Form)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else if strings.Contains(ct, contentTypeJSON) {
		parsedMfItem := &microformatItem{}
		err := json.NewDecoder(r.Body).Decode(parsedMfItem)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if parsedMfItem.Action != "" {
			u, err := url.Parse(parsedMfItem.URL)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
			}
			if parsedMfItem.Action == actionDelete {
				micropubDelete(w, r, u)
				return
			}
			if parsedMfItem.Action == actionUpdate {
				micropubUpdate(w, r, u, parsedMfItem)
				return
			}
			http.Error(w, "Action not supported", http.StatusNotImplemented)
			return
		}
		post, err = convertMPMfToPost(parsedMfItem)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		http.Error(w, "wrong content type", http.StatusBadRequest)
		return
	}
	if !strings.Contains(r.Context().Value("scope").(string), "create") {
		http.Error(w, "create scope missing", http.StatusForbidden)
	}
	err := post.create()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Add("Location", appConfig.Server.PublicAddress+post.Path)
	w.WriteHeader(http.StatusAccepted)
	return
}

func convertMPValueMapToPost(values map[string][]string) (*Post, error) {
	if h, ok := values["h"]; ok && (len(h) != 1 || h[0] != "entry") {
		return nil, errors.New("only entry type is supported so far")
	}
	entry := &Post{
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

func convertMPMfToPost(mf *microformatItem) (*Post, error) {
	if len(mf.Type) != 1 || mf.Type[0] != "h-entry" {
		return nil, errors.New("only entry type is supported so far")
	}
	entry := &Post{}
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

func (post *Post) computeExtraPostParameters() error {
	post.Content = regexp.MustCompile("\r\n").ReplaceAllString(post.Content, "\n")
	if split := strings.Split(post.Content, "---\n"); len(split) >= 3 && len(strings.TrimSpace(split[0])) == 0 {
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
			post.Parameters[key] = []string{}
			if a, ok := value.([]interface{}); ok {
				for _, ae := range a {
					post.Parameters[key] = append(post.Parameters[key], cast.ToString(ae))
				}
			} else {
				post.Parameters[key] = append(post.Parameters[key], cast.ToString(value))
			}
		}
		// Remove frontmatter from content
		post.Content = strings.Join(split[2:], "---\n")
	}
	// Check settings
	if blog := post.Parameters["blog"]; len(blog) == 1 && blog[0] != "" {
		post.Blog = blog[0]
		delete(post.Parameters, "blog")
	} else {
		post.Blog = appConfig.DefaultBlog
	}
	if path := post.Parameters["path"]; len(path) == 1 && path[0] != "" {
		post.Path = path[0]
		delete(post.Parameters, "path")
	}
	if section := post.Parameters["section"]; len(section) == 1 && section[0] != "" {
		post.Section = section[0]
		delete(post.Parameters, "section")
	}
	if slug := post.Parameters["slug"]; len(slug) == 1 && slug[0] != "" {
		post.Slug = slug[0]
		delete(post.Parameters, "slug")
	}
	if post.Path == "" && post.Section == "" {
		// Has no path or section -> default section
		post.Section = appConfig.Blogs[post.Blog].DefaultSection
	}
	if post.Published == "" && post.Section != "" {
		// Has no published date, but section -> published now
		post.Published = time.Now().String()
	}
	// Add images not in content
	images := post.Parameters[appConfig.Micropub.PhotoParam]
	imageAlts := post.Parameters[appConfig.Micropub.PhotoDescriptionParam]
	useAlts := len(images) == len(imageAlts)
	for i, image := range images {
		if !strings.Contains(post.Content, image) {
			if useAlts && len(imageAlts[i]) > 0 {
				post.Content += "\n\n![" + imageAlts[i] + "](" + image + " \"" + imageAlts[i] + "\")"
			} else {
				post.Content += "\n\n![](" + image + ")"
			}
		}
	}
	return nil
}

func micropubDelete(w http.ResponseWriter, r *http.Request, u *url.URL) {
	if !strings.Contains(r.Context().Value("scope").(string), "delete") {
		http.Error(w, "delete scope missing", http.StatusForbidden)
	}
	err := deletePost(u.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	} else {
		w.WriteHeader(http.StatusNoContent)
	}
	return
}

func micropubUpdate(w http.ResponseWriter, r *http.Request, u *url.URL, mf *microformatItem) {
	if !strings.Contains(r.Context().Value("scope").(string), "update") {
		http.Error(w, "update scope missing", http.StatusForbidden)
	}
	post, err := getPost(r.Context(), u.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if mf.Replace != nil {
		for key, value := range mf.Replace {
			switch key {
			case "content":
				post.Content = strings.TrimSpace(strings.Join(cast.ToStringSlice(value), " "))
			case "published":
				post.Published = strings.TrimSpace(strings.Join(cast.ToStringSlice(value), " "))
			case "updated":
				post.Updated = strings.TrimSpace(strings.Join(cast.ToStringSlice(value), " "))
			case "name":
				post.Parameters["title"] = cast.ToStringSlice(value)
			case "category":
				post.Parameters[appConfig.Micropub.CategoryParam] = cast.ToStringSlice(value)
			case "in-reply-to":
				post.Parameters[appConfig.Micropub.ReplyParam] = cast.ToStringSlice(value)
			case "like-of":
				post.Parameters[appConfig.Micropub.LikeParam] = cast.ToStringSlice(value)
			case "bookmark-of":
				post.Parameters[appConfig.Micropub.BookmarkParam] = cast.ToStringSlice(value)
			case "audio":
				post.Parameters[appConfig.Micropub.AudioParam] = cast.ToStringSlice(value)
				// TODO: photo
			}
		}
	}
	if mf.Add != nil {
		for key, value := range mf.Add {
			switch key {
			case "content":
				post.Content += strings.TrimSpace(strings.Join(cast.ToStringSlice(value), " "))
			case "published":
				post.Published = strings.TrimSpace(strings.Join(cast.ToStringSlice(value), " "))
			case "updated":
				post.Updated = strings.TrimSpace(strings.Join(cast.ToStringSlice(value), " "))
			case "category":
				category := post.Parameters[appConfig.Micropub.CategoryParam]
				if category == nil {
					category = []string{}
				}
				post.Parameters[appConfig.Micropub.CategoryParam] = append(category, cast.ToStringSlice(value)...)
			case "in-reply-to":
				post.Parameters[appConfig.Micropub.ReplyParam] = cast.ToStringSlice(value)
			case "like-of":
				post.Parameters[appConfig.Micropub.LikeParam] = cast.ToStringSlice(value)
			case "bookmark-of":
				post.Parameters[appConfig.Micropub.BookmarkParam] = cast.ToStringSlice(value)
			case "audio":
				audio := post.Parameters[appConfig.Micropub.CategoryParam]
				if audio == nil {
					audio = []string{}
				}
				post.Parameters[appConfig.Micropub.AudioParam] = append(audio, cast.ToStringSlice(value)...)
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
						post.Content = ""
					case "published":
						post.Published = ""
					case "updated":
						post.Updated = ""
					case "category":
						delete(post.Parameters, appConfig.Micropub.CategoryParam)
					case "in-reply-to":
						delete(post.Parameters, appConfig.Micropub.ReplyParam)
					case "like-of":
						delete(post.Parameters, appConfig.Micropub.LikeParam)
					case "bookmark-of":
						delete(post.Parameters, appConfig.Micropub.BookmarkParam)
					case "audio":
						delete(post.Parameters, appConfig.Micropub.AudioParam)
					case "photo":
						delete(post.Parameters, appConfig.Micropub.PhotoParam)
						delete(post.Parameters, appConfig.Micropub.PhotoDescriptionParam)
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
							post.Content = ""
						case "published":
							post.Published = ""
						case "updated":
							post.Updated = ""
						case "in-reply-to":
							delete(post.Parameters, appConfig.Micropub.ReplyParam)
						case "like-of":
							delete(post.Parameters, appConfig.Micropub.LikeParam)
						case "bookmark-of":
							delete(post.Parameters, appConfig.Micropub.BookmarkParam)
							// Use content to edit other parameters
						}
					}
				}
			}
		}
	}
	err = post.computeExtraPostParameters()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = post.replace()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func serveMicropubMedia(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement media server
}
