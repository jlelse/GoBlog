package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func Test_checkDeletedPosts(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}

	_ = app.initConfig(false)
	_ = app.initCache()

	// Create a post
	err := app.createPost(&post{
		Content: "Test",
		Status:  statusPublished,
		Path:    "/testpost",
		Section: "posts",
	})
	require.NoError(t, err)

	// Check if post count is 1
	count, err := app.db.countPosts(&postsRequestConfig{})
	require.NoError(t, err)
	require.Equal(t, 1, count)

	// Run deleter
	app.checkDeletedPosts()

	// Check if post count is still 1
	count, err = app.db.countPosts(&postsRequestConfig{})
	require.NoError(t, err)
	require.Equal(t, 1, count)

	// Delete the post
	err = app.deletePost("/testpost")
	require.NoError(t, err)

	// Run deleter
	app.checkDeletedPosts()

	// Check if post count is still 1
	count, err = app.db.countPosts(&postsRequestConfig{})
	require.NoError(t, err)
	require.Equal(t, 1, count)

	// Set deleted time to more than 7 days ago
	err = app.db.replacePostParam("/testpost", "deleted", []string{time.Now().Add(-time.Hour * 24 * 8).Format(time.RFC3339)})
	require.NoError(t, err)

	// Run deleter
	app.checkDeletedPosts()

	// Check if post count is 0
	count, err = app.db.countPosts(&postsRequestConfig{})
	require.NoError(t, err)
	require.Equal(t, 0, count)
}
