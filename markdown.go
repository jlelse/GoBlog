package main

import (
	"fmt"
	"io"
	"runtime/debug"
	"sort"
	"strings"

	chromahtml "github.com/alecthomas/chroma/v3/formatters/html"
	emoji "github.com/yuin/goldmark-emoji/v2"
	highlighting "github.com/yuin/goldmark-highlighting/v3"
	"github.com/yuin/goldmark/v2/ast"
	"github.com/yuin/goldmark/v2/extension"
	"github.com/yuin/goldmark/v2/parser"
	"github.com/yuin/goldmark/v2/renderer"
	"github.com/yuin/goldmark/v2/renderer/html"
	"github.com/yuin/goldmark/v2/text"
	"github.com/yuin/goldmark/v2/util"
	"go.goblog.app/app/pkgs/bufferpool"
	"go.goblog.app/app/pkgs/builderpool"
	"go.goblog.app/app/pkgs/htmlbuilder"
	"go.goblog.app/app/pkgs/mark"
)

// Render options for rendering post markdown.
type renderOptions struct {
	absoluteLinks bool
	postPath      string
	simpleImages  bool
}

// Render state passed through the rendering context.
type renderState struct {
	app     *goBlog
	options renderOptions
}

var renderStateKey = renderer.NewContextKey()

func (a *goBlog) renderContext(options renderOptions) renderer.Context {
	ctx := renderer.NewContext()
	ctx.Set(renderStateKey, renderState{
		app:     a,
		options: options,
	})
	return ctx
}

func stateFromContext(rc renderer.Context) renderState {
	if rc != nil {
		if s, ok := rc.Get(renderStateKey).(renderState); ok {
			return s
		}
	}
	return renderState{}
}

func (a *goBlog) initMarkdown() {
	a.initMarkdownOnce.Do(func() {
		a.mdParser = parser.New(a.defaultMarkdownParserOptions()...)
		a.mdRenderer = html.New(a.defaultMarkdownRendererOptions(
			html.WithExtensions(customRenderer{}),
		)...)
		a.titleMdParser = parser.New(
			parser.WithDefaultParsers(false),
			parser.WithBlockParsers(
				util.Prioritized(parser.NewParagraphParser(), 1000),
			),
			parser.WithExtensions(
				extension.TypographerParser,
				emoji.Parser,
			),
		)
		a.titleMdRenderer = html.New(
			html.WithExtensions(
				emoji.HTMLRenderer,
			),
		)
	})
}

func (a *goBlog) defaultMarkdownParserOptions() []parser.Option {
	return []parser.Option{
		parser.WithAutoHeadingID(),
		parser.WithExtensions(
			extension.TableParser,
			extension.StrikethroughParser,
			extension.FootnoteParser,
			extension.TypographerParser,
			extension.LinkifyParser,
			mark.Parser,
			emoji.Parser,
		),
	}
}

// chromaStyleName is the chroma style used for syntax highlighting.
const chromaStyleName = "monokai"

func (a *goBlog) defaultMarkdownRendererOptions(additional ...html.Option) []html.Option {
	return append([]html.Option{
		html.WithUnsafe(),
		html.WithExtensions(
			extension.TableHTMLRenderer,
			extension.StrikethroughHTMLRenderer,
			extension.FootnoteHTMLRenderer,
			mark.HTMLRenderer,
			emoji.HTMLRenderer,
			highlighting.NewHTMLRenderer(
				highlighting.WithStyle(chromaStyleName),
				highlighting.WithFormatterOptions(
					chromahtml.WithClasses(true),
					chromahtml.ClassPrefix("c-"),
				),
			),
		),
	}, additional...)
}

func (a *goBlog) safeMarkdownRender(fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			a.error("Panic while rendering markdown", "panic", r, "stack", string(debug.Stack()))
			err = fmt.Errorf("panic while rendering markdown: %v", r)
		}
	}()
	return fn()
}

func (a *goBlog) renderMarkdownToWriter(w io.Writer, source string) (err error) {
	err = a.safeMarkdownRender(func() error {
		a.initMarkdown()
		return a.mdRenderer.RenderStringSource(w, source, a.mdParser.ParseStringSource(source), renderer.WithContext(a.renderContext(renderOptions{})))
	})
	if err != nil {
		a.error("Error while rendering markdown", "err", err)
	}
	return err
}

