// Package activitypub provides types and utilities for working with ActivityPub objects.
// This is a minimal internal implementation focused on GoBlog's specific needs.
package activitypub

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"go.goblog.app/app/pkgs/bufferpool"
)

// ActivityPub namespaces
const (
	ActivityBaseURI    = "https://www.w3.org/ns/activitystreams"
	SecurityContextURI = "https://w3id.org/security/v1"
	PublicNS           = IRI("https://www.w3.org/ns/activitystreams#Public")
)

// ActivityType represents the type of an ActivityPub object
type ActivityType string

// Common ActivityPub types
const (
	ObjectType      ActivityType = "Object"
	ActivityObjType ActivityType = "Activity"
	NoteType        ActivityType = "Note"
	ArticleType     ActivityType = "Article"
	PersonType      ActivityType = "Person"
	ImageType       ActivityType = "Image"
	MentionType     ActivityType = "Mention"
	CollectionType  ActivityType = "Collection"

	// Activity types
	CreateType   ActivityType = "Create"
	UpdateType   ActivityType = "Update"
	DeleteType   ActivityType = "Delete"
	FollowType   ActivityType = "Follow"
	AcceptType   ActivityType = "Accept"
	UndoType     ActivityType = "Undo"
	AnnounceType ActivityType = "Announce"
	LikeType     ActivityType = "Like"
	BlockType    ActivityType = "Block"
)

// IRI represents an Internationalized Resource Identifier
type IRI string

// String returns the IRI as a string
func (i IRI) String() string {
	return string(i)
}

// URL parses the IRI as a URL
func (i IRI) URL() (*url.URL, error) {
	return url.Parse(string(i))
}

// GetLink returns the IRI itself (satisfies LinkOrObject interface)
func (i IRI) GetLink() IRI {
	return i
}

// GetType returns empty string for IRI
func (i IRI) GetType() ActivityType {
	return ""
}

// IsLink returns true for IRI
func (i IRI) IsLink() bool {
	return true
}

// IsObject returns false for IRI
func (i IRI) IsObject() bool {
	return false
}

// MarshalJSON implements json.Marshaler
func (i IRI) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(i))
}

// ID represents an ActivityPub ID (alias for IRI)
type ID IRI

// String returns the ID as a string
func (id ID) String() string {
	return string(id)
}

// MimeType represents a MIME type
type MimeType string

// NaturalLanguageValue represents a value in a natural language
type NaturalLanguageValue struct {
	Value string
	Lang  string
}

// NaturalLanguageValues is a collection of language-tagged values
type NaturalLanguageValues []NaturalLanguageValue

// Set sets a value for a language
func (n *NaturalLanguageValues) Set(lang string, value string) {
	if n == nil {
		n = &NaturalLanguageValues{}
	}
	// Check if we need to replace an existing value
	for i, v := range *n {
		if v.Lang == lang {
			(*n)[i].Value = value
			return
		}
	}
	// Add new value
	*n = append(*n, NaturalLanguageValue{Lang: lang, Value: value})
}

// First returns the first value
func (n NaturalLanguageValues) First() NaturalLanguageValue {
	if len(n) > 0 {
		return n[0]
	}
	return NaturalLanguageValue{}
}

// String returns the value as string
func (n NaturalLanguageValue) String() string {
	return n.Value
}

// MarshalJSON implements json.Marshaler for NaturalLanguageValues
func (n NaturalLanguageValues) MarshalJSON() ([]byte, error) {
	if len(n) == 0 {
		return []byte("null"), nil
	}
	if len(n) == 1 && n[0].Lang == "" {
		// Single value without language tag - use no HTML escaping
		buf := bufferpool.Get()
		defer bufferpool.Put(buf)
		enc := json.NewEncoder(buf)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(n[0].Value); err != nil {
			return nil, err
		}
		// Remove trailing newline
		result := buf.Bytes()
		if len(result) > 0 && result[len(result)-1] == '\n' {
			result = result[:len(result)-1]
		}
		return result, nil
	}
	// Multiple values or language tagged
	m := make(map[string]string)
	for _, v := range n {
		lang := v.Lang
		if lang == "" {
			lang = "@value"
		}
		m[lang] = v.Value
	}
	buf := bufferpool.Get()
	defer bufferpool.Put(buf)
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(m); err != nil {
		return nil, err
	}
	// Remove trailing newline
	result := buf.Bytes()
	if len(result) > 0 && result[len(result)-1] == '\n' {
		result = result[:len(result)-1]
	}
	return result, nil
}

