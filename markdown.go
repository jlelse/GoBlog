package main

import (
	"bytes"
	"github.com/kyokomi/emoji"
	"github.com/yuin/goldmark"
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
		),
	)
}

func renderMarkdown(source string) (content []byte, err error) {
	var buffer bytes.Buffer
	err = markdown.Convert([]byte(emoji.Sprint(source)), &buffer)
	content = buffer.Bytes()
	return
}
