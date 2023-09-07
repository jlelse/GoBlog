package main

import (
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_postsDb(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Blogs = map[string]*configBlog{
		"en": {
			Sections: map[string]*configSection{
				"test":  {},
				"micro": {},
			},
		},
	}

	_ = app.initConfig(false)
	app.initMarkdown()
	_ = app.initCache()

	now := toLocalSafe(time.Now().String())
	nowPlus1Hour := toLocalSafe(time.Now().Add(1 * time.Hour).String())

	// Save post
	err := app.db.savePost(&post{
		Path:       "/test/abc",
		Content:    "ABC",
		Published:  now,
		Updated:    nowPlus1Hour,
		Blog:       "en",
		Section:    "test",
		Status:     statusDraft,
		Visibility: visibilityPublic,
		Parameters: map[string][]string{
			"title": {"Title"},
			"tags":  {"C", "A", "B"},
			"empty": {},
		},
	}, &postCreationOptions{new: true})
	must.NoError(err)

	// Check post
	p, err := app.getPost("/test/abc")
	must.NoError(err)
	is.Equal("/test/abc", p.Path)
	is.Equal("ABC", p.Content)
	is.Equal(now, p.Published)
	is.Equal(nowPlus1Hour, p.Updated)
	is.Equal("en", p.Blog)
	is.Equal("test", p.Section)
	is.Equal(statusDraft, p.Status)
	is.Equal("Title", p.Title())
	is.Equal([]string{"C", "A", "B"}, p.Parameters["tags"])

	// Check drafts
	drafts, _ := app.getPosts(&postsRequestConfig{
		blog:   "en",
		status: []postStatus{statusDraft},
	})
	is.Len(drafts, 1)

	// Check by parameter
	count, err := app.db.countPosts(&postsRequestConfig{parameter: "tags"})
	must.NoError(err)
	is.Equal(1, count)
	count, err = app.db.countPosts(&postsRequestConfig{parameter: "empty"})
	must.NoError(err)
	is.Equal(0, count)

	// Check by multiple parameters
	count, err = app.db.countPosts(&postsRequestConfig{anyParams: []string{"tags", "title"}})
	must.NoError(err)
	is.Equal(1, count)

	// Check by taxonomy
	count, err = app.db.countPosts(&postsRequestConfig{taxonomy: &configTaxonomy{Name: "tags"}})
	must.NoError(err)
	is.Equal(1, count)

	// Check by taxonomy value
	count, err = app.db.countPosts(&postsRequestConfig{taxonomy: &configTaxonomy{Name: "tags"}, taxonomyValue: "A"})
	must.NoError(err)
	is.Equal(1, count)

	// Delete post
	err = app.deletePost("/test/abc")
	must.NoError(err)

	// Check if post is marked as deleted
	count, err = app.db.countPosts(&postsRequestConfig{status: []postStatus{statusDraft}})
	must.NoError(err)
	is.Equal(0, count)
	count, err = app.db.countPosts(&postsRequestConfig{status: []postStatus{statusDraftDeleted}})
	must.NoError(err)
	is.Equal(1, count)

	// Delete post again
	err = app.deletePost("/test/abc")
	must.NoError(err)

	// Check that there is no post
	count, err = app.db.countPosts(&postsRequestConfig{})
	must.NoError(err)
	is.Equal(0, count)

	// Save published post
	err = app.db.savePost(&post{
		Path:       "/test/abc",
		Content:    "ABC",
		Published:  "2021-06-10 10:00:00",
		Updated:    "2021-06-15 10:00:00",
		Blog:       "en",
		Section:    "test",
		Status:     statusPublished,
		Visibility: visibilityPublic,
		Parameters: map[string][]string{
			"tags": {"Test", "Blog", "A"},
		},
	}, &postCreationOptions{new: true})
	must.NoError(err)

	// Check that there is a new post
	count, err = app.db.countPosts(&postsRequestConfig{})
	must.NoError(err)
	is.Equal(1, count)

	// Check based on offset
	count, err = app.db.countPosts(&postsRequestConfig{limit: 10, offset: 1})
	must.NoError(err)
	is.Equal(0, count)

	// Check random post path
	rp, err := app.getRandomPostPath("en")
	if is.NoError(err) {
		is.Equal("/test/abc", rp)
	}

	// Check taxonomies
	tags, err := app.db.allTaxonomyValues("en", "tags")
	if is.NoError(err) {
		is.Len(tags, 3)
		is.Equal([]string{"A", "Blog", "Test"}, tags)
	}

	// Check based on date
	count, err = app.db.countPosts(&postsRequestConfig{
		publishedYear: 2020,
	})
	if is.NoError(err) {
		is.Equal(0, count)
	}

	count, err = app.db.countPosts(&postsRequestConfig{
		publishedYear: 2021,
	})
	if is.NoError(err) {
		is.Equal(1, count)
	}

	count, err = app.db.countPosts(&postsRequestConfig{
		publishedMonth: 5,
	})
	if is.NoError(err) {
		is.Equal(0, count)
	}

	count, err = app.db.countPosts(&postsRequestConfig{
		publishedMonth: 6,
	})
	if is.NoError(err) {
		is.Equal(1, count)
	}

	count, err = app.db.countPosts(&postsRequestConfig{
		publishedDay: 15,
	})
	if is.NoError(err) {
		is.Equal(0, count)
	}

	count, err = app.db.countPosts(&postsRequestConfig{
		publishedDay: 10,
	})
	if is.NoError(err) {
		is.Equal(1, count)
	}

	// Check based on tags
	count, err = app.db.countPosts(&postsRequestConfig{
		parameter:      "tags",
		parameterValue: "ABC",
	})
	if is.NoError(err) {
		is.Equal(0, count)
	}

	count, err = app.db.countPosts(&postsRequestConfig{
		parameter:      "tags",
		parameterValue: "Blog",
	})
	if is.NoError(err) {
		is.Equal(1, count)
	}

	// Check that post is already present
	err = app.db.savePost(&post{
		Path:      "/test/abc",
		Content:   "ABCD",
		Published: "2021-06-10 10:00:00",
		Updated:   "2021-06-15 10:00:00",
		Blog:      "en",
		Section:   "test",
		Status:    statusPublished,
	}, &postCreationOptions{new: true})
	must.Error(err)
}

func Test_ftsWithoutTitle(t *testing.T) {
	// Added because there was a bug where there were no search results without title

	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	_ = app.initConfig(false)
	app.initMarkdown()

	err := app.db.savePost(&post{
		Path:      "/test/abc",
		Content:   "ABC",
		Published: toLocalSafe(time.Now().String()),
		Updated:   toLocalSafe(time.Now().Add(1 * time.Hour).String()),
		Blog:      "en",
		Section:   "test",
		Status:    statusDraft,
	}, &postCreationOptions{new: true})
	require.NoError(t, err)

	ps, err := app.getPosts(&postsRequestConfig{
		search: "ABC",
	})
	assert.NoError(t, err)
	assert.Len(t, ps, 1)
}

func Test_postsPriority(t *testing.T) {
	// Added because there was a bug where there were no search results without title

	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	_ = app.initConfig(false)
	app.initMarkdown()

	err := app.db.savePost(&post{
		Path:      "/test/abc",
		Content:   "ABC",
		Published: toLocalSafe(time.Now().String()),
		Blog:      "en",
		Section:   "test",
		Status:    statusPublished,
	}, &postCreationOptions{new: true})
	require.NoError(t, err)

	err = app.db.savePost(&post{
		Path:      "/test/def",
		Content:   "DEF",
		Published: toLocalSafe(time.Now().String()),
		Blog:      "en",
		Section:   "test",
		Status:    statusPublished,
		Priority:  1,
	}, &postCreationOptions{new: true})
	require.NoError(t, err)

	ps, err := app.getPosts(&postsRequestConfig{
		priorityOrder: true,
	})
	require.NoError(t, err)

	if assert.Len(t, ps, 2) {
		post1 := ps[0]

		assert.Equal(t, "/test/def", post1.Path)
		assert.Equal(t, 1, post1.Priority)

		post2 := ps[1]

		assert.Equal(t, "/test/abc", post2.Path)
		assert.Equal(t, 0, post2.Priority)
	}
}

func Test_usesOfMediaFile(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	_ = app.initConfig(false)

	err := app.db.savePost(&post{
		Path:      "/test/abc",
		Content:   "ABC test.jpg DEF",
		Published: toLocalSafe(time.Now().String()),
		Blog:      "en",
		Section:   "test",
		Status:    statusDraft,
	}, &postCreationOptions{new: true})
	require.NoError(t, err)

	err = app.db.savePost(&post{
		Path:      "/test/def",
		Content:   "ABCDEF",
		Published: toLocalSafe(time.Now().String()),
		Blog:      "en",
		Section:   "test",
		Status:    statusDraft,
		Parameters: map[string][]string{
			"test": {
				"https://example.com/test.jpg",
			},
		},
	}, &postCreationOptions{new: true})
	require.NoError(t, err)

	err = app.db.savePost(&post{
		Path:      "/test/hij",
		Content:   "ABCDEF",
		Published: toLocalSafe(time.Now().String()),
		Blog:      "en",
		Section:   "test",
		Status:    statusDraft,
	}, &postCreationOptions{new: true})
	require.NoError(t, err)

	counts, err := app.db.usesOfMediaFile("test.jpg")
	require.NoError(t, err)
	assert.Len(t, counts, 1)
	if assert.NotEmpty(t, counts) {
		assert.Equal(t, 2, counts[0])
	}
}

func Test_replaceParams(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	_ = app.initConfig(false)

	err := app.db.savePost(&post{
		Path:    "/test/abc",
		Content: "ABC",
		Parameters: map[string][]string{
			"test": {
				"ABC", "DEF", "GHI",
			},
		},
	}, &postCreationOptions{new: true})
	require.NoError(t, err)

	err = app.db.replacePostParam("/test/abc", "test", []string{"DEF", "123", "456"})
	require.NoError(t, err)

	p, err := app.getPost("/test/abc")
	require.NoError(t, err)

	if assert.NotNil(t, p) {
		assert.Len(t, p.Parameters["test"], 3)
		union := lo.Union(p.Parameters["test"], []string{"DEF", "123", "456"})
		assert.Len(t, union, 3)
	}
}

func Test_postDeletesParams(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}

	_ = app.initConfig(false)
	app.initMarkdown()
	_ = app.initCache()

	err := app.createPost(&post{
		Path:    "/test/abc",
		Content: "ABC",
		Parameters: map[string][]string{
			"test": {
				"ABC", "DEF", "GHI",
			},
		},
	})
	require.NoError(t, err)

	// Delete the first time (mark as delete)
	err = app.deletePost("/test/abc")
	require.NoError(t, err)

	row, err := app.db.QueryRow("select count(*) from post_parameters where path = ? and parameter = ?", "/test/abc", "test")
	require.NoError(t, err)

	var count int
	err = row.Scan(&count)
	require.NoError(t, err)

	assert.Equal(t, 3, count)

	// Delete the second time (actually delete)
	err = app.deletePost("/test/abc")
	require.NoError(t, err)

	row, err = app.db.QueryRow("select count(*) from post_parameters where path = ? and parameter = ?", "/test/abc", "test")
	require.NoError(t, err)

	err = row.Scan(&count)
	require.NoError(t, err)

	assert.Equal(t, 0, count)
}