// UnmarshalJSON implements json.Unmarshaler for NaturalLanguageValues
func (n *NaturalLanguageValues) UnmarshalJSON(data []byte) error {
	// Try as string first
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*n = NaturalLanguageValues{{Value: s, Lang: ""}}
		return nil
	}

	// Try as map
	var m map[string]string
	if err := json.Unmarshal(data, &m); err == nil {
		*n = make(NaturalLanguageValues, 0, len(m))
		for lang, value := range m {
			if lang == "@value" {
				lang = ""
			}
			*n = append(*n, NaturalLanguageValue{Lang: lang, Value: value})
		}
		return nil
	}

	return fmt.Errorf("invalid natural language value")
}

// DefaultNaturalLanguage creates a NaturalLanguageValues with a single value
func DefaultNaturalLanguage(value string) NaturalLanguageValues {
	return NaturalLanguageValues{{Value: value, Lang: ""}}
}

// Content represents content
type Content string

const DefaultLang = ""

// Item represents an ActivityPub item (can be an IRI or an Object)
type Item interface {
	GetLink() IRI
	GetType() ActivityType
	IsLink() bool
	IsObject() bool
}

// ItemCollection is a collection of items
type ItemCollection []Item

// Append adds items to the collection
func (i *ItemCollection) Append(items ...Item) {
	*i = append(*i, items...)
}

// Contains checks if the collection contains an item
func (i ItemCollection) Contains(item Item) bool {
	for _, it := range i {
		if it.GetLink() == item.GetLink() {
			return true
		}
	}
	return false
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
	// Remove trailing newline
	result := buf.Bytes()
	if len(result) > 0 && result[len(result)-1] == '\n' {
		result = result[:len(result)-1]
	}
	return result, nil
}

// UnmarshalJSON implements json.Unmarshaler for ItemCollection
func (i *ItemCollection) UnmarshalJSON(data []byte) error {
	// Try as array first
	var arr []json.RawMessage
	if err := json.Unmarshal(data, &arr); err == nil {
		*i = make(ItemCollection, 0, len(arr))
		for _, raw := range arr {
			item, err := unmarshalItem(raw)
			if err != nil {
				return err
			}
			*i = append(*i, item)
		}
		return nil
	}

	// Try as single item
	item, err := unmarshalItem(data)
	if err != nil {
		return err
	}
	*i = ItemCollection{item}
	return nil
}

// unmarshalItem unmarshals an item as either IRI or Object
func unmarshalItem(data []byte) (Item, error) {
	// Try as string (IRI) first
	var iri string
	if err := json.Unmarshal(data, &iri); err == nil {
		return IRI(iri), nil
	}

	// Try as object
	var obj Object
	if err := json.Unmarshal(data, &obj); err == nil {
		return &obj, nil
	}

	return nil, fmt.Errorf("invalid item")
}

// PublicKey represents a public key
type PublicKey struct {
	ID           IRI    `json:"id,omitempty"`
	Owner        IRI    `json:"owner,omitempty"`
	PublicKeyPem string `json:"publicKeyPem,omitempty"`
}

// MarshalJSON implements json.Marshaler for PublicKey
func (p PublicKey) MarshalJSON() ([]byte, error) {
	type Alias PublicKey
	return json.Marshal(Alias(p))
}

// Endpoints represents actor endpoints
type Endpoints struct {
	SharedInbox Item `json:"sharedInbox,omitempty"`
}

