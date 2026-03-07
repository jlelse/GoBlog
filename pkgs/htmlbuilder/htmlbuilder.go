// Package htmlbuilder provides an HTML builder for constructing HTML content.
package htmlbuilder

import (
	"fmt"
	"io"
	textTemplate "text/template"
)

// HtmlBuilder builds HTML content.
type HtmlBuilder struct {
	w io.Writer
}

// NewHtmlBuilder creates a new HtmlBuilder that writes to the given writer.
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

// WriteString writes a raw string.
func (h *HtmlBuilder) WriteString(s string) (int, error) {
	return io.WriteString(h.getWriter(), s)
}

// WriteUnescaped writes an unescaped string.
func (h *HtmlBuilder) WriteUnescaped(s string) {
	_, _ = h.WriteString(s)
}

// WriteEscaped writes an HTML-escaped string.
func (h *HtmlBuilder) WriteEscaped(s string) {
	textTemplate.HTMLEscape(h, []byte(s))
}

// WriteAttribute writes an HTML attribute key-value pair.
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

// WriteElementOpen writes an opening HTML element tag with optional attributes.
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

// WriteElementsOpen writes multiple opening HTML element tags.
func (h *HtmlBuilder) WriteElementsOpen(tags ...string) {
	for _, tag := range tags {
		h.WriteElementOpen(tag)
	}
}

// WriteElementClose writes a closing HTML element tag.
func (h *HtmlBuilder) WriteElementClose(tag string) {
	h.WriteUnescaped(`</`)
	h.WriteUnescaped(tag)
	h.WriteUnescaped(`>`)
}

// WriteElementsClose writes multiple closing HTML element tags.
func (h *HtmlBuilder) WriteElementsClose(tags ...string) {
	for _, tag := range tags {
		h.WriteElementClose(tag)
	}
}

// WriteElement writes a complete HTML element with content.
func (h *HtmlBuilder) WriteElement(tag string, attrs ...any) {
	h.WriteElementOpen(tag, attrs...)
	h.WriteElementClose(tag)
}
