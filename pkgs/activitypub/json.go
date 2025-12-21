package activitypub

import (
	"bytes"
	"encoding/json"
	"fmt"

	"go.goblog.app/app/pkgs/bufferpool"
)

// MarshalJSONNoHTMLEscape marshals v to JSON without HTML escaping
func MarshalJSONNoHTMLEscape(v any) ([]byte, error) {
	buf := bufferpool.Get()
	defer bufferpool.Put(buf)
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	// Remove trailing newline added by Encoder
	result := bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
	return result, nil
}

// marshalJSONNoHTMLEscape is the internal version
func marshalJSONNoHTMLEscape(v any) ([]byte, error) {
	return MarshalJSONNoHTMLEscape(v)
}

// JSONWrite appends data to a byte slice
func JSONWrite(b *[]byte, data ...byte) {
	*b = append(*b, data...)
}

// JSONWriteProp writes a property to JSON
func JSONWriteProp(b *[]byte, name string, val []byte) bool {
	if len(val) == 0 {
		return false
	}
	if len(*b) > 1 && (*b)[len(*b)-1] != '{' {
		JSONWrite(b, ',')
	}
	JSONWrite(b, '"')
	JSONWrite(b, []byte(name)...)
	JSONWrite(b, '"', ':')
	JSONWrite(b, val...)
	return true
}

// JSONWriteItemProp writes an Item property to JSON
func JSONWriteItemProp(b *[]byte, name string, item Item) bool {
	if item == nil {
		return false
	}
	val, err := marshalJSONNoHTMLEscape(item)
	if err != nil || len(val) == 0 {
		return false
	}
	return JSONWriteProp(b, name, val)
}

// JSONWriteNaturalLanguageProp writes a NaturalLanguageValues property to JSON
func JSONWriteNaturalLanguageProp(b *[]byte, name string, val NaturalLanguageValues) bool {
	if len(val) == 0 {
		return false
	}
	data, err := marshalJSONNoHTMLEscape(val)
	if err != nil || len(data) == 0 {
		return false
	}
	return JSONWriteProp(b, name, data)
}

// JSONWriteItemCollectionProp writes an ItemCollection property to JSON
func JSONWriteItemCollectionProp(b *[]byte, name string, col ItemCollection, flatten bool) bool {
	if len(col) == 0 {
		return false
	}
	data, err := marshalJSONNoHTMLEscape(col)
	if err != nil || len(data) == 0 {
		return false
	}
	return JSONWriteProp(b, name, data)
}

// JSONWriteObjectValue writes an Object's fields to JSON
func JSONWriteObjectValue(b *[]byte, o Object) bool {
	notEmpty := false

	// Order: id, type, mediaType, name, summary, content, attachment, attributedTo, inReplyTo, tag, href/url, to, cc, published, updated
	if o.ID != "" {
		notEmpty = JSONWriteItemProp(b, "id", o.ID) || notEmpty
	}
	if o.Type != "" {
		val, _ := marshalJSONNoHTMLEscape(string(o.Type))
		notEmpty = JSONWriteProp(b, "type", val) || notEmpty
	}
	if o.MediaType != "" {
		val, _ := marshalJSONNoHTMLEscape(string(o.MediaType))
		notEmpty = JSONWriteProp(b, "mediaType", val) || notEmpty
	}
	if len(o.Name) > 0 {
		notEmpty = JSONWriteNaturalLanguageProp(b, "name", o.Name) || notEmpty
	}
	if len(o.Summary) > 0 {
		notEmpty = JSONWriteNaturalLanguageProp(b, "summary", o.Summary) || notEmpty
	}
	if len(o.Content) > 0 {
		notEmpty = JSONWriteNaturalLanguageProp(b, "content", o.Content) || notEmpty
	}
	if o.Attachment != nil {
		val, _ := marshalJSONNoHTMLEscape(o.Attachment)
		notEmpty = JSONWriteProp(b, "attachment", val) || notEmpty
	}
	if o.AttributedTo != nil {
		notEmpty = JSONWriteItemProp(b, "attributedTo", o.AttributedTo) || notEmpty
	}
	if o.InReplyTo != nil {
		notEmpty = JSONWriteItemProp(b, "inReplyTo", o.InReplyTo) || notEmpty
	}
	if len(o.Tag) > 0 {
		notEmpty = JSONWriteItemCollectionProp(b, "tag", o.Tag, false) || notEmpty
	}
	if o.URL != nil {
		// Use "href" for Mention type, "url" for others
		fieldName := "url"
		if o.Type == MentionType {
			fieldName = "href"
		}
		notEmpty = JSONWriteItemProp(b, fieldName, o.URL) || notEmpty
	}
	if len(o.To) > 0 {
		notEmpty = JSONWriteItemCollectionProp(b, "to", o.To, false) || notEmpty
	}
	if len(o.CC) > 0 {
		notEmpty = JSONWriteItemCollectionProp(b, "cc", o.CC, false) || notEmpty
	}
	if !o.Published.IsZero() {
		val, _ := marshalJSONNoHTMLEscape(o.Published)
		notEmpty = JSONWriteProp(b, "published", val) || notEmpty
	}
	if !o.Updated.IsZero() {
		val, _ := marshalJSONNoHTMLEscape(o.Updated)
		notEmpty = JSONWriteProp(b, "updated", val) || notEmpty
	}

	return notEmpty
}

