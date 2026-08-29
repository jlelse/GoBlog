// Package mark provides a goldmark extension that adds support for the
// ==mark== inline syntax, rendered as <mark> elements.
package mark

import (
	"io"

	"github.com/yuin/goldmark/v2/ast"
	"github.com/yuin/goldmark/v2/parser"
	"github.com/yuin/goldmark/v2/renderer"
	"github.com/yuin/goldmark/v2/renderer/html"
	"github.com/yuin/goldmark/v2/text"
	"github.com/yuin/goldmark/v2/util"
)

// Mark is a node that represents a mark.
type Mark struct {
	ast.BaseInline
}

// Dump implements Node.Dump.
func (n *Mark) Dump(_ []byte) *ast.NodeDump {
	return ast.NewNodeDump(n, nil)
}

// KindMark is a NodeKind of the Mark node.
var KindMark = ast.NewNodeKind("Mark")

// Kind implements Node.Kind.
func (n *Mark) Kind() ast.NodeKind {
	return KindMark
}

// NewMark returns a new Mark node.
func NewMark() *Mark {
	n := &Mark{}
	n.Init(n)
	return n
}

type markDelimiterProcessor struct{}

func (p *markDelimiterProcessor) IsDelimiter(b byte) bool {
	return b == '='
}

func (p *markDelimiterProcessor) CanOpenCloser(opener, closer *parser.Delimiter) bool {
	return opener.Char == closer.Char
}

func (p *markDelimiterProcessor) OnMatch(_ int) ast.Node {
	return NewMark()
}

var defaultMarkDelimiterProcessor = &markDelimiterProcessor{}

type markParser struct{}

var defaultMarkParser = &markParser{}

// NewMarkParser returns a new InlineParser that parses mark expressions.
func NewMarkParser() parser.InlineParser {
	return defaultMarkParser
}

func (s *markParser) Trigger() []byte {
	return []byte{'='}
}

func (s *markParser) Parse(_ ast.Node, block text.Reader, pc parser.Context) ast.Node {
	before := block.PrecedingCharacter()
	if before == '=' {
		return nil
	}
	line, _ := block.PeekLine()
	n := 0
	for n < len(line) && line[n] == '=' {
		n++
	}
	if n < 2 {
		return nil
	}
	return parser.ParseDelimiter(block, 2, defaultMarkDelimiterProcessor, pc)
}

type markParserExtension struct{}

// NewParser returns a new parser.Extension for parsing mark expressions.
func NewParser() parser.Extension {
	return &markParserExtension{}
}

func (e *markParserExtension) ParserOptions(_ *parser.Config) []parser.Option {
	return []parser.Option{
		parser.WithInlineParsers(
			util.Prioritized(NewMarkParser(), 500),
		),
	}
}

// Parser is the default parser.Extension for parsing mark expressions.
var Parser = NewParser()

// MarkAttributeFilter defines attribute names which mark elements can have.
var MarkAttributeFilter = html.GlobalAttributeFilter

type markHTMLRendererExtension struct{}

// NewHTMLRenderer returns a new html.Extension for rendering Mark nodes.
func NewHTMLRenderer() html.Extension {
	return &markHTMLRendererExtension{}
}

func (r *markHTMLRendererExtension) RendererOptions(_ *html.Config) []html.Option {
	return []html.Option{
		html.WithNodeRenderers(map[ast.NodeKind]html.NodeRenderer{
			KindMark: html.NodeRendererFunc(r.renderMark),
		}),
	}
}

func (r *markHTMLRendererExtension) renderMark(
	writer io.Writer, source []byte, n ast.Node, entering bool, rc renderer.Context) (ast.WalkStatus, error) {
	w := writer.(util.BufWriter)
	if entering {
		if n.Attributes() != nil {
			_, _ = w.WriteString("<mark")
			html.RenderAttributes(w, source, n, MarkAttributeFilter, rc)
			_ = w.WriteByte('>')
		} else {
			_, _ = w.WriteString("<mark>")
		}
	} else {
		_, _ = w.WriteString("</mark>")
	}
	return ast.WalkContinue, nil
}

// HTMLRenderer is the default html.Extension for rendering Mark nodes.
var HTMLRenderer = NewHTMLRenderer()
