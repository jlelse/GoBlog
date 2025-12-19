// Package jsonld provides JSON-LD context handling for ActivityPub objects
package jsonld

import (
	"encoding/json"
)

// IRI represents an IRI in JSON-LD context
type IRI string

// ContextBuilder helps build JSON-LD contexts
type ContextBuilder struct {
	contexts []interface{}
}

// WithContext creates a new ContextBuilder with the given contexts
func WithContext(contexts ...IRI) *ContextBuilder {
	cb := &ContextBuilder{
		contexts: make([]interface{}, len(contexts)),
	}
	for i, c := range contexts {
		cb.contexts[i] = string(c)
	}
	return cb
}

// Marshal marshals an object with the configured contexts
func (cb *ContextBuilder) Marshal(obj interface{}) ([]byte, error) {
	// Create a wrapper with @context
	wrapper := map[string]interface{}{
		"@context": cb.contexts,
	}
	
	// Marshal the object first to get its JSON representation
	objData, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	
	// Unmarshal into a map
	var objMap map[string]interface{}
	if err := json.Unmarshal(objData, &objMap); err != nil {
		return nil, err
	}
	
	// Remove any existing @context from the object
	delete(objMap, "@context")
	
	// Merge the object fields into the wrapper
	for k, v := range objMap {
		wrapper[k] = v
	}
	
	// Marshal the final result
	return json.Marshal(wrapper)
}