// MarshalJSON implements json.Marshaler for Activity
func (a Activity) MarshalJSON() ([]byte, error) {
	b := make([]byte, 0)
	JSONWrite(&b, '{')

	notEmpty := false

	if a.ID != "" {
		notEmpty = JSONWriteItemProp(&b, "id", a.ID) || notEmpty
	}
	if a.Type != "" {
		val, _ := marshalJSONNoHTMLEscape(string(a.Type))
		notEmpty = JSONWriteProp(&b, "type", val) || notEmpty
	}
	if a.Actor != nil {
		notEmpty = JSONWriteItemProp(&b, "actor", a.Actor) || notEmpty
	}
	if a.Object != nil {
		notEmpty = JSONWriteItemProp(&b, "object", a.Object) || notEmpty
	}
	if len(a.To) > 0 {
		notEmpty = JSONWriteItemCollectionProp(&b, "to", a.To, false) || notEmpty
	}
	if len(a.CC) > 0 {
		notEmpty = JSONWriteItemCollectionProp(&b, "cc", a.CC, false) || notEmpty
	}
	if !a.Published.IsZero() {
		val, _ := marshalJSONNoHTMLEscape(a.Published)
		notEmpty = JSONWriteProp(&b, "published", val) || notEmpty
	}
	if !a.Updated.IsZero() {
		val, _ := marshalJSONNoHTMLEscape(a.Updated)
		notEmpty = JSONWriteProp(&b, "updated", val) || notEmpty
	}

	if notEmpty {
		JSONWrite(&b, '}')
		return b, nil
	}
	return nil, nil
}

// UnmarshalJSON implements json.Unmarshaler for Activity
func (a *Activity) UnmarshalJSON(data []byte) error {
	// Use a type alias to avoid recursion
	type Alias Activity
	aux := &struct {
		Actor  json.RawMessage `json:"actor,omitempty"`
		Object json.RawMessage `json:"object,omitempty"`
		*Alias
	}{
		Alias: (*Alias)(a),
	}

	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	// Unmarshal Item fields
	if len(aux.Actor) > 0 {
		item, err := unmarshalItem(aux.Actor)
		if err == nil {
			a.Actor = item
		}
	}
	if len(aux.Object) > 0 {
		item, err := unmarshalItem(aux.Object)
		if err == nil {
			a.Object = item
		}
	}

	return nil
}

// MarshalJSON implements json.Marshaler for Person
func (p Person) MarshalJSON() ([]byte, error) {
	b := make([]byte, 0)
	JSONWrite(&b, '{')

	notEmpty := false

	// Write Object fields in custom order: id, type, name, summary, icon, url
	if p.Object.ID != "" {
		notEmpty = JSONWriteItemProp(&b, "id", p.Object.ID) || notEmpty
	}
	if p.Object.Type != "" {
		val, _ := marshalJSONNoHTMLEscape(string(p.Object.Type))
		notEmpty = JSONWriteProp(&b, "type", val) || notEmpty
	}
	if len(p.Object.Name) > 0 {
		notEmpty = JSONWriteNaturalLanguageProp(&b, "name", p.Object.Name) || notEmpty
	}
	if len(p.Object.Summary) > 0 {
		notEmpty = JSONWriteNaturalLanguageProp(&b, "summary", p.Object.Summary) || notEmpty
	}
	if p.Icon != nil {
		if v, err := marshalJSONNoHTMLEscape(p.Icon); err == nil && len(v) > 0 {
			notEmpty = JSONWriteProp(&b, "icon", v) || notEmpty
		}
	}
	if p.Object.URL != nil {
		notEmpty = JSONWriteItemProp(&b, "url", p.Object.URL) || notEmpty
	}

	// Person-specific fields
	if p.Inbox != "" {
		notEmpty = JSONWriteItemProp(&b, "inbox", p.Inbox) || notEmpty
	}
	if p.Outbox != "" {
		notEmpty = JSONWriteItemProp(&b, "outbox", p.Outbox) || notEmpty
	}
	if p.Following != "" {
		notEmpty = JSONWriteItemProp(&b, "following", p.Following) || notEmpty
	}
	if p.Followers != "" {
		notEmpty = JSONWriteItemProp(&b, "followers", p.Followers) || notEmpty
	}
	if len(p.PreferredUsername) > 0 {
		notEmpty = JSONWriteNaturalLanguageProp(&b, "preferredUsername", p.PreferredUsername) || notEmpty
	}
	if len(p.PublicKey.PublicKeyPem)+len(p.PublicKey.ID) > 0 {
		if v, err := marshalJSONNoHTMLEscape(p.PublicKey); err == nil && len(v) > 0 {
			notEmpty = JSONWriteProp(&b, "publicKey", v) || notEmpty
		}
	}
	if p.Endpoints != nil {
		if v, err := marshalJSONNoHTMLEscape(p.Endpoints); err == nil && len(v) > 0 {
			notEmpty = JSONWriteProp(&b, "endpoints", v) || notEmpty
		}
	}

	// GoBlog-specific fields
	if len(p.AlsoKnownAs) > 0 {
		notEmpty = JSONWriteItemCollectionProp(&b, "alsoKnownAs", p.AlsoKnownAs, false) || notEmpty
	}
	if len(p.AttributionDomains) > 0 {
		notEmpty = JSONWriteItemCollectionProp(&b, "attributionDomains", p.AttributionDomains, false) || notEmpty
	}

	if notEmpty {
		JSONWrite(&b, '}')
		return b, nil
	}
	return nil, nil
}

