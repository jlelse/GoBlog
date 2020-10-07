package main

import (
	"errors"
	"net/http"
	"strings"

	"github.com/spf13/cast"
	"gopkg.in/yaml.v3"
)

const micropubMediaSubPath = "/media"

type micropubConfig struct {
	MediaEndpoint string `json:"media-endpoint,omitempty"`
}

func serveMicropubQuery(w http.ResponseWriter, r *http.Request) {
	if q := r.URL.Query().Get("q"); q == "config" {
		w.Header().Add(contentType, contentTypeJSON)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(&micropubConfig{
			// TODO: Uncomment when media endpoint implemented
			// MediaEndpoint: appConfig.Server.PublicAddress + micropubMediaPath,
		})
	} else {
		w.Header().Add(contentType, contentTypeJSON)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}
}

func serveMicropubPost(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var post *Post
	if ct := r.Header.Get(contentType); strings.Contains(ct, contentTypeWWWForm) {
		err := r.ParseForm()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		post, err = convertMPValueMapToPost(r.Form)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else if strings.Contains(ct, contentTypeMultipartForm) {
		err := r.ParseMultipartForm(1024 * 1024 * 16)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		post, err = convertMPValueMapToPost(r.MultipartForm.Value)
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
		post, err = convertMPMfToPost(parsedMfItem)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		http.Error(w, "wrong content type", http.StatusBadRequest)
		return
	}
	err := post.createOrReplace(true)
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
	err := computeExtraPostParameters(entry)
	if err != nil {
		return nil, err
	}
	return entry, nil
}

type microformatItem struct {
	Type       []string               `json:"type"`
	Properties *microformatProperties `json:"properties"`
}

type microformatProperties struct {
	Name       []string      `json:"name"`
	Published  []string      `json:"published"`
	Updated    []string      `json:"updated"`
	Category   []string      `json:"category"`
	Content    []string      `json:"content"`
	URL        []string      `json:"url"`
	InReplyTo  []string      `json:"in-reply-to"`
	LikeOf     []string      `json:"like-of"`
	BookmarkOf []string      `json:"bookmark-of"`
	MpSlug     []string      `json:"mp-slug"`
	Photo      []interface{} `json:"photo"`
	Audio      []string      `json:"audio"`
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
	err := computeExtraPostParameters(entry)
	if err != nil {
		return nil, err
	}
	return entry, nil

}

func computeExtraPostParameters(entry *Post) error {
	// Add images not in content
	images := entry.Parameters[appConfig.Micropub.PhotoParam]
	imageAlts := entry.Parameters[appConfig.Micropub.PhotoDescriptionParam]
	useAlts := len(images) == len(imageAlts)
	for i, image := range images {
		if !strings.Contains(entry.Content, image) {
			if useAlts && len(imageAlts[i]) > 0 {
				entry.Content += "\n\n![" + imageAlts[i] + "](" + image + " \"" + imageAlts[i] + "\")"
			} else {
				entry.Content += "\n\n![](" + image + ")"
			}
		}
	}
	sep := "---\n"
	if split := strings.Split(entry.Content, sep); len(split) > 2 {
		// Contains frontmatter
		fm := split[1]
		meta := map[string]interface{}{}
		err := yaml.Unmarshal([]byte(fm), &meta)
		if err != nil {
			return err
		}
		// Find section and copy frontmatter to params
		for key, value := range meta {
			if a, ok := value.([]interface{}); ok {
				for _, ae := range a {
					entry.Parameters[key] = append(entry.Parameters[key], cast.ToString(ae))
				}
			} else {
				entry.Parameters[key] = append(entry.Parameters[key], cast.ToString(value))
			}
		}
		// Remove frontmatter from content
		entry.Content = strings.Replace(entry.Content, split[0]+sep+split[1]+sep, "", 1)
	}
	// Check settings
	if blog := entry.Parameters["blog"]; len(blog) == 1 && blog[0] != "" {
		entry.Blog = blog[0]
		delete(entry.Parameters, "blog")
	} else {
		entry.Blog = appConfig.DefaultBlog
	}
	if path := entry.Parameters["path"]; len(path) == 1 && path[0] != "" {
		entry.Path = path[0]
		delete(entry.Parameters, "path")
	}
	if section := entry.Parameters["section"]; len(section) == 1 && section[0] != "" {
		entry.Section = section[0]
		delete(entry.Parameters, "section")
	}
	if slug := entry.Parameters["slug"]; len(slug) == 1 && slug[0] != "" {
		entry.Slug = slug[0]
		delete(entry.Parameters, "slug")
	}
	if entry.Path == "" && entry.Section == "" {
		entry.Section = appConfig.Blogs[entry.Blog].DefaultSection
	}
	return nil
}

func serveMicropubMedia(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement media server
}
