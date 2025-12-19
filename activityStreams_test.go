package main

import (
"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.goblog.app/app/pkgs/activitypub"
	"go.goblog.app/app/pkgs/activitypub/jsonld"
)

func Test_apUsername(t *testing.T) {
	item, err := activitypub.UnmarshalJSON([]byte(`
		{
			"@context": [
				"https://www.w3.org/ns/activitystreams",
				"https://w3id.org/security/v1"
			],
			"id": "https://example.org/users/user",
			"type": "Person",
			"preferredUsername": "user",
			"name": "Example user",
			"url": "https://example.org/@user"
		}
		`))
	require.NoError(t, err)

	actor, err := activitypub.ToActor(item)
	require.NoError(t, err)

	username := apUsername(actor)
	assert.Equal(t, "@user@example.org", username)
}

func Test_toAPNote_PublicNote(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Server.PublicAddress = "https://example.com"
	app.cfg.Blogs = map[string]*configBlog{
		"testblog": {
			Path: "",
		},
	}
	app.cfg.Micropub = &configMicropub{
		PhotoParam: "photo",
	}
	app.cfg.ActivityPub = &configActivityPub{
		TagsTaxonomies: []string{"tags"},
	}
	err := app.initConfig(false)
	require.NoError(t, err)
	_ = app.initTemplateStrings()

	p := &post{
		Path:       "/test",
		Content:    "Test content",
		Published:  "2023-01-01T00:00:00Z",
		Updated:    "2023-01-02T00:00:00Z",
		Blog:       "testblog",
		Section:    "posts",
		Status:     statusPublished,
		Visibility: visibilityPublic,
		Parameters: map[string][]string{
			"title": {"Test Title"},
		},
		RenderedTitle: "Test Title",
	}

	note := app.toAPNote(p)

	assert.Equal(t, activitypub.ArticleType, note.Type)
	assert.Equal(t, "Test Title", note.Name.First().String())
	assert.Contains(t, note.Content.First().String(), "Test content")
	assert.Equal(t, activitypub.MimeType("text/html"), note.MediaType)
	assert.True(t, note.To.Contains(activitypub.PublicNS))

	// JSON validation - check structure
	binary, err := jsonld.WithContext(jsonld.IRI(activitypub.ActivityBaseURI), jsonld.IRI(activitypub.SecurityContextURI)).Marshal(note)
	require.NoError(t, err)
	
	var result map[string]interface{}
	err = json.Unmarshal(binary, &result)
	require.NoError(t, err)
	
	assert.Equal(t, "https://example.com/test", result["id"])
	assert.Equal(t, "Article", result["type"])
	assert.Equal(t, "text/html", result["mediaType"])
	assert.Equal(t, "Test Title", result["name"])
	assert.Contains(t, result["content"], "Test content")
	assert.Equal(t, "https://example.com", result["attributedTo"])
	assert.Equal(t, "https://example.com/test", result["url"])
	assert.NotNil(t, result["to"])
	assert.Equal(t, "2023-01-01T00:00:00Z", result["published"])
	assert.Equal(t, "2023-01-02T00:00:00Z", result["updated"])
}

func Test_toAPNote_UnlistedNote(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Server.PublicAddress = "https://example.com"
	app.cfg.Blogs = map[string]*configBlog{
		"testblog": {
			Path: "",
		},
	}
	app.cfg.Micropub = &configMicropub{
		PhotoParam: "photo",
	}
	app.cfg.ActivityPub = &configActivityPub{
		TagsTaxonomies: []string{"tags"},
	}
	err := app.initConfig(false)
	require.NoError(t, err)
	_ = app.initTemplateStrings()

	p := &post{
		Path:       "/test",
		Content:    "Test content",
		Blog:       "testblog",
		Section:    "posts",
		Status:     statusPublished,
		Visibility: visibilityUnlisted,
		Parameters: map[string][]string{},
	}

	note := app.toAPNote(p)

	assert.Equal(t, activitypub.NoteType, note.Type)
	assert.True(t, note.To.Contains(activitypub.IRI("https://example.com/activitypub/followers/testblog")))
	assert.True(t, note.CC.Contains(activitypub.PublicNS))

	// JSON validation - check structure
	binary, err := jsonld.WithContext(jsonld.IRI(activitypub.ActivityBaseURI), jsonld.IRI(activitypub.SecurityContextURI)).Marshal(note)
	require.NoError(t, err)
	
	var result map[string]interface{}
	err = json.Unmarshal(binary, &result)
	require.NoError(t, err)
	
	assert.Equal(t, "https://example.com/test", result["id"])
	assert.Equal(t, "Note", result["type"])
	assert.Contains(t, result["content"], "Test content")
	assert.NotNil(t, result["to"])
	assert.NotNil(t, result["cc"])
}