// MarshalJSON implements json.Marshaler for Endpoints
func (e Endpoints) MarshalJSON() ([]byte, error) {
	m := make(map[string]any)
	if e.SharedInbox != nil {
		m["sharedInbox"] = e.SharedInbox
	}
	return json.Marshal(m)
}

// UnmarshalJSON implements json.Unmarshaler for Endpoints
func (e *Endpoints) UnmarshalJSON(data []byte) error {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}

	if raw, ok := m["sharedInbox"]; ok {
		item, err := unmarshalItem(raw)
		if err != nil {
			return err
		}
		e.SharedInbox = item
	}

	return nil
}

// Object represents an ActivityPub Object
type Object struct {
	Context      any                   `json:"@context,omitempty"`
	ID           IRI                   `json:"id,omitempty"`
	Type         ActivityType          `json:"type,omitempty"`
	Name         NaturalLanguageValues `json:"name,omitempty"`
	Summary      NaturalLanguageValues `json:"summary,omitempty"`
	Content      NaturalLanguageValues `json:"content,omitempty"`
	MediaType    MimeType              `json:"mediaType,omitempty"`
	URL          Item                  `json:"url,omitempty"`
	AttributedTo Item                  `json:"attributedTo,omitempty"`
	InReplyTo    Item                  `json:"inReplyTo,omitempty"`
	To           ItemCollection        `json:"to,omitempty"`
	CC           ItemCollection        `json:"cc,omitempty"`
	Tag          ItemCollection        `json:"tag,omitempty"`
	Attachment   any                   `json:"attachment,omitempty"`
	Published    time.Time             `json:"published,omitzero"`
	Updated      time.Time             `json:"updated,omitzero"`
}

// GetLink returns the object's ID
func (o *Object) GetLink() IRI {
	return o.ID
}

// GetType returns the object's type
func (o *Object) GetType() ActivityType {
	return o.Type
}

// IsLink returns false for Object
func (o *Object) IsLink() bool {
	return false
}

// IsObject returns true for Object
func (o *Object) IsObject() bool {
	return true
}

// Note represents an ActivityPub Note (short-form content)
type Note = Object

// Person represents an ActivityPub Person (actor)
type Person struct {
	Object
	PreferredUsername  NaturalLanguageValues `json:"preferredUsername,omitempty"`
	Inbox              IRI                   `json:"inbox,omitempty"`
	Outbox             IRI                   `json:"outbox,omitempty"`
	Following          IRI                   `json:"following,omitempty"`
	Followers          IRI                   `json:"followers,omitempty"`
	PublicKey          PublicKey             `json:"publicKey"`
	Endpoints          *Endpoints            `json:"endpoints,omitempty"`
	Icon               any                   `json:"icon,omitempty"`
	AlsoKnownAs        ItemCollection        `json:"alsoKnownAs,omitempty"`
	AttributionDomains ItemCollection        `json:"attributionDomains,omitempty"`
}

// Actor is an alias for Person
type Actor = Person

// Image represents an ActivityPub Image
type Image = Object

// Activity represents an ActivityPub Activity
type Activity struct {
	Context   any            `json:"@context,omitempty"`
	ID        IRI            `json:"id,omitempty"`
	Type      ActivityType   `json:"type,omitempty"`
	Actor     Item           `json:"actor,omitempty"`
	Object    Item           `json:"object,omitempty"`
	To        ItemCollection `json:"to,omitempty"`
	CC        ItemCollection `json:"cc,omitempty"`
	Published time.Time      `json:"published,omitzero"`
	Updated   time.Time      `json:"updated,omitzero"`
}

// GetLink returns the activity's ID
func (a *Activity) GetLink() IRI {
	return a.ID
}

// GetType returns the activity's type
func (a *Activity) GetType() ActivityType {
	return a.Type
}

// IsLink returns false for Activity
func (a *Activity) IsLink() bool {
	return false
}

// IsObject returns true for Activity
func (a *Activity) IsObject() bool {
	return true
}

// Collection represents an ActivityPub Collection
type Collection struct {
	Object
	TotalItems uint           `json:"totalItems,omitempty"`
	Items      ItemCollection `json:"items,omitempty"`
}
