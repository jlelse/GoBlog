package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_postsScheduler(t *testing.T) {

	updateHook, postHook := 0, 0

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
	app.pPostHooks = append(app.pPostHooks, func(p *post) {
		postHook++
	})
	app.pUpdateHooks = append(app.pUpdateHooks, func(p *post) {
		updateHook++
	})

	_ = app.initConfig(false)
	_ = app.initCache()

	err := app.db.savePost(&post{
		Path:       "/test/abc",
		Content:    "ABC",
		Published:  toLocalSafe(time.Now().Add(-1 * time.Hour).String()),
		Blog:       "en",
		Section:    "test",
		Status:     statusScheduled,
		Visibility: visibilityPublic,
	}, &postCreationOptions{new: true})
	require.NoError(t, err)

	count, err := app.db.countPosts(&postsRequestConfig{status: []postStatus{statusScheduled}})
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	assert.Equal(t, 0, postHook)
	assert.Equal(t, 0, updateHook)

	app.checkScheduledPosts()

	count, err = app.db.countPosts(&postsRequestConfig{status: []postStatus{statusScheduled}})
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	time.Sleep(time.Second)

	assert.Equal(t, 1, postHook)
	assert.Equal(t, 0, updateHook)

}