func Test_toAPNote_WithImages(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Server.PublicAddress = "https://example.com"
	app.cfg.Blogs = map[string]*configBlog{
		"testblog": {
			Path: "",
		},
	}
	app.cfg.Micropub = &configMicropub{
		PhotoParam: "photo",
	}
	app.cfg.ActivityPub = &configActivityPub{
		TagsTaxonomies: []string{"tags"},
	}
	err := app.initConfig(false)
	require.NoError(t, err)
	_ = app.initTemplateStrings()

	p := &post{
		Path:       "/test",
		Content:    "Test content",
		Blog:       "testblog",
		Section:    "posts",
		Status:     statusPublished,
		Visibility: visibilityPublic,
		Parameters: map[string][]string{
			"photo": {"https://example.com/image1.jpg", "https://example.com/image2.jpg"},
		},
	}

	note := app.toAPNote(p)

	assert.NotNil(t, note.Attachment)
	attachments, ok := note.Attachment.(activitypub.ItemCollection)
	require.True(t, ok)
	assert.Len(t, attachments, 2)
	for _, att := range attachments {
		obj, ok := att.(*activitypub.Object)
		require.True(t, ok)
		assert.Equal(t, activitypub.ImageType, obj.Type)
	}

	// JSON validation - check structure
	binary, err := jsonld.WithContext(jsonld.IRI(activitypub.ActivityBaseURI), jsonld.IRI(activitypub.SecurityContextURI)).Marshal(note)
	require.NoError(t, err)
	
	var result map[string]interface{}
	err = json.Unmarshal(binary, &result)
	require.NoError(t, err)
	
	assert.NotNil(t, result["attachment"])
	assert.Contains(t, string(binary), "image1.jpg")
	assert.Contains(t, string(binary), "image2.jpg")
}

func Test_toAPNote_WithTags(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Server.PublicAddress = "https://example.com"
	app.cfg.Blogs = map[string]*configBlog{
		"testblog": {
			Path: "",
		},
	}
	app.cfg.Micropub = &configMicropub{
		PhotoParam: "photo",
	}
	app.cfg.ActivityPub = &configActivityPub{
		TagsTaxonomies: []string{"tags"},
	}
	err := app.initConfig(false)
	require.NoError(t, err)
	_ = app.initTemplateStrings()

	p := &post{
		Path:       "/test",
		Content:    "Test content",
		Blog:       "testblog",
		Section:    "posts",
		Status:     statusPublished,
		Visibility: visibilityPublic,
		Parameters: map[string][]string{
			"tags": {"tag1", "tag2"},
		},
	}

	note := app.toAPNote(p)

	assert.Len(t, note.Tag, 2)
	for _, tag := range note.Tag {
		obj, ok := tag.(*activitypub.Object)
		require.True(t, ok)
		assert.Equal(t, "Hashtag", string(obj.Type))
	}

	// JSON validation

// JSON validation - check structure
binary, err := jsonld.WithContext(jsonld.IRI(activitypub.ActivityBaseURI), jsonld.IRI(activitypub.SecurityContextURI)).Marshal(note)
require.NoError(t, err)
assert.Contains(t, string(binary), "tag1")
assert.Contains(t, string(binary), "tag2")
assert.Contains(t, string(binary), "Hashtag")
}

