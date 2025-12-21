package activitypub

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.goblog.app/app/pkgs/activitypub/jsonld"
)

func TestIRI(t *testing.T) {
	iri := IRI("https://example.com/users/alice")

	assert.Equal(t, "https://example.com/users/alice", iri.String())
	assert.True(t, iri.IsLink())
	assert.False(t, iri.IsObject())
	assert.Equal(t, iri, iri.GetLink())

	url, err := iri.URL()
	require.NoError(t, err)
	assert.Equal(t, "https", url.Scheme)
	assert.Equal(t, "example.com", url.Host)
}

func TestNaturalLanguageValues(t *testing.T) {
	// Test single value
	nlv := DefaultNaturalLanguage("Hello")
	assert.Equal(t, "Hello", nlv.First().String())

	// Test Set
	nlv2 := NaturalLanguageValues{}
	nlv2.Set("en", "Hello")
	nlv2.Set("fr", "Bonjour")
	assert.Len(t, nlv2, 2)

	// Test JSON marshaling - single value
	single := DefaultNaturalLanguage("Test")
	data, err := json.Marshal(single)
	require.NoError(t, err)
	assert.Equal(t, `"Test"`, string(data))

	// Test JSON unmarshaling - string
	var unmarshaled NaturalLanguageValues
	err = json.Unmarshal([]byte(`"Test"`), &unmarshaled)
	require.NoError(t, err)
	assert.Equal(t, "Test", unmarshaled.First().String())

	// Test JSON unmarshaling - map
	err = json.Unmarshal([]byte(`{"en":"Hello","fr":"Bonjour"}`), &unmarshaled)
	require.NoError(t, err)
	assert.Len(t, unmarshaled, 2)
}

func TestObjectNew(t *testing.T) {
	obj := ObjectNew(NoteType)
	assert.NotNil(t, obj)
	assert.Equal(t, NoteType, obj.Type)
	assert.False(t, obj.IsLink())
	assert.True(t, obj.IsObject())
}

func TestPersonNew(t *testing.T) {
	person := PersonNew(IRI("https://example.com/users/alice"))
	assert.NotNil(t, person)
	assert.Equal(t, PersonType, person.Type)
	assert.Equal(t, IRI("https://example.com/users/alice"), person.ID)
}

func TestCreateNew(t *testing.T) {
	note := ObjectNew(NoteType)
	note.ID = IRI("https://example.com/notes/1")
	note.Content = DefaultNaturalLanguage("Hello, world!")

	create := CreateNew(ID("https://example.com/activities/1"), note)
	assert.NotNil(t, create)
	assert.Equal(t, CreateType, create.Type)
	assert.Equal(t, IRI("https://example.com/activities/1"), create.ID)
	assert.NotNil(t, create.Object)
}

func TestItemCollection(t *testing.T) {
	col := ItemCollection{}

	// Test Append
	col.Append(IRI("https://example.com/1"))
	col.Append(IRI("https://example.com/2"))
	assert.Len(t, col, 2)

	// Test Contains
	assert.True(t, col.Contains(IRI("https://example.com/1")))
	assert.False(t, col.Contains(IRI("https://example.com/3")))

	// Test JSON marshaling - single item (now always returns array)
	single := ItemCollection{IRI("https://example.com/1")}
	data, err := json.Marshal(single)
	require.NoError(t, err)
	assert.Equal(t, `["https://example.com/1"]`, string(data))

	// Test JSON marshaling - multiple items
	data, err = json.Marshal(col)
	require.NoError(t, err)
	assert.Contains(t, string(data), "https://example.com/1")
	assert.Contains(t, string(data), "https://example.com/2")

	// Test JSON unmarshaling - single item
	var unmarshaled ItemCollection
	err = json.Unmarshal([]byte(`"https://example.com/1"`), &unmarshaled)
	require.NoError(t, err)
	assert.Len(t, unmarshaled, 1)
	assert.Equal(t, IRI("https://example.com/1"), unmarshaled[0].GetLink())

	// Test JSON unmarshaling - array
	err = json.Unmarshal([]byte(`["https://example.com/1","https://example.com/2"]`), &unmarshaled)
	require.NoError(t, err)
	assert.Len(t, unmarshaled, 2)
}