func Test_checkPost(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	_ = app.initConfig(false)

	t.Run("New post should get published date", func(t *testing.T) {
		p := &post{}
		app.checkPost(p, true)

		assert.NotEmpty(t, p.Published)
	})

	t.Run("New post with path should get no published date", func(t *testing.T) {
		p := &post{
			Path: "/abc",
		}
		app.checkPost(p, true)

		assert.Empty(t, p.Published)
	})

	t.Run("Updated post with past published date should get updated date", func(t *testing.T) {
		p := &post{
			Published: time.Now().Local().Add(-1 * time.Hour).Format(time.RFC3339),
		}
		app.checkPost(p, false)

		assert.NotEmpty(t, p.Updated)
	})

	t.Run("Updated post with future published date should get no updated date", func(t *testing.T) {
		p := &post{
			Published: time.Now().Local().Add(time.Hour).Format(time.RFC3339),
		}
		app.checkPost(p, false)

		assert.Empty(t, p.Updated)
	})

	t.Run("Updated post with updated date should get new updated date", func(t *testing.T) {
		oldUpdate := time.Now().Local().Add(-1 * time.Hour).Format(time.RFC3339)
		p := &post{
			Published: time.Now().Local().Add(-2 * time.Hour).Format(time.RFC3339),
			Updated:   oldUpdate,
		}
		app.checkPost(p, false)

		assert.NotEmpty(t, p.Updated)
		assert.NotEqual(t, oldUpdate, p.Updated)
	})

	t.Run("Updated post with just updated date should get new updated date", func(t *testing.T) {
		oldUpdate := time.Now().Local().Add(-1 * time.Hour).Format(time.RFC3339)
		p := &post{
			Updated: oldUpdate,
		}
		app.checkPost(p, false)

		assert.Empty(t, p.Published)
		assert.NotEmpty(t, p.Updated)
		assert.NotEqual(t, oldUpdate, p.Updated)
	})

	t.Run("Invalid status should throw error", func(t *testing.T) {
		p := &post{
			Status: "unlisted",
		}
		err := app.checkPost(p, true)

		assert.ErrorContains(t, err, "invalid post status")
	})

	t.Run("Invalid visibility should throw error", func(t *testing.T) {
		p := &post{
			Visibility: "published",
		}
		err := app.checkPost(p, true)

		assert.ErrorContains(t, err, "invalid post visibility")
	})

}
