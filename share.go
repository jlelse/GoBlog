package main

import (
	"cmp"
	"net/url"
	"strings"

	"go.goblog.app/app/pkgs/builderpool"
)

type shareService struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	URL   string `json:"url"`
}

type shareData struct {
	Title    string         `json:"title"`
	URL      string         `json:"url"`
	Text     string         `json:"text"`
	Services []shareService `json:"services"`
}

type shareServiceDefinition struct {
	id      string
	label   string
	limit   int
	builder func(title, url, text string, limit int) string
}

var shareServiceDefinitions = []shareServiceDefinition{
	{id: "email", label: "Email", builder: shareEmailURL},
	{id: "mastodon", label: "Mastodon", limit: 500, builder: shareMastodonURL},
	{id: "bluesky", label: "Bluesky", limit: 300, builder: shareBlueskyURL},
	{id: "linkedin", label: "LinkedIn", builder: shareLinkedInURL},
	{id: "microblog", label: "Micro.blog", limit: 300, builder: shareMicroblogURL},
	{id: "reddit", label: "Reddit", limit: 300, builder: shareRedditURL},
	{id: "hackernews", label: "Hacker News", limit: 80, builder: shareHackerNewsURL},
	{id: "sms", label: "SMS", builder: shareSMSURL},
}

func newShareData(title, url string) shareData {
	text := buildShareText(title, url)
	return shareData{
		Title:    title,
		URL:      url,
		Text:     text,
		Services: buildShareServices(title, url, text),
	}
}

func buildShareText(title, url string) string {
	sb := builderpool.Get()
	defer builderpool.Put(sb)
	if title != "" {
		sb.WriteString(title)
		sb.WriteString("\n\n")
	}
	sb.WriteString(url)
	return sb.String()
}

func buildShareServices(title, url, text string) []shareService {
	services := make([]shareService, 0, len(shareServiceDefinitions))
	for _, def := range shareServiceDefinitions {
		services = append(services, shareService{
			ID:    def.id,
			Label: def.label,
			URL:   def.builder(title, url, text, def.limit),
		})
	}
	return services
}

var mailtoLineBreaks = strings.NewReplacer("\r\n", "\r\n", "\n", "\r\n", "\r", "\r\n")

func shareEmailURL(title, url, text string, _ int) string {
	subject := cmp.Or(strings.Join(strings.Fields(title), " "), url)
	body := mailtoLineBreaks.Replace(cmp.Or(text, url))
	return "mailto:?subject=" + queryEscape(subject) + "&body=" + queryEscape(body)
}

func shareMastodonURL(_, url, text string, limit int) string {
	payload := limitSharePayload(text, url, limit)
	return "https://share.joinmastodon.org/#text=" + queryEscape(payload)
}

func shareBlueskyURL(_, url, text string, limit int) string {
	payload := limitSharePayload(text, url, limit)
	return "https://bsky.app/intent/compose?text=" + queryEscape(payload)
}

func shareRedditURL(title, url, _ string, limit int) string {
	title = cmp.Or(strings.Join(strings.Fields(title), " "), url)
	return "https://www.reddit.com/submit?type=link&url=" + queryEscape(url) + "&title=" + queryEscape(truncateStringWithEllipsis(title, limit))
}

func shareHackerNewsURL(title, url, _ string, limit int) string {
	title = strings.Join(strings.Fields(title), " ")
	return "https://news.ycombinator.com/submitlink?u=" + queryEscape(url) + "&t=" + queryEscape(truncateStringWithEllipsis(title, limit))
}

func shareLinkedInURL(_, url, _ string, _ int) string {
	return "https://www.linkedin.com/sharing/share-offsite/?url=" + queryEscape(url)
}

func shareMicroblogURL(_, url, text string, limit int) string {
	payload := limitSharePayload(text, url, limit)
	return "https://micro.blog/post?bookmarklet=true&text=" + queryEscape(payload)
}

func shareSMSURL(_, url, _ string, _ int) string {
	return "sms:?body=" + queryEscape(url)
}

func limitSharePayload(text, url string, limit int) string {
	payload := cmp.Or(text, url)
	if limit <= 0 || len([]rune(payload)) <= limit {
		return payload
	}
	prefix := strings.TrimRight(strings.TrimSuffix(payload, url), "\n")
	const (
		ellipsis     = "…"
		separator    = "\n\n"
		minTextRunes = 10
	)
	allowed := limit - len([]rune(url)) - len([]rune(separator))
	if allowed < minTextRunes+len([]rune(ellipsis)) {
		return url
	}
	trimmedText := limitRunes(prefix, allowed-len([]rune(ellipsis)))
	if trimmedText == "" {
		return url
	}
	return trimmedText + ellipsis + separator + url
}

func limitRunes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

func queryEscape(value string) string {
	return strings.ReplaceAll(url.QueryEscape(value), "+", "%20")
}
