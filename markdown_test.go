package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.goblog.app/app/pkgs/bufferpool"
)

func (a *goBlog) renderMarkdown(source string, absoluteLinks bool) (rendered []byte, err error) {
	buffer := bufferpool.Get()
	err = a.renderMarkdownToWriter(buffer, source, absoluteLinks)
	rendered = buffer.Bytes()
	bufferpool.Put(buffer)
	return
}

func Test_markdown(t *testing.T) {
	t.Run("Basic Markdown tests", func(t *testing.T) {
		app := &goBlog{
			cfg: &config{
				Server: &configServer{
					PublicAddress: "https://example.com",
				},
			},
		}

		app.initMarkdown()

		// Relative / absolute links

		rendered, err := app.renderMarkdown("[Relative](/relative)", false)
		require.NoError(t, err)

		assert.Contains(t, string(rendered), `href="/relative"`)

		rendered, err = app.renderMarkdown("[Relative](/relative)", true)
		require.NoError(t, err)

		assert.Contains(t, string(rendered), `href="https://example.com/relative"`)
		assert.NotContains(t, string(rendered), `target="_blank"`)

		// Images

		rendered, err = app.renderMarkdown("![](/relative)", false)
		require.NoError(t, err)

		assert.Contains(t, string(rendered), `src="/relative"`)
		assert.Contains(t, string(rendered), `href="/relative"`)

		rendered, err = app.renderMarkdown("![](/relative)", true)
		require.NoError(t, err)

		assert.Contains(t, string(rendered), `src="https://example.com/relative"`)
		assert.Contains(t, string(rendered), `href="https://example.com/relative"`)

		// Image title

		rendered, err = app.renderMarkdown(`![](/test "Test-Title")`, false)
		require.NoError(t, err)

		assert.Contains(t, string(rendered), `title="Test-Title"`)

		// External links

		rendered, err = app.renderMarkdown("[External](https://example.com)", true)
		require.NoError(t, err)

		assert.Contains(t, string(rendered), `target="_blank"`)

		// Link title

		rendered, err = app.renderMarkdown(`[With title](https://example.com "Test-Title")`, true)
		require.NoError(t, err)

		assert.Contains(t, string(rendered), `title="Test-Title"`)

		// Text

		renderedText, err := app.renderText("**This** *is* [text](/)")
		assert.Equal(t, "This is text", renderedText)
		assert.NoError(t, err)

		// Title

		assert.Equal(t, "3. **Test**", app.renderMdTitle("3. **Test**"))
		assert.Equal(t, "Test’s", app.renderMdTitle("Test's"))
		assert.Equal(t, "😂", app.renderMdTitle(":joy:"))
		assert.Equal(t, "<b></b>", app.renderMdTitle("<b></b>"))
	})
}

func Benchmark_markdown(b *testing.B) {
	markdownExample, err := os.ReadFile("testdata/markdownexample.md")
	if err != nil {
		b.Errorf("Failed to read markdown example: %v", err)
	}
	mdExp := string(markdownExample)

	app := &goBlog{
		cfg: &config{
			Server: &configServer{
				PublicAddress: "https://example.com",
			},
		},
	}

	app.initMarkdown()

	b.Run("Markdown Rendering", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := app.renderMarkdown(mdExp, true)
			if err != nil {
				b.Errorf("Error: %v", err)
			}
		}
	})

	b.Run("Title Rendering", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			app.renderMdTitle("**Test**")
		}
	})

	b.Run("Text Rendering", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			app.renderText("**Test**")
		}
	})
}
