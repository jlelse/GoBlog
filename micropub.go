package main

import (
	"cmp"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"maps"
	"mime"
	"mime/multipart"
	"net/http"
	urlpkg "net/url"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"

	"github.com/samber/lo"
	"github.com/spf13/cast"
	"go.goblog.app/app/pkgs/bodylimit"
	"go.hacdias.com/indielib/micropub"
	"gopkg.in/yaml.v3"
)

func (a *goBlog) getMicropubImplementation() *micropubImplementation {
	if a.mpImpl == nil {
		a.mpImpl = &micropubImplementation{a: a}
	}
	return a.mpImpl
}

const (
	micropubPath         = "/micropub"
	micropubMediaSubPath = "/media"
)

type micropubImplementation struct {
	a  *goBlog
	h  http.Handler
	mh http.Handler
}

func (s *micropubImplementation) getHandler() http.Handler {
	if s.h == nil {
		s.h = micropub.NewHandler(
			s,
			micropub.WithMediaEndpoint(s.a.getFullAddress(micropubPath+micropubMediaSubPath)),
			micropub.WithGetCategories(s.getCategories),
			micropub.WithGetChannels(s.getChannels),
			micropub.WithGetVisibility(s.getVisibility),
		)
	}
	return s.h
}

func (s *micropubImplementation) getCategories() []string {
	allCategories := []string{}
	for blog := range s.a.cfg.Blogs {
		values, _ := s.a.db.allTaxonomyValues(blog, s.a.cfg.Micropub.CategoryParam)
		allCategories = append(allCategories, values...)
	}
	return lo.Uniq(allCategories)
}

func (s *micropubImplementation) getChannels() []micropub.Channel {
	allChannels := []micropub.Channel{}
	for b, bc := range s.a.cfg.Blogs {
		allChannels = append(allChannels, micropub.Channel{
			Name: fmt.Sprintf("%s: %s", b, bc.Title),
			UID:  b,
		})
		for s, sc := range bc.Sections {
			allChannels = append(allChannels, micropub.Channel{
				Name: fmt.Sprintf("%s/%s: %s", b, s, sc.Name),
				UID:  fmt.Sprintf("%s/%s", b, s),
			})
		}
	}
	return allChannels
}

func (s *micropubImplementation) getVisibility() []string {
	return []string{string(visibilityPrivate), string(visibilityUnlisted), string(visibilityPublic)}
}

func (s *micropubImplementation) getMediaHandler() http.Handler {
	if s.mh == nil {
		s.mh = micropub.NewMediaHandler(
			s.UploadMedia,
			s.HasScope,
			micropub.WithMaxMemory(0),
			micropub.WithMaxMediaSize(30*bodylimit.MB),
		)
	}
	return s.mh
}

func (s *micropubImplementation) HasScope(r *http.Request, scope string) bool {
	return strings.Contains(r.Context().Value(indieAuthScope).(string), scope)
}

func (s *micropubImplementation) Source(urlStr string) (map[string]any, error) {
	url, err := urlpkg.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", micropub.ErrBadRequest, err)
	}
	p, err := s.a.getPost(url.Path)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", micropub.ErrBadRequest, err)
	}
	return s.a.postToMfMap(p), nil
}

