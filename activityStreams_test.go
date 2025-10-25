package main

import (
	"os"
	"path/filepath"
	"testing"

	ap "github.com/go-ap/activitypub"
	"github.com/go-ap/jsonld"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_apUsername(t *testing.T) {
	item, err := ap.UnmarshalJSON([]byte(`
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

	actor, err := ap.ToActor(item)
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

	assert.Equal(t, ap.ArticleType, note.Type)
	assert.Equal(t, "Test Title", note.Name.First().String())
	assert.Contains(t, note.Content.First().String(), "Test content")
	assert.Equal(t, ap.MimeType("text/html"), note.MediaType)
	assert.True(t, note.To.Contains(ap.PublicNS))

	// JSON validation
	const expectedPublicNoteJSON = `{"@context":["https://www.w3.org/ns/activitystreams","https://w3id.org/security/v1"],"id":"https://example.com/test","type":"Article","mediaType":"text/html","name":"Test Title","content":"<div class=\"e-content\"><p>Test content</p>\n</div>","attributedTo":"https://example.com","url":"https://example.com/test","to":["https://www.w3.org/ns/activitystreams#Public","https://example.com/activitypub/followers/testblog"],"published":"2023-01-01T00:00:00Z","updated":"2023-01-02T00:00:00Z"}`
	binary, err := jsonld.WithContext(jsonld.IRI(ap.ActivityBaseURI), jsonld.IRI(ap.SecurityContextURI)).Marshal(note)
	require.NoError(t, err)
	assert.Equal(t, expectedPublicNoteJSON, string(binary))
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

	assert.Equal(t, ap.NoteType, note.Type)
	assert.True(t, note.To.Contains(ap.IRI("https://example.com/activitypub/followers/testblog")))
	assert.True(t, note.CC.Contains(ap.PublicNS))

	// JSON validation
	const expectedUnlistedNoteJSON = `{"@context":["https://www.w3.org/ns/activitystreams","https://w3id.org/security/v1"],"id":"https://example.com/test","type":"Note","mediaType":"text/html","content":"<div class=\"e-content\"><p>Test content</p>\n</div>","attributedTo":"https://example.com","url":"https://example.com/test","to":["https://example.com/activitypub/followers/testblog"],"cc":["https://www.w3.org/ns/activitystreams#Public"]}`
	binary, err := jsonld.WithContext(jsonld.IRI(ap.ActivityBaseURI), jsonld.IRI(ap.SecurityContextURI)).Marshal(note)
	require.NoError(t, err)
	assert.Equal(t, expectedUnlistedNoteJSON, string(binary))
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
	attachments, ok := note.Attachment.(ap.ItemCollection)
	require.True(t, ok)
	assert.Len(t, attachments, 2)
	for _, att := range attachments {
		obj, ok := att.(*ap.Object)
		require.True(t, ok)
		assert.Equal(t, ap.ImageType, obj.Type)
	}

	// JSON validation
	const expectedNoteWithImagesJSON = `{"@context":["https://www.w3.org/ns/activitystreams","https://w3id.org/security/v1"],"id":"https://example.com/test","type":"Note","mediaType":"text/html","content":"<div class=\"e-content\"><p>Test content</p>\n</div>","attachment":[{"type":"Image","url":"https://example.com/image1.jpg"},{"type":"Image","url":"https://example.com/image2.jpg"}],"attributedTo":"https://example.com","url":"https://example.com/test","to":["https://www.w3.org/ns/activitystreams#Public","https://example.com/activitypub/followers/testblog"]}`
	binary, err := jsonld.WithContext(jsonld.IRI(ap.ActivityBaseURI), jsonld.IRI(ap.SecurityContextURI)).Marshal(note)
	require.NoError(t, err)
	assert.Equal(t, expectedNoteWithImagesJSON, string(binary))
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
		obj, ok := tag.(*ap.Object)
		require.True(t, ok)
		assert.Equal(t, "Hashtag", string(obj.Type))
	}

	// JSON validation
	const expectedNoteWithTagsJSON = `{"@context":["https://www.w3.org/ns/activitystreams","https://w3id.org/security/v1"],"id":"https://example.com/test","type":"Note","mediaType":"text/html","content":"<div class=\"e-content\"><p>Test content</p>\n</div>","attributedTo":"https://example.com","tag":[{"type":"Hashtag","name":"tag1","url":"https://example.com/tags/tag1"},{"type":"Hashtag","name":"tag2","url":"https://example.com/tags/tag2"}],"url":"https://example.com/test","to":["https://www.w3.org/ns/activitystreams#Public","https://example.com/activitypub/followers/testblog"]}`
	binary, err := jsonld.WithContext(jsonld.IRI(ap.ActivityBaseURI), jsonld.IRI(ap.SecurityContextURI)).Marshal(note)
	require.NoError(t, err)
	assert.Equal(t, expectedNoteWithTagsJSON, string(binary))
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
		if tag.GetType() == ap.MentionType {
			mentionCount++
		}
	}
	assert.Equal(t, 2, mentionCount)

	// JSON validation
	const expectedNoteWithMentionsJSON = `{"@context":["https://www.w3.org/ns/activitystreams","https://w3id.org/security/v1"],"id":"https://example.com/test","type":"Note","mediaType":"text/html","content":"<div class=\"e-content\"><p>Test content</p>\n</div>","attributedTo":"https://example.com","tag":[{"id":"https://example.com/@user1","type":"Mention","href":"https://example.com/@user1"},{"id":"https://example.com/@user2","type":"Mention","href":"https://example.com/@user2"}],"url":"https://example.com/test","to":["https://www.w3.org/ns/activitystreams#Public","https://example.com/activitypub/followers/testblog"],"cc":["https://example.com/@user1","https://example.com/@user2"]}`
	binary, err := jsonld.WithContext(jsonld.IRI(ap.ActivityBaseURI), jsonld.IRI(ap.SecurityContextURI)).Marshal(note)
	require.NoError(t, err)
	assert.Equal(t, expectedNoteWithMentionsJSON, string(binary))
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

	assert.Equal(t, ap.IRI("https://example.com/reply-to"), note.InReplyTo)

	// JSON validation
	const expectedNoteWithReplyJSON = `{"@context":["https://www.w3.org/ns/activitystreams","https://w3id.org/security/v1"],"id":"https://example.com/test","type":"Note","mediaType":"text/html","content":"<div class=\"h-cite u-in-reply-to\"><p><strong>Reply to: <a class=\"u-url\" rel=\"noopener\" target=\"_blank\" href=\"https://example.com/reply-to\">https://example.com/reply-to</a></strong></p></div><div class=\"e-content\"><p>Test content</p>\n</div>","attributedTo":"https://example.com","inReplyTo":"https://example.com/reply-to","url":"https://example.com/test","to":["https://www.w3.org/ns/activitystreams#Public","https://example.com/activitypub/followers/testblog"]}`
	binary, err := jsonld.WithContext(jsonld.IRI(ap.ActivityBaseURI), jsonld.IRI(ap.SecurityContextURI)).Marshal(note)
	require.NoError(t, err)
	assert.Equal(t, expectedNoteWithReplyJSON, string(binary))
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
	assert.Equal(t, ap.IRI("https://example.com/test"), id)

	p.Parameters[activityPubVersionParam] = []string{"123456789"}
	id = app.activityPubId(p)
	assert.Equal(t, ap.IRI("https://example.com/test?activitypubversion=123456789"), id)
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
	assert.Equal(t, ap.IRI("https://example.com"), person.ID)
	assert.Equal(t, ap.IRI("https://example.com"), person.URL)
	assert.Equal(t, ap.IRI("https://example.com/activitypub/inbox/testblog"), person.Inbox)
	assert.Equal(t, ap.IRI("https://example.com/activitypub/followers/testblog"), person.Followers)
	assert.Len(t, person.AlsoKnownAs, 1)
	assert.Len(t, person.AttributionDomains, 1)

	// JSON validation
	const expectedPersonJSON = `{"@context":["https://www.w3.org/ns/activitystreams","https://w3id.org/security/v1"],"id":"https://example.com","type":"Person","name":"Test Blog","summary":"A test blog","url":"https://example.com","inbox":"https://example.com/activitypub/inbox/testblog","followers":"https://example.com/activitypub/followers/testblog","preferredUsername":"testblog","publicKey":{"id":"https://example.com#main-key","owner":"https://example.com","publicKeyPem":"-----BEGIN PUBLIC KEY-----\ndGVzdC1rZXk=\n-----END PUBLIC KEY-----\n"},"alsoKnownAs":["https://example.com/aka1"],"attributionDomains":["example.com"]}`
	binary, err := jsonld.WithContext(jsonld.IRI(ap.ActivityBaseURI), jsonld.IRI(ap.SecurityContextURI)).Marshal(person)
	require.NoError(t, err)
	assert.Equal(t, expectedPersonJSON, string(binary))
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
	assert.Equal(t, ap.IRI("https://example.com"), person.ID)
	assert.Equal(t, ap.IRI("https://example.com"), person.URL)
	assert.Equal(t, ap.IRI("https://example.com/activitypub/inbox/testblog"), person.Inbox)
	assert.Equal(t, ap.IRI("https://example.com/activitypub/followers/testblog"), person.Followers)
	assert.Len(t, person.AlsoKnownAs, 1)
	assert.Len(t, person.AttributionDomains, 1)
	assert.NotNil(t, person.Icon)
	iconObj, ok := person.Icon.(*ap.Object)
	require.True(t, ok)
	assert.Equal(t, ap.ImageType, iconObj.Type)
	assert.Equal(t, ap.MimeType("image/jpeg"), iconObj.MediaType)
	assert.Equal(t, ap.IRI("https://example.com/profile.jpg?v=e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"), iconObj.URL)

	// JSON validation
	const expectedPersonWithIconJSON = `{"@context":["https://www.w3.org/ns/activitystreams","https://w3id.org/security/v1"],"id":"https://example.com","type":"Person","name":"Test Blog","summary":"A test blog","icon":{"type":"Image","mediaType":"image/jpeg","url":"https://example.com/profile.jpg?v=e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},"url":"https://example.com","inbox":"https://example.com/activitypub/inbox/testblog","followers":"https://example.com/activitypub/followers/testblog","preferredUsername":"testblog","publicKey":{"id":"https://example.com#main-key","owner":"https://example.com","publicKeyPem":"-----BEGIN PUBLIC KEY-----\ndGVzdC1rZXk=\n-----END PUBLIC KEY-----\n"},"alsoKnownAs":["https://example.com/aka1"],"attributionDomains":["example.com"]}`
	binary, err := jsonld.WithContext(jsonld.IRI(ap.ActivityBaseURI), jsonld.IRI(ap.SecurityContextURI)).Marshal(person)
	require.NoError(t, err)
	assert.Equal(t, expectedPersonWithIconJSON, string(binary))
}

func Test_apUsername_EdgeCases(t *testing.T) {
	// Test with missing preferredUsername
	item, err := ap.UnmarshalJSON([]byte(`
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

	actor, err := ap.ToActor(item)
	require.NoError(t, err)

	username := apUsername(actor)
	assert.Equal(t, "https://example.org/users/user", username)

	// Test with invalid URL
	item2, err := ap.UnmarshalJSON([]byte(`
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

	actor2, err := ap.ToActor(item2)
	require.NoError(t, err)

	username2 := apUsername(actor2)
	assert.Equal(t, "@user@example.org", username2)
}
