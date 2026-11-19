// Package htmlbuilder provides an HTML builder for constructing HTML content.
package htmlbuilder

import (
	"fmt"
	"io"
)

// HTMLBuilder builds HTML content.
type HTMLBuilder struct {
	w  io.Writer
	sw io.StringWriter
}

// NewHTMLBuilder creates a new HTMLBuilder that writes to the given writer.
func NewHTMLBuilder(w io.Writer) *HTMLBuilder {
	sw, _ := w.(io.StringWriter)
	return &HTMLBuilder{
		w:  w,
		sw: sw,
	}
}

func (h *HTMLBuilder) Write(p []byte) (int, error) {
	return h.w.Write(p)
}

// WriteString writes a raw string.
func (h *HTMLBuilder) WriteString(s string) (int, error) {
	if h.sw != nil {
		return h.sw.WriteString(s)
	}
	return io.WriteString(h.w, s)
}

// WriteUnescaped writes an unescaped string.
func (h *HTMLBuilder) WriteUnescaped(s string) {
	_, _ = h.WriteString(s)
}

// WriteEscaped writes an HTML-escaped string. It escapes the same characters
// as text/template.HTMLEscape, but without allocating.
func (h *HTMLBuilder) WriteEscaped(s string) {
	last := 0
	for i := range len(s) {
		var esc string
		switch s[i] {
		case '\x00':
			esc = "\uFFFD"
		case '"':
			esc = "&#34;"
		case '\'':
			esc = "&#39;"
		case '&':
			esc = "&amp;"
		case '<':
			esc = "&lt;"
		case '>':
			esc = "&gt;"
		default:
			continue
		}
		if last < i {
			h.WriteUnescaped(s[last:i])
		}
		h.WriteUnescaped(esc)
		last = i + 1
	}
	if last < len(s) {
		h.WriteUnescaped(s[last:])
	}
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
		return
	}
	h.WriteEscaped(fmt.Sprint(val))
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