func (s *micropubImplementation) SourceMany(limit, offset int) ([]map[string]any, error) {
	posts, err := s.a.getPosts(&postsRequestConfig{
		limit:  limit,
		offset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", micropub.ErrBadRequest, err)
	}
	list := []map[string]any{}
	for _, p := range posts {
		list = append(list, s.a.postToMfMap(p))
	}
	return list, nil
}

func (s *micropubImplementation) Create(req *micropub.Request) (string, error) {
	if req.Type != "h-entry" {
		return "", fmt.Errorf("%w: only h-entry supported", micropub.ErrNotImplemented)
	}
	entry := &post{}
	entry.Parameters = map[string][]string{}
	allValues := lo.Assign(req.Properties, req.Commands)
	// Parameters with special care
	for photoNo, photo := range allValues["photo"] {
		pp := s.a.cfg.Micropub.PhotoParam
		pdp := s.a.cfg.Micropub.PhotoDescriptionParam
		if photoLink, isPhotoLink := photo.(string); isPhotoLink {
			entry.Parameters[pp] = append(entry.Parameters[pp], photoLink)
			if len(allValues["photo-alt"]) > photoNo && allValues["photo-alt"][photoNo] != nil {
				entry.Parameters[pdp] = append(entry.Parameters[pdp], cast.ToString(allValues["photo-alt"][photoNo]))
			} else {
				entry.Parameters[pdp] = append(entry.Parameters[pdp], "")
			}
		} else if photoObject, isPhotoObject := photo.(map[string]any); isPhotoObject {
			entry.Parameters[pp] = append(entry.Parameters[pp], cast.ToString(photoObject["value"]))
			entry.Parameters[pdp] = append(entry.Parameters[pdp], cast.ToString(photoObject["alt"]))
		}
	}
	delete(allValues, "photo")
	delete(allValues, "photo-alt")
	delete(allValues, "file") // Micropublish.net fix
	// Rest of parameters
	for key, values := range allValues {
		values := cast.ToStringSlice(values)
		if len(values) == 0 {
			continue
		}
		switch key {
		case "content":
			entry.Content = values[0]
		case "published":
			entry.Published = values[0]
		case "updated":
			entry.Updated = values[0]
		case "slug":
			entry.Slug = values[0]
		case "channel":
			entry.setChannel(values[0])
		case "post-status":
			entry.Status = micropubStatus(values[0])
		case "visibility":
			entry.Visibility = micropubVisibility(values[0])
		default:
			entry.Parameters[s.mapToParameterName(key)] = values
		}
	}
	if err := s.a.processContentAndParameters(entry); err != nil {
		return "", fmt.Errorf("%w: %w", micropub.ErrBadRequest, err)
	}
	if err := s.a.createPost(entry); err != nil {
		return "", fmt.Errorf("%w: %w", micropub.ErrBadRequest, err)
	}
	return s.a.fullPostURL(entry), nil
}

func (s *micropubImplementation) Update(req *micropub.Request) (string, error) {
	// Get editor options
	editorOptions := req.Updates.Replace["goblog-editor"]
	if editorOptions == nil {
		editorOptions = []any{}
	}
	// Get post
	url, err := urlpkg.Parse(req.URL)
	if err != nil {
		return "", fmt.Errorf("%w: %w", micropub.ErrBadRequest, err)
	}
	postPath := cmp.Or(url.Path, "/")
	entry, err := s.a.getPost(postPath)
	if err != nil {
		return "", fmt.Errorf("%w: %w", micropub.ErrBadRequest, err)
	}
	// Check if post is marked as deleted
	if entry.Deleted() {
		return "", fmt.Errorf("%w: post is marked as deleted, undelete it first", micropub.ErrBadRequest)
	}
	// Update post
	oldPath := entry.Path
	oldStatus := entry.Status
	oldVisibility := entry.Visibility
	if entry.Parameters == nil {
		entry.Parameters = map[string][]string{}
	}
	// Update properties
	properties := s.a.postMfProperties(entry, false)
	properties, err = micropubUpdateMfProperties(properties, req.Updates)
	if err != nil {
		return "", fmt.Errorf("failed to update properties: %w", err)
	}
	s.updatePostPropertiesFromMf(entry, properties)
	err = s.a.processContentAndParameters(entry)
	if err != nil {
		return "", fmt.Errorf("%w: %w", micropub.ErrBadRequest, err)
	}
	err = s.a.replacePost(entry, oldPath, oldStatus, oldVisibility, slices.Contains(editorOptions, "noupdated"))
	if err != nil {
		return "", fmt.Errorf("%w: %w", micropub.ErrBadRequest, err)
	}
	return s.a.fullPostURL(entry), nil
}

func (s *micropubImplementation) Delete(urlStr string) error {
	url, err := urlpkg.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("%w: %w", micropub.ErrBadRequest, err)
	}
	if err := s.a.deletePost(url.Path); err != nil {
		return fmt.Errorf("%w: %w", micropub.ErrBadRequest, err)
	}
	return nil
}

func (s *micropubImplementation) Undelete(urlStr string) error {
	url, err := urlpkg.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("%w: %w", micropub.ErrBadRequest, err)
	}
	if err := s.a.undeletePost(url.Path); err != nil {
		return fmt.Errorf("%w: %w", micropub.ErrBadRequest, err)
	}
	return nil
}

