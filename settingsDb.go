package main

import (
	"database/sql"
	"errors"
)

func (a *goBlog) getSettingValue(name string) (string, error) {
	row, err := a.db.queryRow("select value from settings where name = @name", sql.Named("name", name))
	if err != nil {
		return "",
			err
	}
	var value string
	err = row.Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	} else if err != nil {
		return "", err
	}
	return value, nil
}

func (a *goBlog) saveSettingValue(name, value string) error {
	_, err := a.db.exec(
		"insert into settings (name, value) values (@name, @value) on conflict (name) do update set value = @value2",
		sql.Named("name", name),
		sql.Named("value", value),
		sql.Named("value2", value),
	)
	return err
}

func (a *goBlog) loadSections() error {
	for blog, bc := range a.cfg.Blogs {
		sections, err := a.getSections(blog)
		if err != nil {
			return err
		}
		bc.Sections = sections
	}
	return nil
}

func (a *goBlog) getSections(blog string) (map[string]*configSection, error) {
	rows, err := a.db.query("select name, title, description, pathtemplate, showfull from sections where blog = @blog", sql.Named("blog", blog))
	if err != nil {
		return nil, err
	}
	sections := map[string]*configSection{}
	for rows.Next() {
		section := &configSection{}
		err = rows.Scan(&section.Name, &section.Title, &section.Description, &section.PathTemplate, &section.ShowFull)
		if err != nil {
			return nil, err
		}
		sections[section.Name] = section
	}
	return sections, nil
}

func (a *goBlog) saveAllSections() error {
	for blog, bc := range a.cfg.Blogs {
		for k, s := range bc.Sections {
			s.Name = k
			if err := a.addSection(blog, s); err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *goBlog) addSection(blog string, section *configSection) error {
	_, err := a.db.exec(
		"insert into sections (blog, name, title, description, pathtemplate, showfull) values (@blog, @name, @title, @description, @pathtemplate, @showfull)",
		sql.Named("blog", blog),
		sql.Named("name", section.Name),
		sql.Named("title", section.Title),
		sql.Named("description", section.Description),
		sql.Named("pathtemplate", section.PathTemplate),
		sql.Named("showfull", section.ShowFull),
	)
	return err
}

func (a *goBlog) deleteSection(blog string, name string) error {
	_, err := a.db.exec("delete from sections where blog = @blog and name = @name", sql.Named("blog", blog), sql.Named("name", name))
	return err
}

func (a *goBlog) updateSection(blog string, name string, section *configSection) error {
	_, err := a.db.exec(
		"update sections set title = @title, description = @description, pathtemplate = @pathtemplate, showfull = @showfull where blog = @blog and name = @name",
		sql.Named("title", section.Title),
		sql.Named("description", section.Description),
		sql.Named("pathtemplate", section.PathTemplate),
		sql.Named("showfull", section.ShowFull),
		sql.Named("blog", blog),
		sql.Named("name", section.Name),
	)
	return err
}
