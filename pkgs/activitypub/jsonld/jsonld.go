// Package jsonld provides JSON-LD context handling for ActivityPub objects
package jsonld

import (
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
	if len(objData) > 0 && objData[len(objData)-1] == '\n' {
		objData = objData[:len(objData)-1]
	}

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
	if len(ctxData) > 0 && ctxData[len(ctxData)-1] == '\n' {
		ctxData = ctxData[:len(ctxData)-1]
	}

	// Build the output manually to preserve field order
	// Start with {"@context": and the marshaled context
	result := []byte(`{"@context":`)
	result = append(result, ctxData...)

	// Remove the opening brace from objData if present
	objDataStr := string(objData)
	if len(objDataStr) > 0 && objDataStr[0] == '{' {
		objDataStr = objDataStr[1:]
	}

	// Append a comma and the rest of the object
	if len(objDataStr) > 0 && objDataStr[0] != '}' {
		result = append(result, ',')
	}
	result = append(result, []byte(objDataStr)...)

	return result, nil
}
