// Package jsonld provides JSON-LD context handling for ActivityPub objects
package jsonld

import (
	"bytes"
	"encoding/json"

	"go.goblog.app/app/pkgs/bufferpool"
)

// IRI represents an IRI in JSON-LD context
type IRI string

// ContextBuilder helps build JSON-LD contexts
type ContextBuilder struct {
	contexts []any
}

// WithContext creates a new ContextBuilder with the given contexts
func WithContext(contexts ...IRI) *ContextBuilder {
	cb := &ContextBuilder{
		contexts: make([]any, len(contexts)),
	}
	for i, c := range contexts {
		cb.contexts[i] = string(c)
	}
	return cb
}

// Marshal marshals an object with the configured contexts
func (cb *ContextBuilder) Marshal(obj any) ([]byte, error) {
	// Marshal the object using an encoder with HTML escaping disabled
	objBuf := bufferpool.Get()
	defer bufferpool.Put(objBuf)
	objEnc := json.NewEncoder(objBuf)
	objEnc.SetEscapeHTML(false)
	if err := objEnc.Encode(obj); err != nil {
		return nil, err
	}
	objData := objBuf.Bytes()
	// Remove trailing newline added by Encoder
	objData = bytes.TrimSuffix(objData, []byte("\n"))
	// Remove the opening brace if present
	objData = bytes.TrimPrefix(objData, []byte("{"))

	// Marshal the context using an encoder with HTML escaping disabled
	ctxBuf := bufferpool.Get()
	defer bufferpool.Put(ctxBuf)
	ctxEnc := json.NewEncoder(ctxBuf)
	ctxEnc.SetEscapeHTML(false)
	if err := ctxEnc.Encode(cb.contexts); err != nil {
		return nil, err
	}
	ctxData := ctxBuf.Bytes()
	// Remove trailing newline
	ctxData = bytes.TrimSuffix(ctxData, []byte("\n"))

	// Build the output manually to preserve field order
	// Start with {"@context": and the marshaled context
	base := []byte(`{"@context":`)
	result := make([]byte, 0, len(base)+len(ctxData)+len(objData)+1) // +1 for possible comma
	result = append(result, base...)
	result = append(result, ctxData...)

	// Append a comma and the rest of the object
	if len(objData) > 0 && objData[0] != '}' {
		result = append(result, ',')
	}
	result = append(result, objData...)

	return result, nil
}