func TestUnmarshalJSON(t *testing.T) {
	// Test unmarshaling a Person
	personJSON := `{
		"@context": "https://www.w3.org/ns/activitystreams",
		"type": "Person",
		"id": "https://example.com/users/alice",
		"name": "Alice",
		"preferredUsername": "alice"
	}`

	item, err := UnmarshalJSON([]byte(personJSON))
	require.NoError(t, err)

	person, err := ToActor(item)
	require.NoError(t, err)
	assert.Equal(t, PersonType, person.Type)
	assert.Equal(t, "Alice", person.Name.First().String())

	// Test unmarshaling an Activity
	activityJSON := `{
		"type": "Create",
		"id": "https://example.com/activities/1",
		"actor": "https://example.com/users/alice",
		"object": {
			"type": "Note",
			"content": "Hello"
		}
	}`

	item, err = UnmarshalJSON([]byte(activityJSON))
	require.NoError(t, err)

	activity, err := ToActivity(item)
	require.NoError(t, err)
	assert.Equal(t, CreateType, activity.Type)
	assert.NotNil(t, activity.Actor)
	assert.NotNil(t, activity.Object)
}

func TestToObject(t *testing.T) {
	// Test with Object
	obj := ObjectNew(NoteType)
	obj.ID = IRI("https://example.com/notes/1")

	converted, err := ToObject(obj)
	require.NoError(t, err)
	assert.Equal(t, obj.ID, converted.ID)

	// Test with Person
	person := PersonNew(IRI("https://example.com/users/alice"))
	converted, err = ToObject(person)
	require.NoError(t, err)
	assert.Equal(t, PersonType, converted.Type)

	// Test with IRI (should fail)
	iri := IRI("https://example.com/test")
	_, err = ToObject(iri)
	assert.Error(t, err)
}

func TestIsObject(t *testing.T) {
	obj := ObjectNew(NoteType)
	assert.True(t, IsObject(obj))

	iri := IRI("https://example.com/test")
	assert.False(t, IsObject(iri))

	assert.False(t, IsObject(nil))
}

func TestOnActor(t *testing.T) {
	person := PersonNew(IRI("https://example.com/users/alice"))
	person.Name = DefaultNaturalLanguage("Alice")

	called := false
	err := OnActor(person, func(actor *Actor) error {
		called = true
		assert.Equal(t, "Alice", actor.Name.First().String())
		return nil
	})

	require.NoError(t, err)
	assert.True(t, called)

	// Test with non-actor
	obj := ObjectNew(NoteType)
	err = OnActor(obj, func(actor *Actor) error {
		return nil
	})
	assert.Error(t, err)
}

func TestNoteMarshaling(t *testing.T) {
	note := ObjectNew(NoteType)
	note.ID = IRI("https://example.com/notes/1")
	note.Content = DefaultNaturalLanguage("Hello, world!")
	note.AttributedTo = IRI("https://example.com/users/alice")
	note.Published = time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)

	data, err := json.Marshal(note)
	require.NoError(t, err)

	var unmarshaled Object
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, note.ID, unmarshaled.ID)
	assert.Equal(t, note.Type, unmarshaled.Type)
	assert.Equal(t, "Hello, world!", unmarshaled.Content.First().String())
}

func TestPersonMarshaling(t *testing.T) {
	person := PersonNew(IRI("https://example.com/users/alice"))
	person.Name = DefaultNaturalLanguage("Alice")
	person.PreferredUsername = DefaultNaturalLanguage("alice")
	person.Inbox = IRI("https://example.com/users/alice/inbox")
	person.PublicKey.ID = IRI("https://example.com/users/alice#main-key")
	person.PublicKey.Owner = IRI("https://example.com/users/alice")
	person.PublicKey.PublicKeyPem = "-----BEGIN PUBLIC KEY-----\ntest\n-----END PUBLIC KEY-----"

	data, err := json.Marshal(person)
	require.NoError(t, err)

	var unmarshaled Person
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, person.ID, unmarshaled.ID)
	assert.Equal(t, person.Type, unmarshaled.Type)
	assert.Equal(t, "Alice", unmarshaled.Name.First().String())
	assert.Equal(t, "alice", unmarshaled.PreferredUsername.First().String())
	assert.Equal(t, person.PublicKey.PublicKeyPem, unmarshaled.PublicKey.PublicKeyPem)
}

