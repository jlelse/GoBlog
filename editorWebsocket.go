package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	ws "github.com/coder/websocket"
	"github.com/google/uuid"
	"go.goblog.app/app/pkgs/bodylimit"
	"go.goblog.app/app/pkgs/contenttype"
	"go.goblog.app/app/pkgs/htmlbuilder"
)

func (a *goBlog) serveEditorWebsocket(w http.ResponseWriter, r *http.Request) {
	enablePreview := r.URL.Query().Get("preview") == "1"
	enableSync := r.URL.Query().Get("sync") == "1"
	if !enablePreview && !enableSync {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	// Get blog
	blog, bc := a.getBlog(r)
	// Open websocket connection
	c, err := ws.Accept(w, r, &ws.AcceptOptions{CompressionMode: ws.CompressionContextTakeover})
	if err != nil {
		return
	}
	c.SetReadLimit(10 * bodylimit.MB)
	defer c.Close(ws.StatusNormalClosure, "")
	// Store connection to be able to send updates
	var connectionId string
	if enableSync {
		connectionId = uuid.NewString()
		bc.esws.Store(connectionId, c)
		defer bc.esws.Delete(connectionId)
	}
	// Set cancel context
	ctx, cancel := context.WithTimeout(r.Context(), time.Hour*6)
	defer cancel()
	// Send initial content
	if enableSync {
		initialState, err := a.getEditorStateFromDatabase(ctx, blog)
		if err != nil {
			return
		}
		if initialState != nil {
			// Send initial state
			if err := a.sendEditorState(ctx, c, initialState); err != nil {
				return
			}
			// Send preview
			if enablePreview {
				if err := a.sendEditorPreview(ctx, c, blog, initialState); err != nil {
					return
				}
			}
		}
	} else if !enableSync && enablePreview {
		// Trigger editor to send content to generate the preview
		w, err := c.Writer(ctx, ws.MessageText)
		if err != nil {
			return
		}
		if _, err := io.WriteString(w, "triggerpreview"); err != nil {
			_ = w.Close()
			return
		}
		if err := w.Close(); err != nil {
			return
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
		// and send editor state to other connections
		if enableSync {
			bc.esm.Lock()
			a.updateEditorStateInDatabase(ctx, blog, messageBytes)
			bc.esm.Unlock()
			a.sendNewEditorStateToAllConnections(ctx, bc, connectionId, messageBytes)
		}
		// Create preview
		if enablePreview {
			if err := a.sendEditorPreview(ctx, c, blog, messageBytes); err != nil {
				break
			}
		}
	}
}

// SYNC

func (a *goBlog) sendNewEditorStateToAllConnections(ctx context.Context, bc *configBlog, origin string, state []byte) {
	bc.esws.Range(func(key, value any) bool {
		if key == origin {
			return true
		}
		c, ok := value.(*ws.Conn)
		if !ok {
			return true
		}
		if err := a.sendEditorState(ctx, c, state); err != nil {
			bc.esws.Delete(key)
			return true
		}
		return true
	})
}

func (*goBlog) sendEditorState(ctx context.Context, c *ws.Conn, state []byte) error {
	w, err := c.Writer(ctx, ws.MessageText)
	if err != nil {
		return err
	}
	if _, err := io.WriteString(w, "sync:"); err != nil {
		return errors.Join(err, w.Close())
	}
	if _, err := w.Write(state); err != nil {
		return errors.Join(err, w.Close())
	}
	return w.Close()
}

const editorStateCacheKey = "editorstate_"

func (a *goBlog) updateEditorStateInDatabase(ctx context.Context, blog string, state []byte) {
	_ = a.db.cachePersistentlyContext(ctx, editorStateCacheKey+blog, state)
}

func (a *goBlog) getEditorStateFromDatabase(ctx context.Context, blog string) ([]byte, error) {
	return a.db.retrievePersistentCacheContext(ctx, editorStateCacheKey+blog)
}

// PREVIEW

func (a *goBlog) sendEditorPreview(ctx context.Context, c *ws.Conn, blog string, md []byte) error {
	// Get writer
	w, err := c.Writer(ctx, ws.MessageText)
	if err != nil {
		return err
	}
	if _, err := io.WriteString(w, "preview:"); err != nil {
		return errors.Join(err, w.Close())
	}
	// Create preview post
	p := &post{
		Blog:    blog,
		Content: string(md),
	}
	if err := a.extractParamsFromContent(p); err != nil {
		_, werr := io.WriteString(w, err.Error())
		return errors.Join(werr, w.Close())
	}
	if err := a.checkPost(p, true, true); err != nil {
		_, werr := io.WriteString(w, err.Error())
		return errors.Join(werr, w.Close())
	}
	if t := p.Title(); t != "" {
		p.RenderedTitle = a.renderMdTitle(t)
	}
	// Render post (using post's blog config)
	pr, pw := io.Pipe()
	go func() {
		a.renderEditorPreview(htmlbuilder.NewHtmlBuilder(pw), a.getBlogFromPost(p), p)
		_ = pw.Close()
	}()
	_ = a.min.Get().Minify(contenttype.HTMLUTF8, w, pr)
	_ = pr.Close()
	return w.Close()
}
