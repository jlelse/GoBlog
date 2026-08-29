package mark

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yuin/goldmark/v2/parser"
	"github.com/yuin/goldmark/v2/renderer/html"
)

func convert(t *testing.T, source string) string {
	p := parser.New(parser.WithExtensions(Parser))
	r := html.New(html.WithExtensions(HTMLRenderer))
	doc := p.Parse([]byte(source))
	var buf bytes.Buffer
	err := r.Render(&buf, []byte(source), doc)
	assert.NoError(t, err)
	return buf.String()
}

func TestMark(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		assert.Equal(t, "<p><mark>Hi</mark> Hello, world!</p>\n", convert(t, "==Hi== Hello, world!"))
	})
	t.Run("does not span paragraphs", func(t *testing.T) {
		assert.Equal(t, "<p>This ==has a</p>\n<p>new paragraph==.</p>\n", convert(t, "This ==has a\n\nnew paragraph==."))
	})
	t.Run("short runs are plain text", func(t *testing.T) {
		assert.Equal(t, "<p>=</p>\n", convert(t, "="))
		assert.Equal(t, "<p>==</p>\n", convert(t, "=="))
		assert.Equal(t, "<p>= =</p>\n", convert(t, "= ="))
		assert.Equal(t, "<p>a===b</p>\n", convert(t, "a===b"))
	})
	t.Run("triple run nests like emphasis", func(t *testing.T) {
		assert.Equal(t, "<p><mark><mark>text</mark></mark></p>\n", convert(t, "===text==="))
	})
	t.Run("consecutive delimiters", func(t *testing.T) {
		assert.Equal(t, "<p><mark>a</mark><mark>b</mark>=</p>\n", convert(t, "==a===b=="))
	})
}