// UnmarshalJSON implements json.Unmarshaler for Person
func (p *Person) UnmarshalJSON(data []byte) error {
	// First unmarshal into the embedded Object
	if err := json.Unmarshal(data, &p.Object); err != nil {
		return err
	}

	// Then unmarshal Person-specific fields
	aux := &struct {
		PreferredUsername  NaturalLanguageValues `json:"preferredUsername,omitempty"`
		Inbox              IRI                   `json:"inbox,omitempty"`
		Outbox             IRI                   `json:"outbox,omitempty"`
		Following          IRI                   `json:"following,omitempty"`
		Followers          IRI                   `json:"followers,omitempty"`
		PublicKey          PublicKey             `json:"publicKey"`
		Endpoints          *Endpoints            `json:"endpoints,omitempty"`
		Icon               json.RawMessage       `json:"icon,omitempty"`
		AlsoKnownAs        ItemCollection        `json:"alsoKnownAs,omitempty"`
		AttributionDomains ItemCollection        `json:"attributionDomains,omitempty"`
	}{}

	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	p.PreferredUsername = aux.PreferredUsername
	p.Inbox = aux.Inbox
	p.Outbox = aux.Outbox
	p.Following = aux.Following
	p.Followers = aux.Followers
	p.PublicKey = aux.PublicKey
	p.Endpoints = aux.Endpoints
	p.AlsoKnownAs = aux.AlsoKnownAs
	p.AttributionDomains = aux.AttributionDomains
	if len(aux.Icon) > 0 {
		// Try to unmarshal icon (can be various types)
		var iconObj Object
		if err := json.Unmarshal(aux.Icon, &iconObj); err == nil {
			p.Icon = &iconObj
		}
	}

	return nil
}

// MarshalJSON implements json.Marshaler for Object
func (o Object) MarshalJSON() ([]byte, error) {
	b := make([]byte, 0)
	JSONWrite(&b, '{')

	notEmpty := JSONWriteObjectValue(&b, o)

	if notEmpty {
		JSONWrite(&b, '}')
		return b, nil
	}
	return nil, fmt.Errorf("empty object")
}

// UnmarshalJSON implements json.Unmarshaler for Object
func (o *Object) UnmarshalJSON(data []byte) error {
	// Use a type alias to avoid recursion
	type Alias Object
	aux := &struct {
		AttributedTo json.RawMessage `json:"attributedTo,omitempty"`
		InReplyTo    json.RawMessage `json:"inReplyTo,omitempty"`
		URL          json.RawMessage `json:"url,omitempty"`
		*Alias
	}{
		Alias: (*Alias)(o),
	}

	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	// Unmarshal Item fields
	if len(aux.AttributedTo) > 0 {
		item, err := unmarshalItem(aux.AttributedTo)
		if err == nil {
			o.AttributedTo = item
		}
	}
	if len(aux.InReplyTo) > 0 {
		item, err := unmarshalItem(aux.InReplyTo)
		if err == nil {
			o.InReplyTo = item
		}
	}
	if len(aux.URL) > 0 {
		item, err := unmarshalItem(aux.URL)
		if err == nil {
			o.URL = item
		}
	}

	return nil
}

// MarshalJSON implements json.Marshaler for Collection
func (c Collection) MarshalJSON() ([]byte, error) {
	b := make([]byte, 0)
	JSONWrite(&b, '{')

	notEmpty := JSONWriteObjectValue(&b, c.Object)

	if c.TotalItems > 0 {
		val, _ := marshalJSONNoHTMLEscape(c.TotalItems)
		notEmpty = JSONWriteProp(&b, "totalItems", val) || notEmpty
	}
	if len(c.Items) > 0 {
		notEmpty = JSONWriteItemCollectionProp(&b, "items", c.Items, false) || notEmpty
	}

	if notEmpty {
		JSONWrite(&b, '}')
		return b, nil
	}
	return nil, nil
}

// UnmarshalJSON implements json.Unmarshaler for Collection
func (c *Collection) UnmarshalJSON(data []byte) error {
	// First unmarshal into the embedded Object
	if err := json.Unmarshal(data, &c.Object); err != nil {
		return err
	}

	// Then unmarshal Collection-specific fields
	aux := &struct {
		TotalItems uint           `json:"totalItems,omitempty"`
		Items      ItemCollection `json:"items,omitempty"`
	}{}

	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	c.TotalItems = aux.TotalItems
	c.Items = aux.Items

	return nil
}
