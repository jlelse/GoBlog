// Package htmlbuilder provides an HTML builder for constructing HTML content.
package htmlbuilder

import (
	"fmt"
	"io"
	textTemplate "text/template"
)

// HTMLBuilder builds HTML content.
type HTMLBuilder struct {
	w io.Writer
}

// NewHTMLBuilder creates a new HTMLBuilder that writes to the given writer.
func NewHTMLBuilder(w io.Writer) *HTMLBuilder {
	return &HTMLBuilder{
		w: w,
	}
}

func (h *HTMLBuilder) getWriter() io.Writer {
	return h.w
}

func (h *HTMLBuilder) Write(p []byte) (int, error) {
	return h.getWriter().Write(p)
}

// WriteString writes a raw string.
func (h *HTMLBuilder) WriteString(s string) (int, error) {
	return io.WriteString(h.getWriter(), s)
}

// WriteUnescaped writes an unescaped string.
func (h *HTMLBuilder) WriteUnescaped(s string) {
	_, _ = h.WriteString(s)
}

// WriteEscaped writes an HTML-escaped string.
func (h *HTMLBuilder) WriteEscaped(s string) {
	textTemplate.HTMLEscape(h, []byte(s))
}

// WriteAttribute writes an HTML attribute key-value pair.
func (h *HTMLBuilder) WriteAttribute(attr string, val any) {
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
func (h *HTMLBuilder) WriteElementOpen(tag string, attrs ...any) {
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
func (h *HTMLBuilder) WriteElementsOpen(tags ...string) {
	for _, tag := range tags {
		h.WriteElementOpen(tag)
	}
}

// WriteElementClose writes a closing HTML element tag.
func (h *HTMLBuilder) WriteElementClose(tag string) {
	h.WriteUnescaped(`</`)
	h.WriteUnescaped(tag)
	h.WriteUnescaped(`>`)
}

// WriteElementsClose writes multiple closing HTML element tags.
func (h *HTMLBuilder) WriteElementsClose(tags ...string) {
	for _, tag := range tags {
		h.WriteElementClose(tag)
	}
}

// WriteElement writes a complete HTML element with content.
func (h *HTMLBuilder) WriteElement(tag string, attrs ...any) {
	h.WriteElementOpen(tag, attrs...)
	h.WriteElementClose(tag)
}

// Deprecated: HtmlBuilder is an alias for HTMLBuilder for backward compatibility.
//
//revive:disable:var-naming
//revive:disable:exported
type HtmlBuilder = HTMLBuilder

// Deprecated: NewHtmlBuilder is an alias for NewHTMLBuilder for backward compatibility.
var NewHtmlBuilder = NewHTMLBuilder

//revive:enable:var-naming
//revive:enable:exported
