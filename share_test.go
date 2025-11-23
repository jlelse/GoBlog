package main

import (
	"strings"
	"testing"
)

func TestBuildShareText(t *testing.T) {
	tests := []struct {
		name  string
		title string
		url   string
		want  string
	}{
		{
			name:  "title and url",
			title: "Hello",
			url:   "https://example.com",
			want:  "Hello\n\nhttps://example.com",
		},
		{
			name:  "only url",
			title: "",
			url:   "https://example.com",
			want:  "https://example.com",
		},
		{
			name:  "empty",
			title: "",
			url:   "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildShareText(tt.title, tt.url); got != tt.want {
				t.Fatalf("buildShareText(%q, %q) = %q, want %q", tt.title, tt.url, got, tt.want)
			}
		})
	}
}

func TestBuildShareServices(t *testing.T) {
	title := "Test Title"
	url := "https://example.com"
	text := buildShareText(title, url)

	services := buildShareServices(title, url, text)
	if len(services) != len(shareServiceDefinitions) {
		t.Fatalf("unexpected services count: got %d, want %d", len(services), len(shareServiceDefinitions))
	}

	expect := map[string]string{
		"email":      "mailto:?subject=Test+Title&body=Test+Title%0A%0Ahttps%3A%2F%2Fexample.com",
		"mastodon":   "https://mastodon.social/share?text=Test+Title%0A%0Ahttps%3A%2F%2Fexample.com",
		"bluesky":    "https://bsky.app/intent/compose?text=Test+Title%0A%0Ahttps%3A%2F%2Fexample.com",
		"linkedin":   "https://www.linkedin.com/shareArticle?mini=true&url=https%3A%2F%2Fexample.com&title=Test+Title&summary=Test+Title%0A%0Ahttps%3A%2F%2Fexample.com",
		"microblog":  "https://micro.blog/post?text=Test+Title%0A%0Ahttps%3A%2F%2Fexample.com",
		"reddit":     "https://www.reddit.com/submit?url=https%3A%2F%2Fexample.com&title=Test+Title",
		"hackernews": "https://news.ycombinator.com/submitlink?u=https%3A%2F%2Fexample.com&t=Test+Title",
		"sms":        "sms:?body=https%3A%2F%2Fexample.com",
	}

	for _, svc := range services {
		want, ok := expect[svc.ID]
		if !ok {
			t.Fatalf("unexpected service id %q", svc.ID)
		}
		if svc.URL != want {
			t.Fatalf("service %q url = %q, want %q", svc.ID, svc.URL, want)
		}
	}
}

func TestLimitSharePayload(t *testing.T) {
	limit := 120
	title := strings.Repeat("A", 200)
	url := "https://example.com"
	text := buildShareText(title, url)

	payload := limitSharePayload(text, url, limit)
	if len([]rune(payload)) > limit {
		t.Fatalf("limitSharePayload length = %d, want <= %d", len([]rune(payload)), limit)
	}
	if !strings.HasSuffix(payload, url) {
		t.Fatalf("expected payload to end with url, got %q", payload)
	}
	if !strings.Contains(payload, "…\n\n"+url) {
		t.Fatalf("expected payload to include an ellipsis before the separator, got %q", payload)
	}

	prefix := strings.TrimSuffix(payload, "\n\n"+url)
	if !strings.HasSuffix(prefix, "…") {
		t.Fatalf("expected preview to end with ellipsis, got %q", prefix)
	}
	textPortion := strings.TrimSuffix(prefix, "…")
	allowed := limit - len([]rune(url)) - len([]rune("\n\n")) - len([]rune("…"))
	if len([]rune(textPortion)) > allowed {
		t.Fatalf("text portion length = %d, want <= %d", len([]rune(textPortion)), allowed)
	}
}

func TestLimitSharePayloadKeepsURL(t *testing.T) {
	url := "https://example.com"
	text := buildShareText("Hello", url)
	smallLimit := len([]rune(url)) + len([]rune("\n\n")) + 3
	payload := limitSharePayload(text, url, smallLimit)
	if payload != url {
		t.Fatalf("expected URL to remain intact even when the limit is too tight, got %q", payload)
	}
}
