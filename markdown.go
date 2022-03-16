package main

import (
	"bytes"
	"io"

	marktag "git.jlel.se/jlelse/goldmark-mark"
	"github.com/yuin/goldmark"
	emoji "github.com/yuin/goldmark-emoji"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
	"go.goblog.app/app/pkgs/bufferpool"
	"go.goblog.app/app/pkgs/highlighting"
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
			highlighting.Highlighting,
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

func (a *goBlog) renderMarkdownToWriter(w io.Writer, source string, absoluteLinks bool) (err error) {
	if absoluteLinks {
		err = a.absoluteMd.Convert([]byte(source), w)
	} else {
		err = a.md.Convert([]byte(source), w)
	}
	return err
}

func (a *goBlog) renderText(s string) string {
	if s == "" {
		return ""
	}
	buf := bufferpool.Get()
	defer bufferpool.Put(buf)
	err := a.renderMarkdownToWriter(buf, s, false)
	if err != nil {
		return ""
	}
	text, err := htmlTextFromReader(buf)
	if err != nil {
		return ""
	}
	return text
}

func (a *goBlog) renderMdTitle(s string) string {
	if s == "" {
		return ""
	}
	buf := bufferpool.Get()
	defer bufferpool.Put(buf)
	err := a.titleMd.Convert([]byte(s), buf)
	if err != nil {
		return ""
	}
	text, err := htmlTextFromReader(buf)
	if err != nil {
		return ""
	}
	return text
}

// Extensions etc...

// Links
type customExtension struct {
	publicAddress string
	absoluteLinks bool
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
	publicAddress string
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
	dest := string(n.Destination)
	// Make destination absolute if it's relative
	if c.absoluteLinks && c.publicAddress != "" {
		resolved, err := resolveURLReferences(c.publicAddress, dest)
		if err != nil {
			return ast.WalkStop, err
		}
		if len(resolved) > 0 {
			dest = resolved[0]
		}
	}
	hb := newHtmlBuilder(w)
	hb.writeElementOpen("a", "href", dest)
	imgEls := []any{"src", dest, "alt", string(n.Text(source)), "loading", "lazy"}
	if len(n.Title) > 0 {
		imgEls = append(imgEls, "title", string(n.Title))
	}
	hb.writeElementOpen("img", imgEls...)
	hb.writeElementClose("a")
	return ast.WalkSkipChildren, nil
}
