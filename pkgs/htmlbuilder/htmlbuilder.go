package htmlbuilder

import (
	"fmt"
	"io"
	textTemplate "text/template"
)

type HtmlBuilder struct {
	w io.Writer
}

func NewHtmlBuilder(w io.Writer) *HtmlBuilder {
	return &HtmlBuilder{
		w: w,
	}
}

func (h *HtmlBuilder) getWriter() io.Writer {
	return h.w
}

func (h *HtmlBuilder) Write(p []byte) (int, error) {
	return h.getWriter().Write(p)
}

func (h *HtmlBuilder) WriteString(s string) (int, error) {
	return io.WriteString(h.getWriter(), s)
}

func (h *HtmlBuilder) WriteUnescaped(s string) {
	_, _ = h.WriteString(s)
}

func (h *HtmlBuilder) WriteEscaped(s string) {
	textTemplate.HTMLEscape(h, []byte(s))
}

func (h *HtmlBuilder) WriteAttribute(attr string, val any) {
	h.WriteUnescaped(` `)
	h.WriteUnescaped(attr)
	h.WriteUnescaped(`=`)
	if valStr, ok := val.(string); ok {
		h.WriteUnescaped(`"`)
		h.WriteEscaped(valStr)
		h.WriteUnescaped(`"`)
	} else {
		h.WriteEscaped(fmt.Sprint(val))
	}
}

func (h *HtmlBuilder) WriteElementOpen(tag string, attrs ...any) {
	h.WriteUnescaped(`<`)
	h.WriteUnescaped(tag)
	for i := 0; i < len(attrs); i += 2 {
		if i+2 > len(attrs) {
			break
		}
		attrStr, ok := attrs[i].(string)
		if !ok {
			continue
		}
		h.WriteAttribute(attrStr, attrs[i+1])
	}
	h.WriteUnescaped(`>`)
}

func (h *HtmlBuilder) WriteElementsOpen(tags ...string) {
	for _, tag := range tags {
		h.WriteElementOpen(tag)
	}
}

func (h *HtmlBuilder) WriteElementClose(tag string) {
	h.WriteUnescaped(`</`)
	h.WriteUnescaped(tag)
	h.WriteUnescaped(`>`)
}

func (h *HtmlBuilder) WriteElementsClose(tags ...string) {
	for _, tag := range tags {
		h.WriteElementClose(tag)
	}
}
