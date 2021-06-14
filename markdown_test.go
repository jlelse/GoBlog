package main

import (
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
			t.Errorf("Error: %v", err)
		}
		if !strings.Contains(string(rendered), `href="/relative"`) {
			t.Errorf("Wrong result, got %v", string(rendered))
		}

		rendered, err = app.renderMarkdown("[Relative](/relative)", true)
		if err != nil {
			t.Errorf("Error: %v", err)
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
			t.Errorf("Error: %v", err)
		}
		if !strings.Contains(string(rendered), `target="_blank"`) {
			t.Errorf("Wrong result, got %v", string(rendered))
		}

		// Link title

		rendered, err = app.renderMarkdown(`[With title](https://example.com "Test-Title")`, true)
		if err != nil {
			t.Errorf("Error: %v", err)
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
