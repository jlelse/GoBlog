// Package activitypub provides types and utilities for working with ActivityPub objects.
// This is a minimal internal implementation focused on GoBlog's specific needs.
package activitypub

import (
	"net/url"
	"time"
)

// ActivityPub namespaces
const (
	ActivityBaseURI    = "https://www.w3.org/ns/activitystreams"
	SecurityContextURI = "https://w3id.org/security/v1"
	PublicNS           = IRI("https://www.w3.org/ns/activitystreams#Public")
)

// ActivityType represents the type of an ActivityPub object
type ActivityType string

const (
	// Common ActivityPub types
	ArticleType      ActivityType = "Article"
	CollectionType   ActivityType = "Collection"
	ImageType        ActivityType = "Image"
	MentionType      ActivityType = "Mention"
	NoteType         ActivityType = "Note"
	ObjectType       ActivityType = "Object"
	PersonType       ActivityType = "Person"
	ServiceType      ActivityType = "Service"
	GroupType        ActivityType = "Group"
	OrganizationType ActivityType = "Organization"
	ApplicationType  ActivityType = "Application"

	// Activity types
	AcceptType   ActivityType = "Accept"
	AnnounceType ActivityType = "Announce"
	BlockType    ActivityType = "Block"
	CreateType   ActivityType = "Create"
	DeleteType   ActivityType = "Delete"
	FollowType   ActivityType = "Follow"
	LikeType     ActivityType = "Like"
	MoveType     ActivityType = "Move"
	UndoType     ActivityType = "Undo"
	UpdateType   ActivityType = "Update"
)

// Item represents an ActivityPub item (can be an IRI or an Object)
type Item interface {
	GetLink() IRI
	GetType() ActivityType
	IsLink() bool
	IsObject() bool
}

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

// MimeType represents a MIME type
type MimeType string

// NaturalLanguageValue represents a value in a natural language
type NaturalLanguageValue struct {
	Value string
	Lang  string
}

// NaturalLanguageValues is a collection of language-tagged values
type NaturalLanguageValues []NaturalLanguageValue

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

// IsActorType returns true if the type represents an ActivityPub actor.
func IsActorType(typ ActivityType) bool {
	switch typ {
	case PersonType, ServiceType, GroupType, OrganizationType, ApplicationType:
		return true
	default:
		return false
	}
}

// PublicKey represents a public key
type PublicKey struct {
	ID           IRI    `json:"id,omitempty"`
	Owner        IRI    `json:"owner,omitempty"`
	PublicKeyPem string `json:"publicKeyPem,omitempty"`
}

// Endpoints represents actor endpoints
type Endpoints struct {
	SharedInbox Item `json:"sharedInbox,omitempty"`
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
	Href         Item                  `json:"href,omitempty"`
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

// Actor represents an ActivityPub actor
type Actor struct {
	Object
	PreferredUsername  NaturalLanguageValues `json:"preferredUsername,omitempty"`
	Inbox              IRI                   `json:"inbox,omitempty"`
	Outbox             IRI                   `json:"outbox,omitempty"`
	Following          IRI                   `json:"following,omitempty"`
	Followers          IRI                   `json:"followers,omitempty"`
	PublicKey          PublicKey             `json:"publicKey,omitempty"`
	Endpoints          *Endpoints            `json:"endpoints,omitempty"`
	Icon               Item                  `json:"icon,omitempty"`
	AlsoKnownAs        ItemCollection        `json:"alsoKnownAs,omitempty"`
	AttributionDomains ItemCollection        `json:"attributionDomains,omitempty"`
	MovedTo            Item                  `json:"movedTo,omitempty"`
}

// Image represents an ActivityPub Image
type Image = Object

// Activity represents an ActivityPub Activity
type Activity struct {
	Context   any            `json:"@context,omitempty"`
	ID        IRI            `json:"id,omitempty"`
	Type      ActivityType   `json:"type,omitempty"`
	Actor     Item           `json:"actor,omitempty"`
	Object    Item           `json:"object,omitempty"`
	Target    Item           `json:"target,omitempty"`
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
