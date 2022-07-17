package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_postsScheduler(t *testing.T) {

	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Blogs = map[string]*configBlog{
		"en": {
			Sections: map[string]*configSection{
				"test": {},
			},
			Lang: "en",
		},
	}

	_ = app.initConfig(false)
	_ = app.initCache()

	err := app.db.savePost(&post{
		Path:      "/test/abc",
		Content:   "ABC",
		Published: toLocalSafe(time.Now().Add(-1 * time.Hour).String()),
		Blog:      "en",
		Section:   "test",
		Status:    statusScheduled,
	}, &postCreationOptions{new: true})
	require.NoError(t, err)

	count, err := app.db.countPosts(&postsRequestConfig{status: statusPublished})
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	app.checkScheduledPosts()

	count, err = app.db.countPosts(&postsRequestConfig{status: statusPublished})
	require.NoError(t, err)
	assert.Equal(t, 1, count)

}
