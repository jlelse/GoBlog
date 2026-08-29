// Package highlighting provides syntax highlighting support.
package highlighting

import (
	"io"

	"github.com/yuin/goldmark/v2/ast"
	"github.com/yuin/goldmark/v2/renderer"
	"github.com/yuin/goldmark/v2/renderer/html"
	"github.com/yuin/goldmark/v2/util"

	"github.com/alecthomas/chroma/v2"
	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

type highlighting struct {
	formatter *chromahtml.Formatter
}

// Highlighting is a goldmark html.Extension implementation.
var Highlighting html.Extension = &highlighting{
	formatter: chromahtml.New(
		chromahtml.ClassPrefix("c-"),
		chromahtml.WithClasses(true),
	),
}

// Style is the chroma style used for syntax highlighting.
var Style = styles.Get("monokai")

// RendererOptions implements html.Extension.
func (e *highlighting) RendererOptions(_ *html.Config) []html.Option {
	return []html.Option{
		html.WithNodeRendererDecorator(ast.KindCodeBlock, func(next html.NodeRenderer) html.NodeRenderer {
			return html.NodeRendererFunc(func(w io.Writer, source []byte, node ast.Node, entering bool, rc renderer.Context) (ast.WalkStatus, error) {
				n, ok := node.(*ast.CodeBlock)
				if !ok || n.CodeBlockKind != ast.CodeBlockKindFenced {
					return next.Render(w, source, node, entering, rc)
				}
				return renderFencedCodeBlock(w, source, n, entering, rc, e.formatter)
			})
		}),
	}
}

func renderFencedCodeBlock(w io.Writer, source []byte, n *ast.CodeBlock, entering bool, rc renderer.Context, formatter *chromahtml.Formatter) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}

	language, _ := n.Language(source)

	// Try to highlight.
	if highlight(w, n.Value.Str(source), language, formatter) != nil {
		// Highlight failed, fallback to plain text.
		bw := w.(util.BufWriter)
		_, _ = bw.WriteString("<pre><code>")
		tw := html.ContextTextWriter(rc)
		_, _ = n.Value.WriteTo(tw, source)
		_, _ = bw.WriteString("</code></pre>\n")
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
