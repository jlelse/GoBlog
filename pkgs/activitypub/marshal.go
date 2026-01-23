package activitypub

import (
	"bytes"
	"encoding/json"

	"go.goblog.app/app/pkgs/bufferpool"
)

// MarshalJSON implements json.Marshaler
func (i IRI) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(i))
}

// MarshalJSON implements json.Marshaler for NaturalLanguageValues
func (n NaturalLanguageValues) MarshalJSON() ([]byte, error) {
	if len(n) == 0 {
		return []byte("null"), nil
	}
	// GoBlog uses only single values for now with a default language
	buf := bufferpool.Get()
	defer bufferpool.Put(buf)
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(n[0].Value); err != nil {
		return nil, err
	}
	// Remove trailing newline
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

// MarshalJSON implements json.Marshaler for ItemCollection
func (i ItemCollection) MarshalJSON() ([]byte, error) {
	if len(i) == 0 {
		return []byte("null"), nil
	}
	// Always marshal as array for consistency
	buf := bufferpool.Get()
	defer bufferpool.Put(buf)
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	arr := make([]Item, len(i))
	copy(arr, i)
	if err := enc.Encode(arr); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

// MarshalJSON implements json.Marshaler for PublicKey
func (p PublicKey) MarshalJSON() ([]byte, error) {
	type Alias PublicKey
	return json.Marshal(Alias(p))
}

// MarshalJSON implements json.Marshaler for Endpoints
func (e Endpoints) MarshalJSON() ([]byte, error) {
	m := make(map[string]any)
	if e.SharedInbox != nil {
		m["sharedInbox"] = e.SharedInbox
	}
	return json.Marshal(m)
}
