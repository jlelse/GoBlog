package main

import (
	"bytes"
	"strings"

	kemoji "github.com/kyokomi/emoji"
	"github.com/yuin/goldmark"
	emoji "github.com/yuin/goldmark-emoji"
	"github.com/yuin/goldmark-emoji/definition"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

var emojilib definition.Emojis

var markdown goldmark.Markdown

func initMarkdown() {
	markdown = goldmark.New(
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithExtensions(
			extension.Table,
			extension.Strikethrough,
			extension.Footnote,
			extension.Typographer,
			extension.Linkify,
			// Emojis
			emoji.New(
				emoji.WithEmojis(emojiGoLib()),
			),
			// Custom
			&customExtension{},
		),
	)
}

func renderMarkdown(source string) ([]byte, error) {
	var buffer bytes.Buffer
	err := markdown.Convert([]byte(source), &buffer)
	return buffer.Bytes(), err
}

// Extensions etc...

// All emojis from emoji lib
func emojiGoLib() definition.Emojis {
	if emojilib == nil {
		var emojis []definition.Emoji
		for shotcode, e := range kemoji.CodeMap() {
			emojis = append(emojis, definition.NewEmoji(e, []rune(e), strings.ReplaceAll(shotcode, ":", "")))
		}
		emojilib = definition.NewEmojis(emojis...)
	}
	return emojilib
}

// Links
type customExtension struct{}

func (l *customExtension) Extend(m goldmark.Markdown) {
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(&customRenderer{}, 500),
	))
}

type customRenderer struct{}

func (c *customRenderer) RegisterFuncs(r renderer.NodeRendererFuncRegisterer) {
	r.Register(ast.KindLink, c.renderLink)
	r.Register(ast.KindImage, c.renderImage)
}

func (c *customRenderer) renderLink(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		n := node.(*ast.Link)
		_, _ = w.WriteString("<a href=\"")
		// Make URL absolute if it's relative
		newDestination := util.URLEscape(n.Destination, true)
		if bytes.HasPrefix(newDestination, []byte("/")) {
			_, _ = w.Write(util.EscapeHTML([]byte(appConfig.Server.PublicAddress)))
		}
		_, _ = w.Write(util.EscapeHTML(newDestination))
		_, _ = w.WriteRune('"')
		// Open external links (links that start with "http") in new tab
		if bytes.HasPrefix(n.Destination, []byte("http")) {
			_, _ = w.WriteString(` target="_blank" rel="noopener"`)
		}
		// Title
		if n.Title != nil {
			_, _ = w.WriteString(" title=\"")
			_, _ = w.Write(n.Title)
			_, _ = w.WriteRune('"')
		}
		_, _ = w.WriteRune('>')
	} else {
		_, _ = w.WriteString("</a>")
	}
	return ast.WalkContinue, nil
}

func (c *customRenderer) renderImage(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*ast.Image)
	// Make URL absolute if it's relative
	destination := util.URLEscape(n.Destination, true)
	if bytes.HasPrefix(destination, []byte("/")) {
		destination = util.EscapeHTML(append([]byte(appConfig.Server.PublicAddress), destination...))
	} else {
		destination = util.EscapeHTML(destination)
	}
	_, _ = w.WriteString("<a href=\"")
	_, _ = w.Write(destination)
	_, _ = w.WriteString("\">")
	_, _ = w.WriteString("<img src=\"")
	_, _ = w.Write(destination)
	_, _ = w.WriteString("\" alt=\"")
	_, _ = w.Write(util.EscapeHTML(n.Text(source)))
	_ = w.WriteByte('"')
	_, _ = w.WriteString(" loading=\"lazy\"")
	if n.Title != nil {
		_, _ = w.WriteString(" title=\"")
		_, _ = w.Write(n.Title)
		_ = w.WriteByte('"')
	}
	_, _ = w.WriteString("></a>")
	return ast.WalkSkipChildren, nil
}
