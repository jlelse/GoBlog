package main

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/carlmjohnson/requests"
	"github.com/spf13/cast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_commentsEditor(t *testing.T) {

	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Server.PublicAddress = "https://example.com"

	err := app.initConfig(false)
	require.NoError(t, err)
	err = app.initCache()
	require.NoError(t, err)
	app.initMarkdown()
	app.initSessions()

	bc := app.cfg.Blogs[app.cfg.DefaultBlog]

	addr, _, err := app.createComment(bc, "https://example.com/abc", "Test", "Name", "https://example.org", "")
	require.NoError(t, err)

	splittedAddr := strings.Split(addr, "/")
	id := cast.ToInt(splittedAddr[len(splittedAddr)-1])

	comments, err := app.db.getComments(&commentsRequestConfig{id: id})
	require.NoError(t, err)
	require.Len(t, comments, 1)

	comment := comments[0]

	assert.Equal(t, "/abc", comment.Target)
	assert.Equal(t, "Test", comment.Comment)
	assert.Equal(t, "Name", comment.Name)
	assert.Equal(t, "https://example.org", comment.Website)

	handlerClient := newHandlerClient(http.HandlerFunc(app.serveCommentsEditor))

	requests.URL("https://example.com/comment/edit").
		Method(http.MethodPost).
		ParamInt("id", id).
		Param("name", "Edited name").
		Param("comment", "Edited comment").
		Param("website", "").
		Client(handlerClient).
		Fetch(context.Background())

	comments, err = app.db.getComments(&commentsRequestConfig{id: id})
	require.NoError(t, err)
	require.Len(t, comments, 1)

	comment = comments[0]

	assert.Equal(t, "/abc", comment.Target)
	assert.Equal(t, "Edited comment", comment.Comment)
	assert.Equal(t, "Edited name", comment.Name)
	assert.Equal(t, "", comment.Website)

}
