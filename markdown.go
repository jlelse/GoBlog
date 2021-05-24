package main

import (
	"bytes"

	marktag "git.jlel.se/jlelse/goldmark-mark"
	"github.com/PuerkitoBio/goquery"
	"github.com/yuin/goldmark"
	emoji "github.com/yuin/goldmark-emoji"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

var defaultMarkdown, absoluteMarkdown goldmark.Markdown

func initMarkdown() {
	defaultGoldmarkOptions := []goldmark.Option{
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
			marktag.Mark,
			emoji.Emoji,
		),
	}
	defaultMarkdown = goldmark.New(append(defaultGoldmarkOptions, goldmark.WithExtensions(&customExtension{absoluteLinks: false}))...)
	absoluteMarkdown = goldmark.New(append(defaultGoldmarkOptions, goldmark.WithExtensions(&customExtension{absoluteLinks: true}))...)
}

func renderMarkdown(source string, absoluteLinks bool) (rendered []byte, err error) {
	var buffer bytes.Buffer
	if absoluteLinks {
		err = absoluteMarkdown.Convert([]byte(source), &buffer)
	} else {
		err = defaultMarkdown.Convert([]byte(source), &buffer)
	}
	return buffer.Bytes(), err
}

func renderText(s string) string {
	h, err := renderMarkdown(s, false)
	if err != nil {
		return ""
	}
	d, err := goquery.NewDocumentFromReader(bytes.NewReader(h))
	if err != nil {
		return ""
	}
	return d.Text()
}

// Extensions etc...

// Links
type customExtension struct {
	absoluteLinks bool
}

func (l *customExtension) Extend(m goldmark.Markdown) {
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(&customRenderer{
			absoluteLinks: l.absoluteLinks,
		}, 500),
	))
}

type customRenderer struct {
	absoluteLinks bool
}

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
		if c.absoluteLinks && bytes.HasPrefix(newDestination, []byte("/")) {
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
