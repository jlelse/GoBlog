package main

import (
	"errors"
	"github.com/araddon/dateparse"
	"strings"
	"time"
)

func (p *Post) checkPost() error {
	if p == nil {
		return errors.New("no post")
	}
	if p.Path == "" || !strings.HasPrefix(p.Path, "/") {
		return errors.New("wrong path")
	}
	// Fix path
	p.Path = strings.TrimSuffix(p.Path, "/")
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
	return nil
}

func (p *Post) createOrReplace() error {
	err := p.checkPost()
	if err != nil {
		return err
	}
	startWritingToDb()
	tx, err := appDb.Begin()
	if err != nil {
		return err
	}
	_, err = tx.Exec("insert or replace into posts (path, content, published, updated) values (?, ?, ?, ?)", p.Path, p.Content, p.Published, p.Updated)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	_, err = tx.Exec("delete from post_parameters where path=?", p.Path)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	for param, value := range p.Parameters {
		for _, value := range value {
			if value != "" {
				_, err = tx.Exec("insert into post_parameters (path, parameter, value) values (?, ?, ?)", p.Path, param, value)
				if err != nil {
					_ = tx.Rollback()
					return err
				}
			}
		}
	}
	err = tx.Commit()
	if err != nil {
		return err
	}
	finishWritingToDb()
	go purgeCache(p.Path)
	return reloadRouter()
}

func (p *Post) delete() error {
	err := p.checkPost()
	if err != nil {
		return err
	}
	startWritingToDb()
	tx, err := appDb.Begin()
	if err != nil {
		return err
	}
	_, err = tx.Exec("delete from posts where path=?", p.Path)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	_, err = tx.Exec("delete from post_parameters where path=?", p.Path)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	err = tx.Commit()
	if err != nil {
		return err
	}
	finishWritingToDb()
	go purgeCache(p.Path)
	return reloadRouter()
}
