package main

import (
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
)

func Test_settingsDb_sections(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	_ = app.initConfig(false)

	require.Len(t, lo.Values(app.cfg.Blogs), 1)

	sections, err := app.getSections(app.cfg.DefaultBlog)
	require.NoError(t, err)
	require.Len(t, lo.Values(sections), 1)

	// Update
	section := lo.Values(sections)[0]
	section.Title = "New Title"
	err = app.saveSection(app.cfg.DefaultBlog, section)
	require.NoError(t, err)

	// Check update
	sections, err = app.getSections(app.cfg.DefaultBlog)
	require.NoError(t, err)
	require.Len(t, lo.Values(sections), 1)
	section = lo.Values(sections)[0]
	require.Equal(t, "New Title", section.Title)

	// New section
	section = &configSection{
		Name:  "new",
		Title: "New section",
	}
	err = app.saveSection(app.cfg.DefaultBlog, section)
	require.NoError(t, err)

	// Check new section count
	sections, err = app.getSections(app.cfg.DefaultBlog)
	require.NoError(t, err)
	require.Len(t, lo.Values(sections), 2)

	// Delete section
	err = app.deleteSection(app.cfg.DefaultBlog, "new")
	require.NoError(t, err)
	sections, err = app.getSections(app.cfg.DefaultBlog)
	require.NoError(t, err)
	require.Len(t, lo.Values(sections), 1)

}
