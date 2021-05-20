package main

import (
	"bytes"
	"strings"

	"github.com/PuerkitoBio/goquery"
	kemoji "github.com/kyokomi/emoji/v2"
	"github.com/yuin/goldmark"
	emoji "github.com/yuin/goldmark-emoji"
	"github.com/yuin/goldmark-emoji/definition"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

var emojilib definition.Emojis

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
			// Emojis
			emoji.New(
				emoji.WithEmojis(emojiGoLib()),
			),
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
type customExtension struct {
	absoluteLinks bool
}

func (l *customExtension) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(parser.WithInlineParsers(
		util.Prioritized(&markdownMarkParser{}, 500),
	))
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
	r.Register(kindMarkdownMark, c.renderMarkTag)
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

type markdownMark struct {
	ast.BaseInline
}

func (n *markdownMark) Kind() ast.NodeKind {
	return kindMarkdownMark
}

func (n *markdownMark) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, nil, nil)
}

type markDelimiterProcessor struct {
}

func (p *markDelimiterProcessor) IsDelimiter(b byte) bool {
	return b == '='
}

func (p *markDelimiterProcessor) CanOpenCloser(opener, closer *parser.Delimiter) bool {
	return opener.Char == closer.Char
}

func (p *markDelimiterProcessor) OnMatch(consumes int) ast.Node {
	return &markdownMark{}
}

var defaultMarkDelimiterProcessor = &markDelimiterProcessor{}

type markdownMarkParser struct {
}

func (s *markdownMarkParser) Trigger() []byte {
	return []byte{'='}
}

func (s *markdownMarkParser) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	before := block.PrecendingCharacter()
	line, segment := block.PeekLine()
	node := parser.ScanDelimiter(line, before, 2, defaultMarkDelimiterProcessor)
	if node == nil {
		return nil
	}
	node.Segment = segment.WithStop(segment.Start + node.OriginalLength)
	block.Advance(node.OriginalLength)
	pc.PushDelimiter(node)
	return node
}

func (s *markdownMarkParser) CloseBlock(parent ast.Node, pc parser.Context) {
}

var kindMarkdownMark = ast.NewNodeKind("Mark")

func (c *customRenderer) renderMarkTag(w util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		_, _ = w.WriteString("<mark>")
	} else {
		_, _ = w.WriteString("</mark>")
	}
	return ast.WalkContinue, nil
}
