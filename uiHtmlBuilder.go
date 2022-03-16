package main

import (
	"fmt"
	"io"
	textTemplate "text/template"
)

type htmlBuilder struct {
	w io.Writer
}

func newHtmlBuilder(w io.Writer) *htmlBuilder {
	return &htmlBuilder{
		w: w,
	}
}

func (h *htmlBuilder) getWriter() io.Writer {
	return h.w
}

func (h *htmlBuilder) Write(p []byte) (int, error) {
	return h.getWriter().Write(p)
}

func (h *htmlBuilder) WriteString(s string) (int, error) {
	return io.WriteString(h.getWriter(), s)
}

func (h *htmlBuilder) write(s string) {
	_, _ = h.WriteString(s)
}

func (h *htmlBuilder) writeEscaped(s string) {
	textTemplate.HTMLEscape(h, []byte(s))
}

func (h *htmlBuilder) writeAttribute(attr string, val any) {
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

func (h *htmlBuilder) writeElementOpen(tag string, attrs ...any) {
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
