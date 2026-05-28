package main

import (
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
	"go.goblog.app/app/pkgs/builderpool"
	"go.goblog.app/app/pkgs/highlighting"
	"go.goblog.app/app/pkgs/htmlbuilder"
)

func (a *goBlog) initMarkdown() {
	a.initMarkdownOnce.Do(func() {
		defaultGoldmarkOptions := a.defaultGoldmarkOptions()
		publicAddress := ""
		if srv := a.cfg.Server; srv != nil {
			publicAddress = srv.PublicAddress
		}
		a.md = goldmark.New(append(defaultGoldmarkOptions, goldmark.WithExtensions(&customExtension{
			absoluteLinks: false,
			publicAddress: publicAddress,
			app:           a,
		}))...)
		a.titleMd = goldmark.New(
			goldmark.WithParser(
				// Override, no need for special Markdown parsers
				parser.NewParser(
					parser.WithBlockParsers(util.Prioritized(parser.NewParagraphParser(), 1000)),
				),
			),
			goldmark.WithExtensions(
				extension.Typographer,
				emoji.Emoji,
			),
		)
	})
}

func (a *goBlog) defaultGoldmarkOptions() []goldmark.Option {
	return []goldmark.Option{
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
}

func (a *goBlog) renderMarkdownToWriter(w io.Writer, source string) (err error) {
	a.initMarkdown()
	return a.md.Convert([]byte(source), w)
}

func (a *goBlog) renderText(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	pr, pw := io.Pipe()
	go func() {
		_ = pw.CloseWithError(a.renderMarkdownToWriter(pw, s))
	}()
	text, err := htmlTextFromReader(pr)
	_ = pr.CloseWithError(err)
	if err != nil {
		return "", nil
	}
	return text, nil
}

func (a *goBlog) renderTextSafe(s string) string {
	r, _ := a.renderText(s)
	return r
}

func (a *goBlog) renderMdTitle(s string) string {
	if s == "" {
		return ""
	}
	a.initMarkdown()
	pr, pw := io.Pipe()
	go func() {
		_ = pw.CloseWithError(a.titleMd.Convert([]byte(s), pw))
	}()
	text, err := htmlTextFromReader(pr)
	_ = pr.CloseWithError(err)
	if err != nil {
		return ""
	}
	return text
}

func (a *goBlog) renderPostMarkdownToWriter(w io.Writer, source string, absoluteLinks bool, postPath string, simpleImages bool) (err error) {
	a.initMarkdown()
	publicAddress := ""
	if srv := a.cfg.Server; srv != nil {
		publicAddress = srv.PublicAddress
	}
	md := goldmark.New(append(a.defaultGoldmarkOptions(), goldmark.WithExtensions(&customExtension{
		absoluteLinks: absoluteLinks,
		publicAddress: publicAddress,
		app:           a,
		postPath:      postPath,
		simpleImages:  simpleImages,
	}))...)
	return md.Convert([]byte(source), w)
}

// Extensions etc...

// Links
type customExtension struct {
	publicAddress string
	absoluteLinks bool
	app           *goBlog
	postPath      string
	simpleImages  bool
}

func (l *customExtension) Extend(m goldmark.Markdown) {
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(&customRenderer{
			absoluteLinks: l.absoluteLinks,
			publicAddress: l.publicAddress,
			app:           l.app,
			postPath:      l.postPath,
			simpleImages:  l.simpleImages,
		}, 500),
	))
}

type customRenderer struct {
	publicAddress string
	absoluteLinks bool
	app           *goBlog
	postPath      string
	simpleImages  bool
}

func (c *customRenderer) RegisterFuncs(r renderer.NodeRendererFuncRegisterer) {
	r.Register(ast.KindLink, c.renderLink)
	r.Register(ast.KindImage, c.renderImage)
}

func (c *customRenderer) renderLink(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	hb := htmlbuilder.NewHTMLBuilder(w)
	if entering {
		n := node.(*ast.Link)
		dest := string(n.Destination)
		if c.absoluteLinks && c.publicAddress != "" {
			resolved, err := resolveURLReferences(c.publicAddress, dest)
			if err != nil {
				return ast.WalkStop, err
			}
			if len(resolved) > 0 {
				dest = resolved[0]
			}
		}
		tagOpts := []any{"href", dest}
		if isAbsoluteURL(string(n.Destination)) {
			tagOpts = append(tagOpts, "target", "_blank", "rel", "noopener")
		}
		if n.Title != nil {
			tagOpts = append(tagOpts, "title", string(n.Title))
		}
		hb.WriteElementOpen("a", tagOpts...)
	} else {
		hb.WriteElementClose("a")
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
	hb := htmlbuilder.NewHTMLBuilder(w)
	c.app.writePictureElement(hb, dest, c.extractTextFromChildren(n, source), string(n.Title), "", c.postPath, c.simpleImages)
	return ast.WalkSkipChildren, nil
}

func (c *customRenderer) extractTextFromChildren(node ast.Node, source []byte) string {
	if node == nil {
		return ""
	}
	b := builderpool.Get()
	defer builderpool.Put(b)
	for ch := node.FirstChild(); ch != nil; ch = ch.NextSibling() {
		if s, ok := ch.(*ast.String); ok {
			b.Write(s.Value)
		} else if t, ok := ch.(*ast.Text); ok {
			b.Write(t.Segment.Value(source))
		} else {
			b.WriteString(c.extractTextFromChildren(ch, source))
		}
	}
	return b.String()
}
