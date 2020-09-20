package main

import (
	"errors"
	"github.com/jeremywohl/flatten"
	"github.com/spf13/cast"
	"gopkg.in/yaml.v3"
	"strconv"
	"strings"
)

func parseHugoFile(fileContent string, path string) (*Post, error) {
	if path == "" {
		return nil, errors.New("empty path")
	}
	frontmatterSep := "---\n"
	frontmatter := ""
	if split := strings.Split(fileContent, frontmatterSep); len(split) > 2 {
		frontmatter = split[1]
	}
	post := &Post{
		Path:       path,
		Content:    strings.TrimPrefix(fileContent, frontmatterSep+frontmatter+frontmatterSep),
		Parameters: map[string][]string{},
	}
	// Parse frontmatter
	meta := map[string]interface{}{}
	err := yaml.Unmarshal([]byte(frontmatter), &meta)
	if err != nil {
		return nil, err
	}
	flat, err := flatten.Flatten(meta, "", flatten.DotStyle)
	if err != nil {
		return nil, err
	}
	// Read dates
	post.Published = cast.ToString(flat["date"])
	post.Updated = cast.ToString(flat["lastmod"])
	// Read parameters
	for _, fm := range appConfig.Hugo.Frontmatter {
		var values []string
		for fk, value := range flat {
			if strings.HasPrefix(fk, fm.Meta) {
				trimmed := strings.TrimPrefix(fk, fm.Meta)
				if len(trimmed) == 0 {
					values = append(values, cast.ToString(value))
				} else {
					trimmed = strings.TrimPrefix(trimmed, ".")
					if _, e := strconv.Atoi(trimmed); e == nil {
						values = append(values, cast.ToString(value))
					}
				}
			}
		}
		if len(values) > 0 {
			post.Parameters[fm.Parameter] = values
		}
	}
	// Create redirects
	var aliases []string
	for fk, value := range flat {
		if strings.HasPrefix(fk, "aliases") {
			trimmed := strings.TrimPrefix(fk, "aliases")
			if len(trimmed) == 0 {
				aliases = append(aliases, cast.ToString(value))
			} else {
				trimmed = strings.TrimPrefix(trimmed, ".")
				if _, e := strconv.Atoi(trimmed); e == nil {
					aliases = append(aliases, cast.ToString(value))
				}
			}
		}
	}
	for _, alias := range aliases {
		// Fix relativ paths
		if !strings.HasPrefix(alias, "/") {
			splittedPostPath := strings.Split(post.Path, "/")
			alias = strings.TrimSuffix(post.Path, splittedPostPath[len(splittedPostPath)-1]) + alias
		}
		_ = createOrReplaceRedirect(alias, post.Path)
	}
	// Return post
	return post, nil
}
