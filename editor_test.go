package main

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/posener/wstest"
	"github.com/stretchr/testify/require"
)

func Test_editorPreview(t *testing.T) {

	app := &goBlog{
		cfg: &config{
			Server: &configServer{},
			Db: &configDb{
				File: filepath.Join(t.TempDir(), "test.db"),
			},
			Blogs: map[string]*configBlog{
				"en": {
					Lang: "en",
					Sections: map[string]*configSection{
						"test": {
							Title: "Test",
						},
					},
					DefaultSection: "test",
				},
			},
			DefaultBlog: "en",
			Micropub:    &configMicropub{},
		},
	}
	_ = app.initDatabase(false)
	app.initComponents(false)

	h := http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		app.serveEditorPreview(rw, r.WithContext(context.WithValue(r.Context(), blogKey, "en")))
	})

	d := wstest.NewDialer(h)

	c, _, err := d.Dial("ws://whatever/editor/preview", nil)
	require.NoError(t, err)
	require.NotNil(t, c)

	err = c.WriteMessage(websocket.TextMessage, []byte("---\ntitle: Title\nsection: test\n---\nContent."))
	require.NoError(t, err)

	mt, msg, err := c.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, websocket.TextMessage, mt)

	msgStr := string(msg)
	require.Contains(t, msgStr, "<h1>Title")
	require.Contains(t, msgStr, "<p>Content")
	require.Contains(t, msgStr, "Test")

}
