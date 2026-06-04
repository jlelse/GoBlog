package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	ws "github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/samber/lo"
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
	var connectionID string
	if enableSync {
		connectionID = uuid.NewString()
		bc.esws.Store(connectionID, c)
		defer bc.esws.Delete(connectionID)
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
		// Handle format messages
		if strings.HasPrefix(string(messageBytes), "format:") {
			a.handleEditorFormat(ctx, c, messageBytes)
			continue
		}
		// Save editor state
		// and send editor state to other connections
		if enableSync {
			bc.esm.Lock()
			a.updateEditorStateInDatabase(ctx, blog, messageBytes)
			bc.esm.Unlock()
			a.sendNewEditorStateToAllConnections(ctx, bc, connectionID, messageBytes)
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

// FORMAT

// jsPosToByteOffset converts a JS UTF-16 code unit position to a byte offset in a UTF-8 Go string.
func jsPosToByteOffset(s string, jsPos int) int {
	if jsPos <= 0 {
		return 0
	}
	units := 0
	for bytePos, r := range s {
		rUnits := 1
		if r > 0xFFFF {
			rUnits = 2
		}
		if units+rUnits > jsPos {
			return bytePos
		}
		units += rUnits
		if units == jsPos {
			continue
		}
	}
	return len(s)
}

// byteOffsetToJsPos converts a UTF-8 byte offset to a JS UTF-16 code unit position.
func byteOffsetToJsPos(s string, byteOff int) int {
	if byteOff <= 0 {
		return 0
	}
	units := 0
	for bytePos, r := range s {
		if bytePos >= byteOff {
			return units
		}
		if r > 0xFFFF {
			units += 2
		} else {
			units++
		}
	}
	return units
}

// jsLen returns the JS string length (UTF-16 code units) of a Go UTF-8 string.
func jsLen(s string) int {
	length := 0
	for _, r := range s {
		if r == utf8.RuneError {
			length++
			continue
		}
		if r > 0xFFFF {
			length += 2
		} else {
			length++
		}
	}
	return length
}

func (a *goBlog) handleEditorFormat(ctx context.Context, c *ws.Conn, msg []byte) {
	// Parse: format:<action>:<start>:<end>:<content>
	parts := strings.SplitN(string(msg), ":", 5)
	if len(parts) < 5 {
		return
	}
	action := parts[1]
	jsStart, err := strconv.Atoi(parts[2])
	if err != nil {
		return
	}
	jsEnd, err := strconv.Atoi(parts[3])
	if err != nil {
		return
	}
	content := parts[4]

	if l := jsLen(content); jsStart > l || jsEnd > l || jsStart > jsEnd {
		return
	}

	// Convert JS UTF-16 code unit positions to UTF-8 byte offsets
	start := jsPosToByteOffset(content, jsStart)
	end := jsPosToByteOffset(content, jsEnd)

	if start > len(content) || end > len(content) {
		return
	}

	selected := content[start:end]
	var replacement string
	var cursorOffset, cursorOffsetSelected int

	switch action {
	case "bold":
		replacement = "**" + selected + "**"
		cursorOffset, cursorOffsetSelected = 2, len(replacement)
	case "italic":
		replacement = "*" + selected + "*"
		cursorOffset, cursorOffsetSelected = 1, len(replacement)
	case "strikethrough":
		replacement = "~~" + selected + "~~"
		cursorOffset, cursorOffsetSelected = 2, len(replacement)
	case "link":
		replacement = "[" + selected + "]()"
		cursorOffset, cursorOffsetSelected = 1, len(selected)+3
	default:
		return
	}

	newContent := content[:start] + replacement + content[end:]
	byteCursor := start + lo.If(start == end, cursorOffset).Else(cursorOffsetSelected)

	// Send formatted content back with JS cursor position
	err = c.Write(ctx, ws.MessageText,
		fmt.Appendf(nil, "formatted:%s:%d", newContent, byteOffsetToJsPos(newContent, byteCursor)))
	if err != nil {
		return
	}
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
	if err := a.processContentAndParameters(p); err != nil {
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
		a.renderEditorPreview(htmlbuilder.NewHTMLBuilder(pw), a.getBlogFromPost(p), p)
		_ = pw.Close()
	}()
	_ = a.min.Get().Minify(contenttype.HTMLUTF8, w, pr)
	_ = pr.Close()
	return w.Close()
}
