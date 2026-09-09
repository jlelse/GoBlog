package main

import (
	"bytes"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.goblog.app/app/pkgs/bufferpool"
)

func (a *goBlog) renderMarkdown(source string) (rendered []byte, err error) {
	buffer := bufferpool.Get()
	err = a.renderMarkdownToWriter(buffer, source)
	rendered = buffer.Bytes()
	bufferpool.Put(buffer)
	return
}

func Test_markdown(t *testing.T) {
	t.Run("Basic Markdown tests", func(t *testing.T) {
		cfg := createDefaultTestConfig(t)
		cfg.Server.PublicAddress = "https://example.com"
		app := &goBlog{cfg: cfg}

		// Relative links

		rendered, err := app.renderMarkdown("[Relative](/relative)")
		require.NoError(t, err)

		assert.Contains(t, string(rendered), `href="/relative"`)

		// Images

		rendered, err = app.renderMarkdown("![](/relative)")
		require.NoError(t, err)

		assert.Contains(t, string(rendered), `src="/relative"`)
		assert.Contains(t, string(rendered), `href="/relative"`)

		// Image title

		rendered, err = app.renderMarkdown(`![](/test "Test-Title")`)
		require.NoError(t, err)

		assert.Contains(t, string(rendered), `title="Test-Title"`)

		// Image alt text

		rendered, err = app.renderMarkdown(`![Test-Alt](/test)`)
		require.NoError(t, err)

		assert.Contains(t, string(rendered), `alt="Test-Alt"`)

		rendered, err = app.renderMarkdown(`![*Test-Alt*](/test)`)
		require.NoError(t, err)

		assert.Contains(t, string(rendered), `alt="Test-Alt"`)

		// External links

		rendered, err = app.renderMarkdown("[External](https://example.com)")
		require.NoError(t, err)

		assert.Contains(t, string(rendered), `target="_blank"`)

		// Link title

		rendered, err = app.renderMarkdown(`[With title](https://example.com "Test-Title")`)
		require.NoError(t, err)

		assert.Contains(t, string(rendered), `title="Test-Title"`)

		// Text

		renderedText, err := app.renderText("**This** *is* [text](/)")
		assert.Equal(t, "This is text", renderedText)
		assert.NoError(t, err)

		// Mark

		rendered, err = app.renderMarkdown("This is ==marked== text.")
		require.NoError(t, err)

		assert.Contains(t, string(rendered), "<mark>marked</mark>")

		// Title

		assert.Equal(t, "3. **Test**", app.renderMdTitle("3. **Test**"))
		assert.Equal(t, "Test\u2019s", app.renderMdTitle("Test's"))
		assert.Equal(t, "😂", app.renderMdTitle(":joy:"))
		assert.Equal(t, "<b></b>", app.renderMdTitle("<b></b>"))

		// Syntax highlighting

		rendered, err = app.renderMarkdown("```go\npackage main\n\nfunc main() {\n\tprintln(\"Hello, World!\")\n}\n```\n")
		require.NoError(t, err)

		assert.Contains(t, string(rendered), `<span class="c-kn">package</span>`)
		assert.Contains(t, string(rendered), `<span class="c-s">&#34;Hello, World!&#34;</span>`)

		// Code blocks without a known language are rendered as plain code

		rendered, err = app.renderMarkdown("```unknownlang\nThis is some text.\n```\n")
		require.NoError(t, err)

		assert.Contains(t, string(rendered), "<pre><code class=\"language-unknownlang\">This is some text.\n</code></pre>")

		rendered, err = app.renderMarkdown("```\nThis is a code block without a language.\n```\n")
		require.NoError(t, err)

		assert.Contains(t, string(rendered), "<pre><code>This is a code block without a language.\n</code></pre>")
	})

	t.Run("renderPostMarkdownToWriter", func(t *testing.T) {
		cfg := createDefaultTestConfig(t)
		cfg.Server.PublicAddress = "https://example.com"
		app := &goBlog{cfg: cfg}

		t.Run("matches renderMarkdownToWriter when absoluteLinks=false postPath=\"\"", func(t *testing.T) {
			source := "**Bold** and *italic* with [a link](/path)"
			var buf1, buf2 bytes.Buffer
			err := app.renderMarkdownToWriter(&buf1, source)
			require.NoError(t, err)
			err = app.renderPostMarkdownToWriter(&buf2, source, false, "", false)
			require.NoError(t, err)
			assert.Equal(t, buf1.String(), buf2.String())
		})

		t.Run("absolute link prefixing", func(t *testing.T) {
			var buf bytes.Buffer
			err := app.renderPostMarkdownToWriter(&buf, "[Relative](/relative)", true, "", false)
			require.NoError(t, err)
			output := buf.String()
			assert.Contains(t, output, `href="https://example.com/relative"`)
		})

		t.Run("relative links without absoluteLinks", func(t *testing.T) {
			var buf bytes.Buffer
			err := app.renderPostMarkdownToWriter(&buf, "[Relative](/relative)", false, "", false)
			require.NoError(t, err)
			output := buf.String()
			assert.Contains(t, output, `href="/relative"`)
		})

		t.Run("image rendered through writePictureElement", func(t *testing.T) {
			var buf bytes.Buffer
			err := app.renderPostMarkdownToWriter(&buf, "![Alt](/m/pic.jpg)", false, "", false)
			require.NoError(t, err)
			output := buf.String()
			assert.Contains(t, output, `<a href=`)
			assert.Contains(t, output, `<img`)
			assert.Contains(t, output, `src=`)
			assert.Contains(t, output, `alt="Alt"`)
		})

		t.Run("postPath does not affect output without plugins", func(t *testing.T) {
			source := "Text with ![image](/m/pic.jpg)"
			var buf1, buf2 bytes.Buffer
			err := app.renderPostMarkdownToWriter(&buf1, source, false, "", false)
			require.NoError(t, err)
			err = app.renderPostMarkdownToWriter(&buf2, source, false, "/blog/my-post", false)
			require.NoError(t, err)
			assert.Equal(t, buf1.String(), buf2.String())
		})

		t.Run("handles complex markdown", func(t *testing.T) {
			source := "# Heading\n\nParagraph with **bold** and `code`.\n\n- List item 1\n- List item 2\n\n> Blockquote"
			var buf bytes.Buffer
			err := app.renderPostMarkdownToWriter(&buf, source, false, "", false)
			require.NoError(t, err)
			output := buf.String()
			assert.Contains(t, output, "Heading")
			assert.Contains(t, output, "<strong>")
			assert.Contains(t, output, "<code>")
			assert.Contains(t, output, "<ul>")
			assert.Contains(t, output, "<blockquote>")
		})

		t.Run("options are isolated per render", func(t *testing.T) {
			var wg sync.WaitGroup
			outputs := make([]string, 10)
			errs := make(chan error, 10)
			for i := range 10 {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					var buf bytes.Buffer
					if err := app.renderPostMarkdownToWriter(&buf, "[Relative](/relative)", i%2 == 0, "", false); err != nil {
						errs <- err
						return
					}
					outputs[i] = buf.String()
				}(i)
			}
			wg.Wait()
			close(errs)
			for err := range errs {
				t.Error(err)
			}
			for i, output := range outputs {
				href := "/relative"
				if i%2 == 0 {
					href = "https://example.com/relative"
				}
				assert.Contains(t, output, `href="`+href+`"`)
			}
		})
	})
}

func Benchmark_markdown(b *testing.B) {
	markdownExample, err := os.ReadFile("testdata/markdownexample.md")
	if err != nil {
		b.Errorf("Failed to read markdown example: %v", err)
	}
	mdExp := string(markdownExample)

	app := &goBlog{
		cfg: createDefaultTestConfig(b),
	}

	b.Run("Markdown Rendering", func(b *testing.B) {
		for b.Loop() {
			_, err := app.renderMarkdown(mdExp)
			if err != nil {
				b.Errorf("Error: %v", err)
			}
		}
	})

	b.Run("Title Rendering", func(b *testing.B) {
		for b.Loop() {
			app.renderMdTitle("**Test**")
		}
	})

	b.Run("Text Rendering", func(b *testing.B) {
		for b.Loop() {
			app.renderText("**Test**")
		}
	})
}
