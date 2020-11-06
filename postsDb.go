package main

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/araddon/dateparse"
)

func (p *post) checkPost() error {
	if p == nil {
		return errors.New("no post")
	}
	now := time.Now()
	// Fix content
	p.Content = strings.TrimSuffix(strings.TrimPrefix(p.Content, "\n"), "\n")
	// Fix date strings
	if p.Published != "" {
		d, err := dateparse.ParseIn(p.Published, time.Local)
		if err != nil {
			return err
		}
		p.Published = d.String()
	}
	if p.Updated != "" {
		d, err := dateparse.ParseIn(p.Updated, time.Local)
		if err != nil {
			return err
		}
		p.Updated = d.String()
	}
	// Cleanup params
	for key, value := range p.Parameters {
		if value == nil {
			delete(p.Parameters, key)
			continue
		}
		allValues := []string{}
		for _, v := range value {
			if v != "" {
				allValues = append(allValues, v)
			}
		}
		if len(allValues) >= 1 {
			p.Parameters[key] = allValues
		} else {
			delete(p.Parameters, key)
		}
	}
	// Check blog
	if p.Blog == "" {
		p.Blog = appConfig.DefaultBlog
	}
	// Check path
	p.Path = strings.TrimSuffix(p.Path, "/")
	if p.Path == "" {
		if p.Section == "" {
			p.Section = appConfig.Blogs[p.Blog].DefaultSection
		}
		if p.Slug == "" {
			random := generateRandomString(5)
			p.Slug = fmt.Sprintf("%v-%02d-%02d-%v", now.Year(), int(now.Month()), now.Day(), random)
		}
		published, _ := dateparse.ParseIn(p.Published, time.Local)
		pathTmplString := appConfig.Blogs[p.Blog].Sections[p.Section].PathTemplate
		if pathTmplString == "" {
			return errors.New("path template empty")
		}
		pathTmpl, err := template.New("location").Parse(pathTmplString)
		if err != nil {
			return errors.New("failed to parse location template")
		}
		var pathBuffer bytes.Buffer
		err = pathTmpl.Execute(&pathBuffer, map[string]interface{}{
			"BlogPath": appConfig.Blogs[p.Blog].Path,
			"Year":     published.Year(),
			"Month":    int(published.Month()),
			"Day":      published.Day(),
			"Slug":     p.Slug,
			"Section":  p.Section,
		})
		if err != nil {
			return errors.New("failed to execute location template")
		}
		p.Path = pathBuffer.String()
	}
	if p.Path != "" && !strings.HasPrefix(p.Path, "/") {
		return errors.New("wrong path")
	}
	return nil
}

func (p *post) create() error {
	return p.createOrReplace(true)
}

func (p *post) replace() error {
	return p.createOrReplace(false)
}

func (p *post) createOrReplace(new bool) error {
	err := p.checkPost()
	if err != nil {
		return err
	}
	startWritingToDb()
	tx, err := appDb.Begin()
	if err != nil {
		finishWritingToDb()
		return err
	}
	sqlCommand := "insert"
	if !new {
		sqlCommand = "insert or replace"
	}
	_, err = tx.Exec(sqlCommand+" into posts (path, content, published, updated, blog, section) values (?, ?, ?, ?, ?, ?)", p.Path, p.Content, p.Published, p.Updated, p.Blog, p.Section)
	if err != nil {
		_ = tx.Rollback()
		finishWritingToDb()
		return err
	}
	_, err = tx.Exec("delete from post_parameters where path=?", p.Path)
	if err != nil {
		_ = tx.Rollback()
		finishWritingToDb()
		return err
	}
	for param, value := range p.Parameters {
		for _, value := range value {
			if value != "" {
				_, err = tx.Exec("insert into post_parameters (path, parameter, value) values (?, ?, ?)", p.Path, param, value)
				if err != nil {
					_ = tx.Rollback()
					finishWritingToDb()
					return err
				}
			}
		}
	}
	err = tx.Commit()
	if err != nil {
		finishWritingToDb()
		return err
	}
	finishWritingToDb()
	go purgeCache()
	defer func(p *post) {
		postPostHooks(p.Path)
		go apPost(p)
	}(p)
	return reloadRouter()
}

func deletePost(path string) error {
	if path == "" {
		return nil
	}
	startWritingToDb()
	tx, err := appDb.Begin()
	if err != nil {
		finishWritingToDb()
		return err
	}
	_, err = tx.Exec("delete from posts where path=?", path)
	if err != nil {
		_ = tx.Rollback()
		finishWritingToDb()
		return err
	}
	_, err = tx.Exec("delete from post_parameters where path=?", path)
	if err != nil {
		_ = tx.Rollback()
		finishWritingToDb()
		return err
	}
	err = tx.Commit()
	if err != nil {
		finishWritingToDb()
		return err
	}
	finishWritingToDb()
	go purgeCache()
	defer postDeleteHooks(path)
	return reloadRouter()
}