func Test_toAPNote_WithMentions(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Server.PublicAddress = "https://example.com"
	app.cfg.Blogs = map[string]*configBlog{
		"testblog": {
			Path: "",
		},
	}
	app.cfg.Micropub = &configMicropub{
		PhotoParam: "photo",
	}
	app.cfg.ActivityPub = &configActivityPub{
		TagsTaxonomies: []string{"tags"},
	}
	err := app.initConfig(false)
	require.NoError(t, err)
	_ = app.initTemplateStrings()

	p := &post{
		Path:       "/test",
		Content:    "Test content",
		Blog:       "testblog",
		Section:    "posts",
		Status:     statusPublished,
		Visibility: visibilityPublic,
		Parameters: map[string][]string{
			activityPubMentionsParameter: {"https://example.com/@user1", "https://example.com/@user2"},
		},
	}

	note := app.toAPNote(p)

	mentionCount := 0
	for _, tag := range note.Tag {
		if tag.GetType() == activitypub.MentionType {
			mentionCount++
		}
	}
	assert.Equal(t, 2, mentionCount)

	// JSON validation

// JSON validation - check structure
	binary, err := jsonld.WithContext(jsonld.IRI(activitypub.ActivityBaseURI), jsonld.IRI(activitypub.SecurityContextURI)).Marshal(note)
	require.NoError(t, err)
	assert.Contains(t, string(binary), "@user1")
	assert.Contains(t, string(binary), "@user2")
	assert.Contains(t, string(binary), "Mention")
}

func Test_toAPNote_WithReply(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Server.PublicAddress = "https://example.com"
	app.cfg.Blogs = map[string]*configBlog{
		"testblog": {
			Path: "",
		},
	}
	app.cfg.Micropub = &configMicropub{
		PhotoParam: "photo",
		ReplyParam: "in-reply-to",
	}
	app.cfg.ActivityPub = &configActivityPub{
		TagsTaxonomies: []string{"tags"},
	}
	err := app.initConfig(false)
	require.NoError(t, err)
	_ = app.initTemplateStrings()

	p := &post{
		Path:       "/test",
		Content:    "Test content",
		Blog:       "testblog",
		Section:    "posts",
		Status:     statusPublished,
		Visibility: visibilityPublic,
		Parameters: map[string][]string{
			"in-reply-to": {"https://example.com/reply-to"},
		},
	}

	note := app.toAPNote(p)

	assert.Equal(t, activitypub.IRI("https://example.com/reply-to"), note.InReplyTo)

	// JSON validation - check structure
	binary, err := jsonld.WithContext(jsonld.IRI(activitypub.ActivityBaseURI), jsonld.IRI(activitypub.SecurityContextURI)).Marshal(note)
	require.NoError(t, err)
	assert.Contains(t, string(binary), "inReplyTo")
	assert.Contains(t, string(binary), "reply-to")
}

func Test_activityPubId(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Server.PublicAddress = "https://example.com"
	err := app.initConfig(false)
	require.NoError(t, err)
	_ = app.initTemplateStrings()

	p := &post{
		Path:       "/test",
		Parameters: map[string][]string{},
	}

	id := app.activityPubId(p)
	assert.Equal(t, activitypub.IRI("https://example.com/test"), id)

	p.Parameters[activityPubVersionParam] = []string{"123456789"}
	id = app.activityPubId(p)
	assert.Equal(t, activitypub.IRI("https://example.com/test?activitypubversion=123456789"), id)
}

func Test_toApPerson(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Server.PublicAddress = "https://example.com"
	app.cfg.Blogs = map[string]*configBlog{
		"testblog": {
			Title:       "Test Blog",
			Description: "A test blog",
		},
	}
	app.cfg.ActivityPub = &configActivityPub{
		AlsoKnownAs:        []string{"https://example.com/aka1"},
		AttributionDomains: []string{"example.com"},
	}
	app.apPubKeyBytes = []byte("test-key")
	err := app.initConfig(false)
	require.NoError(t, err)
	_ = app.initTemplateStrings()

	person := app.toApPerson("testblog")

	assert.Equal(t, "Test Blog", person.Name.First().String())
	assert.Equal(t, "A test blog", person.Summary.First().String())
	assert.Equal(t, "testblog", person.PreferredUsername.First().String())
	assert.Equal(t, activitypub.IRI("https://example.com"), person.ID)
	assert.Equal(t, activitypub.IRI("https://example.com"), person.URL)
	assert.Equal(t, activitypub.IRI("https://example.com/activitypub/inbox/testblog"), person.Inbox)
	assert.Equal(t, activitypub.IRI("https://example.com/activitypub/followers/testblog"), person.Followers)
	assert.Len(t, person.AlsoKnownAs, 1)
	assert.Len(t, person.AttributionDomains, 1)

	// JSON validation - check structure
	binary, err := jsonld.WithContext(jsonld.IRI(activitypub.ActivityBaseURI), jsonld.IRI(activitypub.SecurityContextURI)).Marshal(person)
	require.NoError(t, err)
	assert.Contains(t, string(binary), "Test Blog")
	assert.Contains(t, string(binary), "preferredUsername")
	assert.Contains(t, string(binary), "publicKey")
	assert.Contains(t, string(binary), "alsoKnownAs")
	assert.Contains(t, string(binary), "attributionDomains")
}

