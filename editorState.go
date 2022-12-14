package main

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	ws "nhooyr.io/websocket"
)

func (a *goBlog) serveEditorStateSync(w http.ResponseWriter, r *http.Request) {
	// Get blog
	blog, bc := a.getBlog(r)
	// Open websocket connection
	c, err := ws.Accept(w, r, &ws.AcceptOptions{CompressionMode: ws.CompressionContextTakeover})
	if err != nil {
		return
	}
	c.SetReadLimit(1 << 20) // 1MB
	defer c.Close(ws.StatusNormalClosure, "")
	// Store connection to be able to send updates
	connectionId := uuid.NewString()
	bc.esws.Store(connectionId, c)
	defer bc.esws.Delete(connectionId)
	// Set cancel context
	ctx, cancel := context.WithTimeout(r.Context(), time.Hour*6)
	defer cancel()
	// Send initial content
	if r.URL.Query().Get("initial") == "1" {
		initialState, err := a.getEditorStateFromDatabase(ctx, blog)
		if err != nil {
			return
		}
		if initialState != nil {
			w, err := c.Writer(ctx, ws.MessageText)
			if err != nil {
				return
			}
			_, err = w.Write(initialState)
			if err != nil {
				return
			}
			_ = w.Close()
		}
	}
	// Listen for new messages
	for {
		// Retrieve content
		mt, message, err := c.Reader(ctx)
		if err != nil {
			break
		}
		if mt != ws.MessageText {
			continue
		}
		messageBytes, err := io.ReadAll(message)
		if err != nil {
			break
		}
		// Save editor state
		bc.esm.Lock()
		a.updateEditorStateInDatabase(ctx, blog, messageBytes)
		bc.esm.Unlock()
		// Send editor state to other connections
		a.sendNewEditorStateToAllConnections(ctx, bc, connectionId, messageBytes)
	}
}

func (*goBlog) sendNewEditorStateToAllConnections(ctx context.Context, bc *configBlog, origin string, state []byte) {
	bc.esws.Range(func(key, value any) bool {
		if key == origin {
			return true
		}
		c, ok := value.(*ws.Conn)
		if !ok {
			return true
		}
		w, err := c.Writer(ctx, ws.MessageText)
		if err != nil {
			bc.esws.Delete(key)
			return true
		}
		defer w.Close()
		_, err = w.Write(state)
		if err != nil {
			bc.esws.Delete(key)
			return true
		}
		return true
	})
}

const editorStateCacheKey = "editorstate_"

func (a *goBlog) updateEditorStateInDatabase(ctx context.Context, blog string, state []byte) {
	_ = a.db.cachePersistentlyContext(ctx, editorStateCacheKey+blog, state)
}

func (a *goBlog) getEditorStateFromDatabase(ctx context.Context, blog string) ([]byte, error) {
	return a.db.retrievePersistentCacheContext(ctx, editorStateCacheKey+blog)
}
