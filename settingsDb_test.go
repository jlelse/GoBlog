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

func Test_settingsDb_blogTitleDescription(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	_ = app.initConfig(false)

	blog := app.cfg.DefaultBlog

	// After initConfig, the default config values are migrated to database
	// So we expect the default "My Blog" and "Welcome to my blog." values
	title, err := app.getBlogTitle(blog)
	require.NoError(t, err)
	require.Equal(t, "My Blog", title)

	description, err := app.getBlogDescription(blog)
	require.NoError(t, err)
	require.Equal(t, "Welcome to my blog.", description)

	// Update title and description
	err = app.setBlogTitle(blog, "Test Blog Title")
	require.NoError(t, err)

	err = app.setBlogDescription(blog, "Test Blog Description")
	require.NoError(t, err)

	// Read back
	title, err = app.getBlogTitle(blog)
	require.NoError(t, err)
	require.Equal(t, "Test Blog Title", title)

	description, err = app.getBlogDescription(blog)
	require.NoError(t, err)
	require.Equal(t, "Test Blog Description", description)

	// Update again
	err = app.setBlogTitle(blog, "Updated Title")
	require.NoError(t, err)

	err = app.setBlogDescription(blog, "Updated Description")
	require.NoError(t, err)

	// Read back updated values
	title, err = app.getBlogTitle(blog)
	require.NoError(t, err)
	require.Equal(t, "Updated Title", title)

	description, err = app.getBlogDescription(blog)
	require.NoError(t, err)
	require.Equal(t, "Updated Description", description)
}

func Test_hasDeprecatedBlogTitleDescriptionConfig(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}

	// Before initConfig, there's no configTitle/configDescription set
	require.False(t, app.hasDeprecatedBlogTitleDescriptionConfig())

	// Now run initConfig which will set configTitle/configDescription from default blog config
	_ = app.initConfig(false)

	// After initConfig, configTitle and configDescription should be set from the default blog config
	// Default blog has Title: "My Blog" and Description: "Welcome to my blog."
	require.True(t, app.hasDeprecatedBlogTitleDescriptionConfig())

	// Simulate removing deprecated config by clearing configTitle and configDescription
	blog := app.cfg.DefaultBlog
	app.cfg.Blogs[blog].configTitle = ""
	app.cfg.Blogs[blog].configDescription = ""
	require.False(t, app.hasDeprecatedBlogTitleDescriptionConfig())

	// Set only title
	app.cfg.Blogs[blog].configTitle = "Old Title"
	require.True(t, app.hasDeprecatedBlogTitleDescriptionConfig())

	// Reset title, set description
	app.cfg.Blogs[blog].configTitle = ""
	app.cfg.Blogs[blog].configDescription = "Old Description"
	require.True(t, app.hasDeprecatedBlogTitleDescriptionConfig())

	// Reset both
	app.cfg.Blogs[blog].configDescription = ""
	require.False(t, app.hasDeprecatedBlogTitleDescriptionConfig())
}
