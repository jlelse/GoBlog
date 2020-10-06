package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/araddon/dateparse"
)

func (p *Post) checkPost(new bool) error {
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
	if p.Published == "" {
		p.Published = now.String()
	}
	if p.Updated != "" {
		d, err := dateparse.ParseIn(p.Updated, time.Local)
		if err != nil {
			return err
		}
		p.Updated = d.String()
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
			random := generateRandomString(now, 5)
			p.Slug = fmt.Sprintf("%v-%02d-%02d-%v", now.Year(), int(now.Month()), now.Day(), random)
		}
		published, _ := dateparse.ParseIn(p.Published, time.Local)
		pathVars := struct {
			BlogPath string
			Year     int
			Month    int
			Day      int
			Slug     string
			Section  string
		}{
			BlogPath: appConfig.Blogs[p.Blog].Path,
			Year:     published.Year(),
			Month:    int(published.Month()),
			Day:      published.Day(),
			Slug:     p.Slug,
			Section:  p.Section,
		}
		pathTmplString := appConfig.Blogs[p.Blog].Sections[p.Section].PathTemplate
		if pathTmplString == "" {
			return errors.New("path template empty")
		}
		pathTmpl, err := template.New("location").Parse(pathTmplString)
		if err != nil {
			return errors.New("failed to parse location template")
		}
		var pathBuffer bytes.Buffer
		err = pathTmpl.Execute(&pathBuffer, pathVars)
		if err != nil {
			return errors.New("failed to execute location template")
		}
		p.Path = pathBuffer.String()
	}
	if p.Path != "" && !strings.HasPrefix(p.Path, "/") {
		return errors.New("wrong path")
	}
	// Check if post with path already exists
	if new {
		post, _ := getPost(context.Background(), p.Path)
		if post != nil {
			return errors.New("path already exists")
		}
	}
	return nil
}

func (p *Post) createOrReplace(new bool) error {
	err := p.checkPost(new)
	if err != nil {
		return err
	}
	startWritingToDb()
	tx, err := appDb.Begin()
	if err != nil {
		finishWritingToDb()
		return err
	}
	_, err = tx.Exec("insert or replace into posts (path, content, published, updated, blog, section) values (?, ?, ?, ?, ?, ?)", p.Path, p.Content, p.Published, p.Updated, p.Blog, p.Section)
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
	return reloadRouter()
}

func (p *Post) delete() error {
	// TODO
	err := p.checkPost(false)
	if err != nil {
		return err
	}
	startWritingToDb()
	tx, err := appDb.Begin()
	if err != nil {
		finishWritingToDb()
		return err
	}
	_, err = tx.Exec("delete from posts where path=?", p.Path)
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
	err = tx.Commit()
	if err != nil {
		finishWritingToDb()
		return err
	}
	finishWritingToDb()
	go purgeCache()
	return reloadRouter()
}
