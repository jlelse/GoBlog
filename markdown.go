package main

import (
	"bytes"
	kemoji "github.com/kyokomi/emoji"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark-emoji"
	"github.com/yuin/goldmark-emoji/definition"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"strings"
	"sync"
)

var emojilib definition.Emojis
var emojiOnce sync.Once

var markdown goldmark.Markdown

func initMarkdown() {
	markdown = goldmark.New(
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithExtensions(
			extension.GFM,
			extension.Footnote,
			extension.Typographer,
			// Emojis
			emoji.New(
				emoji.WithEmojis(EmojiGoLib()),
			),
		),
	)
}

func renderMarkdown(source string) (content []byte, err error) {
	var buffer bytes.Buffer
	err = markdown.Convert([]byte(source), &buffer)
	content = buffer.Bytes()
	return
}

func EmojiGoLib() definition.Emojis {
	emojiOnce.Do(func() {
		var emojis []definition.Emoji
		for shotcode, e := range kemoji.CodeMap() {
			emojis = append(emojis, definition.NewEmoji(e, []rune(e), strings.ReplaceAll(shotcode, ":", "")))
		}
		emojilib = definition.NewEmojis(emojis...)
	})
	return emojilib
}
