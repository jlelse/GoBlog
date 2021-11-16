package main

import (
	"bytes"
	"html/template"

	marktag "git.jlel.se/jlelse/goldmark-mark"
	chromahtml "github.com/alecthomas/chroma/formatters/html"
	"github.com/yuin/goldmark"
	emoji "github.com/yuin/goldmark-emoji"
	highlighting "github.com/yuin/goldmark-highlighting"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

func (a *goBlog) initMarkdown() {
	if a.md != nil && a.absoluteMd != nil && a.titleMd != nil {
		// Already initialized
		return
	}
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
			highlighting.NewHighlighting(
				highlighting.WithCustomStyle(chromaGoBlogStyle),
				highlighting.WithFormatOptions(
					chromahtml.ClassPrefix("c-"),
					chromahtml.WithClasses(true),
				),
			),
		),
	}
	publicAddress := ""
	if srv := a.cfg.Server; srv != nil {
		publicAddress = srv.PublicAddress
	}
	a.md = goldmark.New(append(defaultGoldmarkOptions, goldmark.WithExtensions(&customExtension{
		absoluteLinks: false,
		publicAddress: publicAddress,
	}))...)
	a.absoluteMd = goldmark.New(append(defaultGoldmarkOptions, goldmark.WithExtensions(&customExtension{
		absoluteLinks: true,
		publicAddress: publicAddress,
	}))...)
	a.titleMd = goldmark.New(
		goldmark.WithParser(
			// Override, no need for special Markdown parsers
			parser.NewParser(
				parser.WithBlockParsers(
					util.Prioritized(parser.NewParagraphParser(), 1000)),
				parser.WithInlineParsers(),
				parser.WithParagraphTransformers(),
			),
		),
		goldmark.WithExtensions(
			extension.Typographer,
			emoji.Emoji,
		),
	)
}

func (a *goBlog) renderMarkdown(source string, absoluteLinks bool) (rendered []byte, err error) {
	var buffer bytes.Buffer
	if absoluteLinks {
		err = a.absoluteMd.Convert([]byte(source), &buffer)
	} else {
		err = a.md.Convert([]byte(source), &buffer)
	}
	return buffer.Bytes(), err
}

func (a *goBlog) renderMarkdownAsHTML(source string, absoluteLinks bool) (rendered template.HTML, err error) {
	b, err := a.renderMarkdown(source, absoluteLinks)
	if err != nil {
		return "", err
	}
	return template.HTML(b), nil
}

func (a *goBlog) safeRenderMarkdownAsHTML(source string) template.HTML {
	h, _ := a.renderMarkdownAsHTML(source, false)
	return h
}

func (a *goBlog) renderText(s string) string {
	if s == "" {
		return ""
	}
	h, err := a.renderMarkdown(s, false)
	if err != nil {
		return ""
	}
	return htmlText(string(h))
}

func (a *goBlog) renderMdTitle(s string) string {
	if s == "" {
		return ""
	}
	var buffer bytes.Buffer
	err := a.titleMd.Convert([]byte(s), &buffer)
	if err != nil {
		return ""
	}
	return htmlText(buffer.String())
}

// Extensions etc...

// Links
type customExtension struct {
	absoluteLinks bool
	publicAddress string
}

func (l *customExtension) Extend(m goldmark.Markdown) {
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(&customRenderer{
			absoluteLinks: l.absoluteLinks,
			publicAddress: l.publicAddress,
		}, 500),
	))
}

type customRenderer struct {
	absoluteLinks bool
	publicAddress string
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
		if c.absoluteLinks && c.publicAddress != "" && bytes.HasPrefix(newDestination, []byte("/")) {
			_, _ = w.Write(util.EscapeHTML([]byte(c.publicAddress)))
		}
		_, _ = w.Write(util.EscapeHTML(newDestination))
		_, _ = w.WriteRune('"')
		// Open external links (links that start with "http") in new tab
		if isAbsoluteURL(string(n.Destination)) {
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
	if c.absoluteLinks && c.publicAddress != "" && bytes.HasPrefix(destination, []byte("/")) {
		destination = util.EscapeHTML(append([]byte(c.publicAddress), destination...))
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
