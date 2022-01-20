package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	textTemplate "text/template"
)

type htmlBuilder struct {
	w   io.Writer
	buf bytes.Buffer
}

func newHtmlBuilder(w io.Writer) *htmlBuilder {
	return &htmlBuilder{
		w: w,
	}
}

func (h *htmlBuilder) getWriter() io.Writer {
	if h.w != nil {
		return h.w
	}
	return &h.buf
}

func (h *htmlBuilder) Write(p []byte) (int, error) {
	return h.getWriter().Write(p)
}

func (h *htmlBuilder) WriteString(s string) (int, error) {
	return io.WriteString(h.getWriter(), s)
}

func (h *htmlBuilder) Read(p []byte) (int, error) {
	return h.buf.Read(p)
}

func (h *htmlBuilder) String() string {
	return h.buf.String()
}

func (h *htmlBuilder) Bytes() []byte {
	return h.buf.Bytes()
}

func (h *htmlBuilder) html() template.HTML {
	return template.HTML(h.String())
}

func (h *htmlBuilder) write(s string) {
	_, _ = h.WriteString(s)
}

func (h *htmlBuilder) writeHtml(s template.HTML) {
	h.write(string(s))
}

func (h *htmlBuilder) writeEscaped(s string) {
	textTemplate.HTMLEscape(h, []byte(s))
}

func (h *htmlBuilder) writeAttribute(attr string, val interface{}) {
	h.write(` `)
	h.write(attr)
	h.write(`=`)
	if valStr, ok := val.(string); ok {
		h.write(`"`)
		h.writeEscaped(valStr)
		h.write(`"`)
	} else {
		h.writeEscaped(fmt.Sprint(val))
	}
}

func (h *htmlBuilder) writeElementOpen(tag string, attrs ...interface{}) {
	h.write(`<`)
	h.write(tag)
	for i := 0; i < len(attrs); i += 2 {
		if i+2 > len(attrs) {
			break
		}
		attrStr, ok := attrs[i].(string)
		if !ok {
			continue
		}
		h.writeAttribute(attrStr, attrs[i+1])
	}
	h.write(`>`)
}

func (h *htmlBuilder) writeElementClose(tag string) {
	h.write(`</`)
	h.write(tag)
	h.write(`>`)
}
