package main

import (
	"bytes"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark-emoji"
	"github.com/yuin/goldmark-emoji/definition"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

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
				emoji.WithEmojis(definition.Github()),
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