func Test_toApPerson_WithProfileImage(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Server.PublicAddress = "https://example.com"
	app.cfg.Blogs = map[string]*configBlog{
		"testblog": {
			Title:       "Test Blog",
			Description: "A test blog",
		},
	}
	app.cfg.ActivityPub = &configActivityPub{
		AlsoKnownAs:        []string{"https://example.com/aka1"},
		AttributionDomains: []string{"example.com"},
	}
	app.apPubKeyBytes = []byte("test-key")
	err := app.initConfig(false)
	require.NoError(t, err)
	_ = app.initTemplateStrings()

	// Create temporary profile image file (empty to have known hash)
	tempFile := filepath.Join(t.TempDir(), "profile.jpg")
	err = os.WriteFile(tempFile, []byte{}, 0644)
	require.NoError(t, err)
	app.cfg.User.ProfileImageFile = tempFile
	app.profileImageHashGroup = nil // Reset to recompute hash

	person := app.toApPerson("testblog")

	assert.Equal(t, "Test Blog", person.Name.First().String())
	assert.Equal(t, "A test blog", person.Summary.First().String())
	assert.Equal(t, "testblog", person.PreferredUsername.First().String())
	assert.Equal(t, activitypub.IRI("https://example.com"), person.ID)
	assert.Equal(t, activitypub.IRI("https://example.com"), person.URL)
	assert.Equal(t, activitypub.IRI("https://example.com/activitypub/inbox/testblog"), person.Inbox)
	assert.Equal(t, activitypub.IRI("https://example.com/activitypub/followers/testblog"), person.Followers)
	assert.Len(t, person.AlsoKnownAs, 1)
	assert.Len(t, person.AttributionDomains, 1)
	assert.NotNil(t, person.Icon)
	iconObj, ok := person.Icon.(*activitypub.Object)
	require.True(t, ok)
	assert.Equal(t, activitypub.ImageType, iconObj.Type)
	assert.Equal(t, activitypub.MimeType("image/jpeg"), iconObj.MediaType)
	assert.Equal(t, activitypub.IRI("https://example.com/profile.jpg?v=e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"), iconObj.URL)

	// JSON validation - check structure
	binary, err := jsonld.WithContext(jsonld.IRI(activitypub.ActivityBaseURI), jsonld.IRI(activitypub.SecurityContextURI)).Marshal(person)
	require.NoError(t, err)
	assert.Contains(t, string(binary), "Test Blog")
	assert.Contains(t, string(binary), "icon")
	assert.Contains(t, string(binary), "profile.jpg")
}

func Test_apUsername_EdgeCases(t *testing.T) {
	// Test with missing preferredUsername
	item, err := activitypub.UnmarshalJSON([]byte(`
		{
			"@context": [
				"https://www.w3.org/ns/activitystreams",
				"https://w3id.org/security/v1"
			],
			"id": "https://example.org/users/user",
			"type": "Person",
			"name": "Example user",
			"url": "https://example.org/@user"
		}
		`))
	require.NoError(t, err)

	actor, err := activitypub.ToActor(item)
	require.NoError(t, err)

	username := apUsername(actor)
	assert.Equal(t, "https://example.org/users/user", username)

	// Test with invalid URL
	item2, err := activitypub.UnmarshalJSON([]byte(`
		{
			"@context": [
				"https://www.w3.org/ns/activitystreams",
				"https://w3id.org/security/v1"
			],
			"id": "https://example.org/users/user",
			"type": "Person",
			"preferredUsername": "user",
			"name": "Example user",
			"url": "invalid-url"
		}
		`))
	require.NoError(t, err)

	actor2, err := activitypub.ToActor(item2)
	require.NoError(t, err)

	username2 := apUsername(actor2)
	assert.Equal(t, "@user@example.org", username2)
}
