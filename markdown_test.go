package main

import (
	"os"
	"strings"
	"testing"
)

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
		if err != nil {
			t.Fatalf("Error: %v", err)
		}
		if !strings.Contains(string(rendered), `href="/relative"`) {
			t.Errorf("Wrong result, got %v", string(rendered))
		}

		rendered, err = app.renderMarkdown("[Relative](/relative)", true)
		if err != nil {
			t.Fatalf("Error: %v", err)
		}
		if !strings.Contains(string(rendered), `href="https://example.com/relative"`) {
			t.Errorf("Wrong result, got %v", string(rendered))
		}
		if strings.Contains(string(rendered), `target="_blank"`) {
			t.Errorf("Wrong result, got %v", string(rendered))
		}

		// External links

		rendered, err = app.renderMarkdown("[External](https://example.com)", true)
		if err != nil {
			t.Fatalf("Error: %v", err)
		}
		if !strings.Contains(string(rendered), `target="_blank"`) {
			t.Errorf("Wrong result, got %v", string(rendered))
		}

		// Link title

		rendered, err = app.renderMarkdown(`[With title](https://example.com "Test-Title")`, true)
		if err != nil {
			t.Fatalf("Error: %v", err)
		}
		if !strings.Contains(string(rendered), `title="Test-Title"`) {
			t.Errorf("Wrong result, got %v", string(rendered))
		}

		// Text

		renderedText := app.renderText("**This** *is* [text](/)")
		if renderedText != "This is text" {
			t.Errorf("Wrong result, got \"%v\"", renderedText)
		}
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

	b.Run("Benchmark Markdown Rendering", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := app.renderMarkdown(mdExp, true)
			if err != nil {
				b.Errorf("Error: %v", err)
			}
		}
	})
}
