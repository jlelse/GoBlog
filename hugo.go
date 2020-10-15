package main

import (
	"strconv"
	"strings"

	"github.com/jeremywohl/flatten"
	"github.com/spf13/cast"
	"gopkg.in/yaml.v3"
)

func parseHugoFile(fileContent string) (p *post, aliases []string, e error) {
	frontmatterSep := "---\n"
	frontmatter := ""
	if split := strings.Split(fileContent, frontmatterSep); len(split) > 2 {
		frontmatter = split[1]
	}
	p = &post{
		Content:    strings.TrimPrefix(fileContent, frontmatterSep+frontmatter+frontmatterSep),
		Parameters: map[string][]string{},
	}
	// Parse frontmatter
	meta := map[string]interface{}{}
	err := yaml.Unmarshal([]byte(frontmatter), &meta)
	if err != nil {
		return nil, nil, err
	}
	flat, err := flatten.Flatten(meta, "", flatten.DotStyle)
	if err != nil {
		return nil, nil, err
	}
	// Read dates
	p.Published = cast.ToString(flat["date"])
	p.Updated = cast.ToString(flat["lastmod"])
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
			p.Parameters[fm.Parameter] = values
		}
	}
	// Parse redirects
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
	// Return post
	return p, aliases, nil
}
