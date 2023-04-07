package main

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/samber/lo"
)

func settingNameWithBlog(blog, name string) string {
	return fmt.Sprintf("%s---%s", blog, name)
}

const (
	defaultSectionSetting        = "defaultsection"
	hideOldContentWarningSetting = "hideoldcontentwarning"
	hideShareButtonSetting       = "hidesharebutton"
	hideTranslateButtonSetting   = "hidetranslatebutton"
	userNickSetting              = "usernick"
	userNameSetting              = "username"
	addReplyTitleSetting         = "addreplytitle"
	addReplyContextSetting       = "addreplycontext"
	addLikeTitleSetting          = "addliketitle"
	addLikeContextSetting        = "addlikecontext"
)

func (a *goBlog) getSettingValue(name string) (string, error) {
	row, err := a.db.QueryRow("select value from settings where name = @name", sql.Named("name", name))
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

func (a *goBlog) getBooleanSettingValue(name string, defaultValue bool) (bool, error) {
	stringValue, err := a.getSettingValue(name)
	if err != nil {
		return defaultValue, err
	}
	if stringValue == "" {
		return defaultValue, nil
	}
	return stringValue == "1", nil
}

func (a *goBlog) saveSettingValue(name, value string) error {
	_, err := a.db.Exec(
		"insert into settings (name, value) values (@name, @value) on conflict (name) do update set value = @value2",
		sql.Named("name", name),
		sql.Named("value", value),
		sql.Named("value2", value),
	)
	return err
}

func (a *goBlog) saveBooleanSettingValue(name string, value bool) error {
	return a.saveSettingValue(name, lo.If(value, "1").Else("0"))
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
	rows, err := a.db.Query("select name, title, description, pathtemplate, showfull, hideonstart from sections where blog = @blog", sql.Named("blog", blog))
	if err != nil {
		return nil, err
	}
	sections := map[string]*configSection{}
	for rows.Next() {
		section := &configSection{}
		err = rows.Scan(&section.Name, &section.Title, &section.Description, &section.PathTemplate, &section.ShowFull, &section.HideOnStart)
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
			if err := a.saveSection(blog, s); err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *goBlog) saveSection(blog string, section *configSection) error {
	_, err := a.db.Exec(
		`
		insert into sections (blog, name, title, description, pathtemplate, showfull, hideonstart) values (@blog, @name, @title, @description, @pathtemplate, @showfull, @hideonstart)
		on conflict (blog, name) do update set title = @title2, description = @description2, pathtemplate = @pathtemplate2, showfull = @showfull2, hideonstart = @hideonstart2
		`,
		sql.Named("blog", blog),
		sql.Named("name", section.Name),
		sql.Named("title", section.Title),
		sql.Named("description", section.Description),
		sql.Named("pathtemplate", section.PathTemplate),
		sql.Named("showfull", section.ShowFull),
		sql.Named("hideonstart", section.HideOnStart),
		sql.Named("title2", section.Title),
		sql.Named("description2", section.Description),
		sql.Named("pathtemplate2", section.PathTemplate),
		sql.Named("showfull2", section.ShowFull),
		sql.Named("hideonstart2", section.HideOnStart),
	)
	return err
}

func (a *goBlog) deleteSection(blog string, name string) error {
	_, err := a.db.Exec("delete from sections where blog = @blog and name = @name", sql.Named("blog", blog), sql.Named("name", name))
	return err
}
