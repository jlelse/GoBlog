package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func dialWS(t *testing.T, h http.Handler, query string) *websocket.Conn {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	wsURL := "ws://" + strings.TrimPrefix(srv.URL, "http://") + "/editor/ws?" + query
	c, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close(); _ = resp.Body.Close() })
	require.NotNil(t, c)
	return c
}

func Test_editorPreview(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	_ = app.initConfig(false)
	_ = app.initTemplateStrings()

	h := http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		app.serveEditorWebsocket(rw, r.WithContext(context.WithValue(r.Context(), blogKey, "default")))
	})

	c := dialWS(t, h, "preview=1")

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
	assert.Equal(t, "preview:yaml: construct errors:\n  line 2: mapping key \"title\" already defined at line 1", msgStr)
}

func Test_editorSync(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	_ = app.initConfig(false)

	h := http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		app.serveEditorWebsocket(rw, r.WithContext(context.WithValue(r.Context(), blogKey, "default")))
	})

	c1 := dialWS(t, h, "sync=1")
	c2 := dialWS(t, h, "sync=1")

	// Send message on editor connection 1, should be received by connection 2

	err := c1.WriteMessage(websocket.TextMessage, []byte("Test"))
	require.NoError(t, err)

	mt, msg, err := c2.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, websocket.TextMessage, mt)

	msgStr := string(msg)
	assert.Equal(t, "sync:Test", msgStr)

	// Connection 3 should receive the initial state

	c3 := dialWS(t, h, "sync=1")

	mt, msg, err = c3.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, websocket.TextMessage, mt)

	msgStr = string(msg)
	assert.Equal(t, "sync:Test", msgStr)
}