func (s *micropubImplementation) UploadMedia(file multipart.File, header *multipart.FileHeader) (string, error) {
	// Generate sha256 hash for file
	hash := sha256.New()
	_, err := io.Copy(hash, file)
	if err != nil {
		return "", fmt.Errorf("%w: failed to get file hash", micropub.ErrBadRequest)
	}
	// Get file extension
	fileExtension := filepath.Ext(header.Filename)
	if fileExtension == "" {
		// Find correct file extension if original filename does not contain one
		mimeType := header.Header.Get(contentType)
		if len(mimeType) > 0 {
			allExtensions, _ := mime.ExtensionsByType(mimeType)
			if len(allExtensions) > 0 {
				fileExtension = allExtensions[0]
			}
		}
	}
	// Generate the file name
	fileName := fmt.Sprintf("%x%s", hash.Sum(nil), fileExtension)
	// Save file
	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		return "", fmt.Errorf("%w: failed to read multipart file", micropub.ErrBadRequest)
	}
	location, err := s.a.saveMediaFile(fileName, file)
	if err != nil {
		return "", fmt.Errorf("%w: failed to save original file", micropub.ErrBadRequest)
	}
	// Try to compress file (only when not in private mode)
	if !s.a.isPrivate() {
		compressedLocation, compressionErr := s.a.compressMediaFile(location)
		if compressionErr != nil {
			return "", fmt.Errorf("%w: failed to compress file: %w", micropub.ErrBadRequest, compressionErr)
		}
		// Overwrite location
		if compressedLocation != "" {
			location = compressedLocation
		}
	}
	return location, nil
}

func (s *micropubImplementation) mapToParameterName(key string) string {
	switch key {
	case "name":
		return "title"
	case "category":
		return s.a.cfg.Micropub.CategoryParam
	case "in-reply-to":
		return s.a.cfg.Micropub.ReplyParam
	case "like-of":
		return s.a.cfg.Micropub.LikeParam
	case "bookmark-of":
		return s.a.cfg.Micropub.BookmarkParam
	case "audio":
		return s.a.cfg.Micropub.AudioParam
	case "location":
		return s.a.cfg.Micropub.LocationParam
	default:
		return key
	}
}

func (a *goBlog) processContentAndParameters(p *post) error {
	// Ensure parameters map is initialized
	if p.Parameters == nil {
		p.Parameters = map[string][]string{}
	}

	// Normalize line endings in content
	p.Content = regexp.MustCompile("\r\n").ReplaceAllString(p.Content, "\n")

	// Check for frontmatter
	err := extractFrontmatter(p)
	if err != nil {
		return err
	}

	// Extract specific parameters
	extractParam := func(paramName string, field any) {
		if values, ok := p.Parameters[paramName]; len(values) == 1 && ok {
			if stringPointer, ok := field.(*string); ok {
				*stringPointer = values[0]
			} else if stringFunc, ok := field.(func(string)); ok {
				stringFunc(values[0])
			}
			delete(p.Parameters, paramName)
		}
	}

	extractParam("blog", &p.Blog)
	extractParam("path", &p.Path)
	extractParam("section", &p.Section)
	extractParam("slug", &p.Slug)
	extractParam("published", &p.Published)
	extractParam("updated", &p.Updated)
	extractParam("status", func(status string) { p.Status = postStatus(status) })
	extractParam("visibility", func(visibility string) { p.Visibility = postVisibility(visibility) })
	extractParam("priority", func(priority string) { p.Priority = cast.ToInt(priority) })

	// Add images not in content
	images, imageAlts := p.Parameters[a.cfg.Micropub.PhotoParam], p.Parameters[a.cfg.Micropub.PhotoDescriptionParam]
	useAlts := len(images) == len(imageAlts)
	for i, image := range images {
		if !strings.Contains(p.Content, image) {
			if useAlts && imageAlts[i] != "" {
				p.Content += fmt.Sprintf("\n\n![%s](%s \"%s\")", imageAlts[i], image, imageAlts[i])
			} else {
				p.Content += fmt.Sprintf("\n\n![](%s)", image)
			}
		}
	}

	return nil
}