func (a *goBlog) renderText(s string) (text string, err error) {
	if s == "" {
		return "", nil
	}
	err = a.safeMarkdownRender(func() error {
		buf := bufferpool.Get()
		defer bufferpool.Put(buf)
		if err := a.renderMarkdownToWriter(buf, s); err != nil {
			return err
		}
		text, err = htmlText(buf)
		return err
	})
	if err != nil {
		a.error("Error while rendering markdown as text", "err", err)
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
	var text string
	err := a.safeMarkdownRender(func() error {
		a.initMarkdown()
		buf := bufferpool.Get()
		defer bufferpool.Put(buf)
		if err := a.titleMdRenderer.RenderStringSource(buf, s, a.titleMdParser.ParseStringSource(s)); err != nil {
			return err
		}
		var err error
		text, err = htmlText(buf)
		return err
	})
	if err != nil {
		a.error("Error while rendering markdown title", "err", err)
		return ""
	}
	return text
}

func (a *goBlog) renderPostMarkdownToWriter(w io.Writer, source string, absoluteLinks bool, postPath string, simpleImages bool) (err error) {
	err = a.safeMarkdownRender(func() error {
		a.initMarkdown()
		return a.mdRenderer.RenderStringSource(w, source, a.mdParser.ParseStringSource(source), renderer.WithContext(a.renderContext(renderOptions{
			absoluteLinks: absoluteLinks,
			postPath:      postPath,
			simpleImages:  simpleImages,
		})))
	})
	if err != nil {
		a.error("Error while rendering post markdown", "err", err)
	}
	return err
}

// RewriteMarkdownURLs makes relative link and image destinations absolute and
// replaces media URLs with optimized variants where available.
func (a *goBlog) RewriteMarkdownURLs(markdown string) (result string, err error) {
	if markdown == "" {
		return "", nil
	}
	err = a.safeMarkdownRender(func() error {
		a.initMarkdown()
		source := []byte(markdown)
		doc := a.mdParser.ParseStringSource(markdown)
		type replacement struct {
			start, stop int
			value       string
		}
		var replacements []replacement
		if err := ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
			if !entering {
				return ast.WalkContinue, nil
			}
			var destination text.SingleLineValue
			switch n := node.(type) {
			case *ast.Link:
				destination = n.Destination
			case *ast.Image:
				destination = n.Destination
			default:
				return ast.WalkContinue, nil
			}
			if destination.IsEmpty() || destination.IsOwned() {
				return ast.WalkContinue, nil
			}
			original := destination.Str(source)
			rewritten := a.rewriteMarkdownURL(original)
			if rewritten == original {
				return ast.WalkContinue, nil
			}
			index := destination.Index()
			replacements = append(replacements, replacement{start: index.Start, stop: index.Stop, value: rewritten})
			return ast.WalkContinue, nil
		}); err != nil {
			return err
		}
		sort.Slice(replacements, func(i, j int) bool {
			return replacements[i].start < replacements[j].start
		})
		var b strings.Builder
		last := 0
		for _, r := range replacements {
			if r.start < last || r.stop > len(markdown) {
				continue
			}
			b.WriteString(markdown[last:r.start])
			b.WriteString(r.value)
			last = r.stop
		}
		b.WriteString(markdown[last:])
		// Also catch media URLs in places the AST doesn't expose, such as link
		// reference definitions or raw HTML.
		result = mediaImgLinkRe.ReplaceAllStringFunc(b.String(), func(match string) string {
			return a.mediaFallbackURL(match)
		})
		return nil
	})
	if err != nil {
		a.error("Error while rewriting markdown URLs", "err", err)
		return "", err
	}
	return result, nil
}

func (a *goBlog) rewriteMarkdownURL(destination string) string {
	if publicAddress := a.getFullAddress(""); publicAddress != "" {
		if resolved, err := resolveURLReferences(publicAddress, destination); err == nil && len(resolved) > 0 {
			destination = resolved[0]
		}
	}
	return a.mediaFallbackURL(destination)
}

// Extensions etc...

// Links
type customRenderer struct{}

func (customRenderer) RendererOptions(_ *html.Config) []html.Option {
	return []html.Option{
		html.WithNodeRenderer(ast.KindLink, html.NodeRendererFunc(renderLink)),
		html.WithNodeRenderer(ast.KindImage, html.NodeRendererFunc(renderImage)),
	}
}

func renderLink(w io.Writer, source []byte, node ast.Node, entering bool, rc renderer.Context) (ast.WalkStatus, error) {
	state := stateFromContext(rc)
	hb := htmlbuilder.NewHTMLBuilder(w)
	if entering {
		n := node.(*ast.Link)
		originalDest := n.Destination.Value(source)
		dest := originalDest
		if publicAddress := state.app.getFullAddress(""); state.options.absoluteLinks && publicAddress != "" {
			resolved, err := resolveURLReferences(publicAddress, dest)
			if err != nil {
				return ast.WalkStop, err
			}
			if len(resolved) > 0 {
				dest = resolved[0]
			}
		}
		dest = state.app.mediaFallbackURL(dest)
		tagOpts := []any{"href", dest}
		if isAbsoluteURL(originalDest) {
			tagOpts = append(tagOpts, "target", "_blank", "rel", "noopener")
		}
		if !n.Title.IsEmpty() {
			tagOpts = append(tagOpts, "title", n.Title.Value(source))
		}
		hb.WriteElementOpen("a", tagOpts...)
	} else {
		hb.WriteElementClose("a")
	}
	return ast.WalkContinue, nil
}

func renderImage(w io.Writer, source []byte, node ast.Node, entering bool, rc renderer.Context) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	state := stateFromContext(rc)
	n := node.(*ast.Image)
	dest := n.Destination.Value(source)
	// Make destination absolute if it's relative
	if publicAddress := state.app.getFullAddress(""); state.options.absoluteLinks && publicAddress != "" {
		resolved, err := resolveURLReferences(publicAddress, dest)
		if err != nil {
			return ast.WalkStop, err
		}
		if len(resolved) > 0 {
			dest = resolved[0]
		}
	}
	hb := htmlbuilder.NewHTMLBuilder(w)
	state.app.writePictureElement(hb, dest, extractTextFromChildren(n, source), n.Title.Value(source), "", state.options.postPath, state.options.simpleImages)
	return ast.WalkSkipChildren, nil
}

func extractTextFromChildren(node ast.Node, source []byte) string {
	if node == nil {
		return ""
	}
	b := builderpool.Get()
	defer builderpool.Put(b)
	for ch := node.FirstChild(); ch != nil; ch = ch.NextSibling() {
		if t, ok := ch.(*ast.Text); ok {
			b.WriteString(t.Value.Str(source))
		} else {
			b.WriteString(extractTextFromChildren(ch, source))
		}
	}
	return b.String()
}
