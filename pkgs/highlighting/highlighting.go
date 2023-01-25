package highlighting

import (
	"io"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
	"go.goblog.app/app/pkgs/bufferpool"

	"github.com/alecthomas/chroma/v2"
	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

type config struct {
	html.Config
	formatter *chromahtml.Formatter
}

func newConfig() *config {
	return &config{
		Config: html.NewConfig(),
		formatter: chromahtml.New(
			chromahtml.ClassPrefix("c-"),
			chromahtml.WithClasses(true),
		),
	}
}

// SetOption implements renderer.SetOptioner.
func (c *config) SetOption(name renderer.OptionName, value any) {
	c.Config.SetOption(name, value)
}

// htmlRenderer struct is a renderer.NodeRenderer implementation for the extension.
type htmlRenderer struct {
	*config
}

func newHTMLRenderer() renderer.NodeRenderer {
	return &htmlRenderer{
		config: newConfig(),
	}
}

// RegisterFuncs implements NodeRenderer.RegisterFuncs.
func (r *htmlRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindFencedCodeBlock, r.renderFencedCodeBlock)
}

func (r *htmlRenderer) renderFencedCodeBlock(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}

	n := node.(*ast.FencedCodeBlock)

	// Read code block content.
	buf := bufferpool.Get()
	defer bufferpool.Put(buf)
	for _, line := range n.Lines().Sliced(0, n.Lines().Len()) {
		buf.Write(line.Value(source))
	}

	// Try to highlight.
	if highlight(w, buf.String(), string(n.Language(source)), r.formatter) != nil {
		// Highlight failed, fallback to plain text.
		_, _ = w.WriteString("<pre><code>")
		r.Writer.RawWrite(w, buf.Bytes())
		_, _ = w.WriteString("</code></pre>\n")
	}

	return ast.WalkContinue, nil
}

func highlight(w io.Writer, source, language string, f *chromahtml.Formatter) error {
	l := lexers.Get(language)
	if l == nil {
		l = lexers.Fallback
	}
	l = chroma.Coalesce(l)
	it, err := l.Tokenise(nil, source)
	if err != nil {
		return err
	}
	return f.Format(w, Style, it)
}

type highlighting struct{}

// Highlighting is a goldmark.Extender implementation.
var Highlighting = &highlighting{}

var Style = styles.Monokai

// Extend implements goldmark.Extender.
func (*highlighting) Extend(m goldmark.Markdown) {
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(newHTMLRenderer(), 200),
	))
}