func extractFrontmatter(p *post) error {
	lines := strings.Split(p.Content, "\n")
	if len(lines) > 2 {
		// Check if the first line contains a repeated character
		firstLine := strings.TrimSpace(lines[0])
		if len(firstLine) >= 3 && strings.Count(firstLine, string(firstLine[0])) == len(firstLine) {
			separator := firstLine
			// Find the next occurrence of the separator
			for i := 1; i < len(lines); i++ {
				if strings.TrimSpace(lines[i]) == separator {
					// Extract frontmatter
					fm := strings.Join(lines[1:i], "\n")
					meta := map[string]any{}
					if err := yaml.Unmarshal([]byte(fm), &meta); err != nil {
						return err
					}
					// Copy frontmatter to parameters
					for key, value := range meta {
						// For parameters starting with "+", use existing parameters and just append
						// For other parameters, create a new slice
						if !strings.HasPrefix(key, "+") {
							p.Parameters[key] = []string{}
						} else {
							key = strings.TrimPrefix(key, "+")
						}
						// Append to existing parameters
						if a, ok := value.([]any); ok {
							for _, ae := range a {
								p.Parameters[key] = append(p.Parameters[key], cast.ToString(ae))
							}
						} else {
							p.Parameters[key] = append(p.Parameters[key], cast.ToString(value))
						}
					}
					// Remove frontmatter from content
					p.Content = strings.Join(lines[i+1:], "\n")
					break
				}
			}
		}
	}
	return nil
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

func micropubUpdateMfProperties(properties map[string][]any, req micropub.RequestUpdate) (map[string][]any, error) {
	if req.Replace != nil {
		delete(req.Replace, "goblog-editor")
		maps.Copy(properties, req.Replace)
	}

	if req.Add != nil {
		for key, value := range req.Add {
			if _, ok := properties[key]; !ok {
				properties[key] = []any{}
			}
			properties[key] = append(properties[key], value...)
		}
	}

	if req.Delete != nil {
		if reflect.TypeOf(req.Delete).Kind() == reflect.Slice {
			toDelete, ok := req.Delete.([]any)
			if !ok {
				return nil, errors.New("invalid delete array")
			}
			for _, key := range toDelete {
				delete(properties, cast.ToString(key))
			}
		} else {
			toDelete, ok := req.Delete.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("invalid delete object: expected map[string]any, got: %s", reflect.TypeOf(req.Delete))
			}
			for key, v := range toDelete {
				value, ok := v.([]any)
				if !ok {
					// Wrong type, ignore
					continue
				}
				if _, ok := properties[key]; !ok {
					// Parameter not present, ignore delete
					continue
				}
				properties[key] = lo.Filter(properties[key], func(ss any, _ int) bool {
					return !slices.Contains(value, ss)
				})
			}
		}
	}
	return properties, nil
}

func (s *micropubImplementation) updatePostPropertiesFromMf(p *post, properties map[string][]any) {
	if properties == nil || p == nil {
		return
	}

	// Ignore the following properties
	delete(properties, "url")
	delete(properties, "photo")
	delete(properties, "photo-alt")

	// Helper function
	getFirstStringFromArray := func(arr any) string {
		if strArr, ok := arr.([]any); ok && len(strArr) > 0 {
			if str, ok := strArr[0].(string); ok {
				return str
			}
		}
		return ""
	}

	// Set other properties
	p.Content = getFirstStringFromArray(properties["content"])
	delete(properties, "content")
	p.Published = getFirstStringFromArray(properties["published"])
	delete(properties, "published")
	p.Updated = getFirstStringFromArray(properties["updated"])
	delete(properties, "updated")
	p.Slug = getFirstStringFromArray(properties["mp-slug"])
	delete(properties, "mp-slug")
	p.setChannel(getFirstStringFromArray(properties["mp-channel"]))
	delete(properties, "mp-channel")
	p.Visibility = postVisibility(cmp.Or(getFirstStringFromArray(properties["visibility"]), string(p.Visibility)))
	delete(properties, "visibility")
	if newStatusString := getFirstStringFromArray(properties["post-status"]); newStatusString != "" {
		if newStatus := postStatus(newStatusString); newStatus == statusPublished || newStatus == statusDraft {
			p.Status = newStatus
		}
	}
	delete(properties, "post-status")

	for key, value := range properties {
		p.Parameters[s.mapToParameterName(key)] = cast.ToStringSlice(value)
	}

}
