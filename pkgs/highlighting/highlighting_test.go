package highlighting

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yuin/goldmark"
)

func TestHighlighting_Extend(t *testing.T) {
	md := goldmark.New(
		goldmark.WithExtensions(Highlighting),
	)

	var buf bytes.Buffer
	source := "```go\npackage main\n\nfunc main() {\n\tprintln(\"Hello, World!\")\n}\n```\n"
	err := md.Convert([]byte(source), &buf)
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "<span class=\"c-kn\">package</span>")
	assert.Contains(t, buf.String(), "<span class=\"c-s\">&#34;Hello, World!&#34;</span>")
}

func TestHighlighting_Unknown(t *testing.T) {
	md := goldmark.New(
		goldmark.WithExtensions(Highlighting),
	)

	var buf bytes.Buffer
	source := "```unknownlang\nThis is some text.\n```\n"
	err := md.Convert([]byte(source), &buf)
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "<pre class=\"c-chroma\"><code><span class=\"c-line\"><span class=\"c-cl\">This is some text.\n</span></span></code></pre>")
}

func TestHighlighting_NoLang(t *testing.T) {
	md := goldmark.New(
		goldmark.WithExtensions(Highlighting),
	)

	var buf bytes.Buffer
	source := "```\nThis is a code block without a language.\n```\n"
	err := md.Convert([]byte(source), &buf)
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "<pre class=\"c-chroma\"><code><span class=\"c-line\"><span class=\"c-cl\">This is a code block without a language.\n</span></span></code></pre>")
}
