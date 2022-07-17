package main

import (
	"context"
	"net/http"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/posener/wstest"
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
		app.serveEditorPreview(rw, r.WithContext(context.WithValue(r.Context(), blogKey, "default")))
	})
	d := wstest.NewDialer(h)

	c, resp, err := d.Dial("ws://example.com/editor/preview", nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.NotNil(t, c)

	err = c.WriteMessage(websocket.TextMessage, []byte("---\ntitle: Title\nsection: posts\n---\nContent."))
	require.NoError(t, err)

	mt, msg, err := c.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, websocket.TextMessage, mt)

	msgStr := string(msg)
	require.Contains(t, msgStr, "<h1")
	require.Contains(t, msgStr, ">Title")
	require.Contains(t, msgStr, "<p>Content")
	require.Contains(t, msgStr, "Posts")

}
