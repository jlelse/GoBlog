package highlighting

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yuin/goldmark/v2/parser"
	"github.com/yuin/goldmark/v2/renderer/html"
)

func convert(t *testing.T, source string) string {
	p := parser.New()
	r := html.New(html.WithExtensions(Highlighting))
	doc := p.Parse([]byte(source))
	var buf bytes.Buffer
	err := r.Render(&buf, []byte(source), doc)
	assert.NoError(t, err)
	return buf.String()
}

func TestHighlighting_Extend(t *testing.T) {
	source := "```go\npackage main\n\nfunc main() {\n\tprintln(\"Hello, World!\")\n}\n```\n"
	output := convert(t, source)
	assert.Contains(t, output, "<span class=\"c-kn\">package</span>")
	assert.Contains(t, output, "<span class=\"c-s\">&#34;Hello, World!&#34;</span>")
}

func TestHighlighting_Unknown(t *testing.T) {
	source := "```unknownlang\nThis is some text.\n```\n"
	output := convert(t, source)
	assert.Contains(t, output, "<pre class=\"c-chroma\"><code><span class=\"c-line\"><span class=\"c-cl\">This is some text.\n</span></span></code></pre>")
}

func TestHighlighting_NoLang(t *testing.T) {
	source := "```\nThis is a code block without a language.\n```\n"
	output := convert(t, source)
	assert.Contains(t, output, "<pre class=\"c-chroma\"><code><span class=\"c-line\"><span class=\"c-cl\">This is a code block without a language.\n</span></span></code></pre>")
}
