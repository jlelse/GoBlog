package main

import (
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
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
	const (
		title      = "C++\r\n& café #1"
		singleLine = "C++ & café #1"
		permalink  = "https://example.com/posts/c%2B%2B?q=a+b&lang=de#résumé"
		text       = title + "\n\n" + permalink
	)
	want := map[string]struct {
		base   string
		params url.Values
	}{
		"email":      {"mailto:", url.Values{"subject": {singleLine}, "body": {title + "\r\n\r\n" + permalink}}},
		"mastodon":   {"https://share.joinmastodon.org/", url.Values{"text": {text}}},
		"bluesky":    {"https://bsky.app/intent/compose", url.Values{"text": {text}}},
		"linkedin":   {"https://www.linkedin.com/sharing/share-offsite/", url.Values{"url": {permalink}}},
		"microblog":  {"https://micro.blog/post", url.Values{"bookmarklet": {"true"}, "text": {text}}},
		"reddit":     {"https://www.reddit.com/submit", url.Values{"type": {"link"}, "url": {permalink}, "title": {singleLine}}},
		"hackernews": {"https://news.ycombinator.com/submitlink", url.Values{"u": {permalink}, "t": {singleLine}}},
		"sms":        {"sms:", url.Values{"body": {permalink}}},
	}
	for _, svc := range newShareData(title, permalink).Services {
		t.Run(svc.ID, func(t *testing.T) {
			expect, ok := want[svc.ID]
			require.True(t, ok, "unknown or duplicate service")
			delete(want, svc.ID)
			link, err := url.Parse(svc.URL)
			require.NoError(t, err)
			params := link.RawQuery
			if svc.ID == "mastodon" {
				require.Empty(t, params)
				params = link.EscapedFragment()
			} else {
				require.Empty(t, link.Fragment)
			}
			require.NotContains(t, params, "+")
			values, err := url.ParseQuery(params)
			require.NoError(t, err)
			require.Equal(t, expect.params, values)
			link.RawQuery, link.Fragment, link.RawFragment = "", "", ""
			require.Equal(t, expect.base, link.String())
		})
	}
	require.Empty(t, want, "missing services")
}

func TestRedditShareTitleLimit(t *testing.T) {
	const permalink = "https://example.com/post"
	for _, svc := range newShareData(strings.Repeat("界", 301), permalink).Services {
		if svc.ID == "reddit" {
			link, err := url.Parse(svc.URL)
			require.NoError(t, err)
			title := link.Query().Get("title")
			require.LessOrEqual(t, len([]rune(title)), 300)
			require.True(t, strings.HasSuffix(title, "…"))
			require.Equal(t, permalink, link.Query().Get("url"))
			return
		}
	}
	t.Fatal("missing Reddit share link")
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
