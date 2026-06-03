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

func Test_editorFormat(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	_ = app.initConfig(false)
	_ = app.initTemplateStrings()

	h := http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		app.serveEditorWebsocket(rw, r.WithContext(context.WithValue(r.Context(), blogKey, "default")))
	})

	t.Run("Bold with selection", func(t *testing.T) {
		c := dialWS(t, h, "preview=1")

		// Consume triggerpreview
		mt, msg, err := c.ReadMessage()
		require.NoError(t, err)
		require.Equal(t, websocket.TextMessage, mt)
		require.Equal(t, "triggerpreview", string(msg))

		// "Hello World": select "World" at bytes 6..11
		err = c.WriteMessage(websocket.TextMessage, []byte("format:bold:6:11:Hello World"))
		require.NoError(t, err)

		mt, msg, err = c.ReadMessage()
		require.NoError(t, err)
		require.Equal(t, websocket.TextMessage, mt)
		assert.Equal(t, "formatted:Hello **World**:15", string(msg))
	})

	t.Run("Italic with selection", func(t *testing.T) {
		c := dialWS(t, h, "preview=1")

		mt, msg, err := c.ReadMessage()
		require.NoError(t, err)
		require.Equal(t, "triggerpreview", string(msg))

		// "Hello World": select "Hello" at bytes 0..5
		err = c.WriteMessage(websocket.TextMessage, []byte("format:italic:0:5:Hello World"))
		require.NoError(t, err)

		mt, msg, err = c.ReadMessage()
		require.NoError(t, err)
		require.Equal(t, websocket.TextMessage, mt)
		assert.Equal(t, "formatted:*Hello* World:7", string(msg))
	})

	t.Run("Strikethrough with selection", func(t *testing.T) {
		c := dialWS(t, h, "preview=1")

		mt, msg, err := c.ReadMessage()
		require.NoError(t, err)
		require.Equal(t, "triggerpreview", string(msg))

		// "Hello World": select "World" at bytes 6..11
		err = c.WriteMessage(websocket.TextMessage, []byte("format:strikethrough:6:11:Hello World"))
		require.NoError(t, err)

		mt, msg, err = c.ReadMessage()
		require.NoError(t, err)
		require.Equal(t, websocket.TextMessage, mt)
		assert.Equal(t, "formatted:Hello ~~World~~:15", string(msg))
	})

	t.Run("Link with selection", func(t *testing.T) {
		c := dialWS(t, h, "preview=1")

		mt, msg, err := c.ReadMessage()
		require.NoError(t, err)
		require.Equal(t, "triggerpreview", string(msg))

		// "Hello World": select "World" at bytes 6..11
		err = c.WriteMessage(websocket.TextMessage, []byte("format:link:6:11:Hello World"))
		require.NoError(t, err)

		mt, msg, err = c.ReadMessage()
		require.NoError(t, err)
		require.Equal(t, websocket.TextMessage, mt)
		// [World]() — cursor after "(" at position 14
		assert.Equal(t, "formatted:Hello [World]():14", string(msg))
	})

	t.Run("Link with no selection", func(t *testing.T) {
		c := dialWS(t, h, "preview=1")

		mt, msg, err := c.ReadMessage()
		require.NoError(t, err)
		require.Equal(t, "triggerpreview", string(msg))

		// start==end=5, inserts []()
		err = c.WriteMessage(websocket.TextMessage, []byte("format:link:5:5:Hello World"))
		require.NoError(t, err)

		mt, msg, err = c.ReadMessage()
		require.NoError(t, err)
		require.Equal(t, websocket.TextMessage, mt)
		// []() = 4 chars, cursor after "[" at 5+1=6
		assert.Equal(t, "formatted:Hello[]() World:6", string(msg))
	})

	t.Run("Bold with no selection inserts markers", func(t *testing.T) {
		c := dialWS(t, h, "preview=1")

		mt, msg, err := c.ReadMessage()
		require.NoError(t, err)
		require.Equal(t, "triggerpreview", string(msg))

		// start==end=5 means no selection, inserts empty bold markers
		err = c.WriteMessage(websocket.TextMessage, []byte("format:bold:5:5:Hello World"))
		require.NoError(t, err)

		mt, msg, err = c.ReadMessage()
		require.NoError(t, err)
		require.Equal(t, websocket.TextMessage, mt)
		// "Hello" + "****" + " World" = "Hello**** World", cursor at 5+2=7
		assert.Equal(t, "formatted:Hello**** World:7", string(msg))
	})

	t.Run("Unknown action is ignored", func(t *testing.T) {
		c := dialWS(t, h, "preview=1")

		mt, msg, err := c.ReadMessage()
		require.NoError(t, err)
		require.Equal(t, "triggerpreview", string(msg))

		err = c.WriteMessage(websocket.TextMessage, []byte("format:underline:0:5:Hello"))
		require.NoError(t, err)

		// Send something else to get a response (the unknown action sends nothing back)
		err = c.WriteMessage(websocket.TextMessage, []byte("Some content"))
		require.NoError(t, err)

		mt, msg, err = c.ReadMessage()
		require.NoError(t, err)
		require.Equal(t, websocket.TextMessage, mt)
		// Should get a preview for the regular content
		assert.Contains(t, string(msg), "preview:")
	})

	t.Run("Malformed message with too few parts", func(t *testing.T) {
		c := dialWS(t, h, "preview=1")

		mt, msg, err := c.ReadMessage()
		require.NoError(t, err)
		require.Equal(t, "triggerpreview", string(msg))

		err = c.WriteMessage(websocket.TextMessage, []byte("format:bold:0"))
		require.NoError(t, err)

		// Send something else to verify connection still works
		err = c.WriteMessage(websocket.TextMessage, []byte("Test"))
		require.NoError(t, err)

		mt, msg, err = c.ReadMessage()
		require.NoError(t, err)
		require.Equal(t, websocket.TextMessage, mt)
		assert.Contains(t, string(msg), "preview:")
	})

	t.Run("Invalid start value", func(t *testing.T) {
		c := dialWS(t, h, "preview=1")

		mt, msg, err := c.ReadMessage()
		require.NoError(t, err)
		require.Equal(t, "triggerpreview", string(msg))

		err = c.WriteMessage(websocket.TextMessage, []byte("format:bold:abc:5:Hello"))
		require.NoError(t, err)

		// Send something else to verify connection still works
		err = c.WriteMessage(websocket.TextMessage, []byte("Test"))
		require.NoError(t, err)

		mt, msg, err = c.ReadMessage()
		require.NoError(t, err)
		require.Equal(t, websocket.TextMessage, mt)
		assert.Contains(t, string(msg), "preview:")
	})

	t.Run("Start beyond content length", func(t *testing.T) {
		c := dialWS(t, h, "preview=1")

		mt, msg, err := c.ReadMessage()
		require.NoError(t, err)
		require.Equal(t, "triggerpreview", string(msg))

		err = c.WriteMessage(websocket.TextMessage, []byte("format:bold:0:100:Hello"))
		require.NoError(t, err)

		// Send something else to verify connection still works
		err = c.WriteMessage(websocket.TextMessage, []byte("Test"))
		require.NoError(t, err)

		mt, msg, err = c.ReadMessage()
		require.NoError(t, err)
		require.Equal(t, websocket.TextMessage, mt)
		assert.Contains(t, string(msg), "preview:")
	})

	t.Run("Content with colons", func(t *testing.T) {
		c := dialWS(t, h, "preview=1")

		mt, msg, err := c.ReadMessage()
		require.NoError(t, err)
		require.Equal(t, "triggerpreview", string(msg))

		// Content "Hello: World" with selection "World" (positions 7-12)
		err = c.WriteMessage(websocket.TextMessage, []byte("format:bold:7:12:Hello: World"))
		require.NoError(t, err)

		mt, msg, err = c.ReadMessage()
		require.NoError(t, err)
		require.Equal(t, websocket.TextMessage, mt)
		assert.Equal(t, "formatted:Hello: **World**:16", string(msg))
	})

	t.Run("Format with sync does not broadcast", func(t *testing.T) {
		c1 := dialWS(t, h, "sync=1")
		c2 := dialWS(t, h, "sync=1")

		// Seed state so c2 gets initial sync
		err := c1.WriteMessage(websocket.TextMessage, []byte("Initial"))
		require.NoError(t, err)

		mt, msg, err := c2.ReadMessage()
		require.NoError(t, err)
		require.Equal(t, "sync:Initial", string(msg))

		// Send format message on c1
		err = c1.WriteMessage(websocket.TextMessage, []byte("format:bold:0:5:Hello"))
		require.NoError(t, err)

		// c1 should receive the formatted response
		mt, msg, err = c1.ReadMessage()
		require.NoError(t, err)
		require.Equal(t, websocket.TextMessage, mt)
		assert.Equal(t, "formatted:**Hello**:9", string(msg))

		// A regular message after format should still sync to c2
		err = c1.WriteMessage(websocket.TextMessage, []byte("After format"))
		require.NoError(t, err)

		mt, msg, err = c2.ReadMessage()
		require.NoError(t, err)
		require.Equal(t, websocket.TextMessage, mt)
		assert.Equal(t, "sync:After format", string(msg))
	})
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
