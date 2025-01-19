package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

/*
Imports markdown files to GoBlog (good for HUGO websites)
1. Iterates ONLY over 1st level folders, each folder created as section
2. pages in top level are pages the empty section

Skip if errors and inform of error..
Will most probably give give error because of Markdown title
Ugly workaround: for i in {1..300}; do ./GoBlog -config config.yml import /path/to/content; done
PROBLEM: maybe catch the error/panic and continue fails
KISS principle = Keep It Super Simple
*/
func (a *goBlog) importMarkdownFiles(importDir string) error {
	fmt.Println(" WARNING IF YOU GET AN ERROR, UGLY WORKAROUND try: for i in {1..300}; do ./GoBlog -config config.yml import /path/to/content; done")
	iterateOverFiles := func(dir string, section string) error {
		files, err := os.ReadDir(dir)
		if err != nil {
			fmt.Println("Error reading directory:", err)
			return err
		}

		for _, f := range files {
			if !f.IsDir() { // Check if it's a file
				filePath := filepath.Join(dir, f.Name())
				data, err := os.ReadFile(filePath)
				if err != nil {
					fmt.Println("Error reading file:", err)
					continue
				}
				if !slices.Contains([]string{".md", ".adoc"}, path.Ext(f.Name())) {
					continue
				}
				if strings.Contains(f.Name(), "_index") {
					continue
				}

				newSection := &configSection{
					Name:        section,
					Title:       section,
					Description: section,
					// PathTemplate: sectionPathTemplate,
					// ShowFull:     sectionShowFull,
					// HideOnStart:  sectionHideOnStart,
				}
				if section != "" {
					err = a.saveSection("en", newSection)
					if err != nil {
						fmt.Printf(" Error creating post for file %s:  ERROR: %s\n", f.Name(), err)
					}
				}

				p := &post{Content: string(data), Path: path.Join("/", section, strings.TrimSuffix(f.Name(), filepath.Ext(f.Name()))), Section: section}

				err = a.extractParamsFromContent(p)

				if err != nil {
					fmt.Printf(" Error processing %s: %s", f.Name(), err)
				}

				//delete adoc variables... and misbehaving variables
				for key, value := range p.Parameters {
					if strings.HasPrefix(key, ":") {

						// fmt.Println("Removing param ", key)
						delete(p.Parameters, key)
					}

					if !slices.Contains([]string{"path", "blog", "section", "slug", "published", "updated", "status", "visibility", "priority", "title", "tags"}, key) {
						fmt.Println("Removing param ", key)
						if key == "date" {
							// p.Parameters["published"] = value
							p.Published = value[0]
						}

						delete(p.Parameters, key)
					}
				}
				//TODO fails sometimes due to upstream processing
				//Can be removed/hidden but then you need to MANUALLY
				p.RenderedTitle = p.Title()
				// filepath.Join()
				err = a.createPost(p)
				if err != nil {
					fmt.Printf(" Error creating post for file %s:  ERROR: %s\n", f.Name(), err)
				}
			}
		}
		return nil
	}
	err := filepath.Walk(importDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() && path != importDir { // Check if it's a subdirectory and not the 'import' directory itself
			section := filepath.Base(path)
			// fmt.Printf("Section: %s\n", section)
			iterateOverFiles(path, section)
			return nil // Skip subdirectories for file iteration
		}

		return nil
	})
	if err != nil {
		fmt.Println("Error walking through directories:", err)
		return err
	}

	// Iterate over files in the 'import' directory
	iterateOverFiles(importDir, "")
	return nil

}
