package main

import (
	"context"
	"net/http"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/posener/wstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_editorPreview(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	_ = app.initConfig(false)
	app.initMarkdown()
	_ = app.initTemplateStrings()

	h := http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		app.serveEditorWebsocket(rw, r.WithContext(context.WithValue(r.Context(), blogKey, "default")))
	})
	d := wstest.NewDialer(h)

	c, resp, err := d.Dial("ws://example.com/editor/ws?preview=1", nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.NotNil(t, c)

	// Should receive the trigger to send content for preview

	mt, msg, err := c.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, websocket.TextMessage, mt)

	msgStr := string(msg)
	assert.Equal(t, "triggerpreview", msgStr)

	// Write correct preview content

	err = c.WriteMessage(websocket.TextMessage, []byte("---\ntitle: Title\nsection: posts\n---\nContent."))
	require.NoError(t, err)

	mt, msg, err = c.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, websocket.TextMessage, mt)

	msgStr = string(msg)
	assert.Contains(t, msgStr, "preview:")
	assert.Contains(t, msgStr, "<h1")
	assert.Contains(t, msgStr, ">Title")
	assert.Contains(t, msgStr, "<p>Content")
	assert.Contains(t, msgStr, "Posts")

	// Write content that fails to render

	err = c.WriteMessage(websocket.TextMessage, []byte("---\ntitle: Title\ntitle: Test\n---\nContent."))
	require.NoError(t, err)

	mt, msg, err = c.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, websocket.TextMessage, mt)

	msgStr = string(msg)
	assert.Equal(t, "preview:yaml: unmarshal errors:\n  line 2: mapping key \"title\" already defined at line 1", msgStr)
}

func Test_editorSync(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	_ = app.initConfig(false)

	h := http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		app.serveEditorWebsocket(rw, r.WithContext(context.WithValue(r.Context(), blogKey, "default")))
	})

	d1 := wstest.NewDialer(h)
	c1, resp1, err := d1.Dial("ws://example.com/editor/ws?sync=1", nil)
	require.NoError(t, err)
	defer resp1.Body.Close()
	require.NotNil(t, c1)

	d2 := wstest.NewDialer(h)
	c2, resp2, err := d2.Dial("ws://example.com/editor/ws?sync=1", nil)
	require.NoError(t, err)
	defer resp2.Body.Close()
	require.NotNil(t, c2)

	// Send message on editor connection 1, should be received by connection 2

	err = c1.WriteMessage(websocket.TextMessage, []byte("Test"))
	require.NoError(t, err)

	mt, msg, err := c2.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, websocket.TextMessage, mt)

	msgStr := string(msg)
	assert.Equal(t, "sync:Test", msgStr)

	// Connection 3 should receive the initial state

	d3 := wstest.NewDialer(h)
	c3, resp3, err := d3.Dial("ws://example.com/editor/ws?sync=1", nil)
	require.NoError(t, err)
	defer resp3.Body.Close()
	require.NotNil(t, c2)

	mt, msg, err = c3.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, websocket.TextMessage, mt)

	msgStr = string(msg)
	assert.Equal(t, "sync:Test", msgStr)
}
