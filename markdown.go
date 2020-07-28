package main

import (
	"bytes"
	_ "bytes"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

var markdown goldmark.Markdown

func init() {
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

func renderMarkdown(source string) (content string, err error) {
	context := parser.NewContext()
	var buffer bytes.Buffer
	err = markdown.Convert([]byte(source), &buffer, parser.WithContext(context))
	content = string(emojify(buffer.Bytes()))
	return
}