func TestPersonMarshalingWithExtensions(t *testing.T) {
	// Test Person with AlsoKnownAs and AttributionDomains
	person := PersonNew(IRI("https://example.com/users/alice"))
	person.Name = DefaultNaturalLanguage("Alice")
	person.PreferredUsername = DefaultNaturalLanguage("alice")
	person.AlsoKnownAs = ItemCollection{IRI("https://other.example/@alice"), IRI("https://another.example/alice")}
	person.AttributionDomains = ItemCollection{IRI("example.com"), IRI("other.example")}

	data, err := json.Marshal(person)
	require.NoError(t, err)

	var unmarshaled Person
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, person.ID, unmarshaled.ID)
	assert.Len(t, unmarshaled.AlsoKnownAs, 2)
	assert.Len(t, unmarshaled.AttributionDomains, 2)
	assert.Contains(t, string(data), "alsoKnownAs")
	assert.Contains(t, string(data), "attributionDomains")
}

func TestActivityMarshaling(t *testing.T) {
	note := ObjectNew(NoteType)
	note.ID = IRI("https://example.com/notes/1")
	note.Content = DefaultNaturalLanguage("Hello")

	create := CreateNew(ID("https://example.com/activities/1"), note)
	create.Actor = IRI("https://example.com/users/alice")
	create.Published = time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)

	data, err := json.Marshal(create)
	require.NoError(t, err)

	var unmarshaled Activity
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, create.ID, unmarshaled.ID)
	assert.Equal(t, create.Type, unmarshaled.Type)
	assert.NotNil(t, unmarshaled.Actor)
	assert.NotNil(t, unmarshaled.Object)
}

func TestCollectionMarshaling(t *testing.T) {
	collection := CollectionNew(IRI("https://example.com/followers"))
	collection.Items.Append(IRI("https://example.com/users/alice"))
	collection.Items.Append(IRI("https://example.com/users/bob"))
	collection.TotalItems = 2

	data, err := json.Marshal(collection)
	require.NoError(t, err)

	var unmarshaled Collection
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, collection.ID, unmarshaled.ID)
	assert.Equal(t, uint(2), unmarshaled.TotalItems)
	assert.Len(t, unmarshaled.Items, 2)
}

func TestJSONLDMarshaling(t *testing.T) {
	note := ObjectNew(NoteType)
	note.ID = IRI("https://example.com/notes/1")
	note.Content = DefaultNaturalLanguage("Hello")

	data, err := jsonld.WithContext(
		jsonld.IRI(ActivityBaseURI),
		jsonld.IRI(SecurityContextURI),
	).Marshal(note)
	require.NoError(t, err)

	// Verify it contains the context
	var result map[string]any
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	assert.NotNil(t, result["@context"])
	assert.Equal(t, "https://example.com/notes/1", result["id"])
	assert.Equal(t, "Note", result["type"])
}

func TestMentionNew(t *testing.T) {
	mention := MentionNew(IRI("https://example.com/users/alice"))
	assert.NotNil(t, mention)
	assert.Equal(t, MentionType, mention.Type)
	assert.Equal(t, IRI("https://example.com/users/alice"), mention.ID)
}

func TestEndpoints(t *testing.T) {
	endpoints := &Endpoints{
		SharedInbox: IRI("https://example.com/inbox"),
	}

	data, err := json.Marshal(endpoints)
	require.NoError(t, err)

	var unmarshaled Endpoints
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.NotNil(t, unmarshaled.SharedInbox)
	assert.Equal(t, IRI("https://example.com/inbox"), unmarshaled.SharedInbox.GetLink())
}

func TestPublicKey(t *testing.T) {
	pk := PublicKey{
		ID:           IRI("https://example.com/users/alice#main-key"),
		Owner:        IRI("https://example.com/users/alice"),
		PublicKeyPem: "test-key",
	}

	data, err := json.Marshal(pk)
	require.NoError(t, err)

	var unmarshaled PublicKey
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, pk.ID, unmarshaled.ID)
	assert.Equal(t, pk.Owner, unmarshaled.Owner)
	assert.Equal(t, pk.PublicKeyPem, unmarshaled.PublicKeyPem)
}
