package main

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/gob"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.goblog.app/app/pkgs/activitypub"
)

func Test_apEnabled(t *testing.T) {
	t.Run("Enabled", func(t *testing.T) {
		app := &goBlog{
			cfg: createDefaultTestConfig(t),
		}
		app.cfg.ActivityPub = &configActivityPub{
			Enabled: true,
		}
		err := app.initConfig(false)
		require.NoError(t, err)
		assert.True(t, app.apEnabled())
	})

	t.Run("Disabled", func(t *testing.T) {
		app := &goBlog{
			cfg: createDefaultTestConfig(t),
		}
		app.cfg.ActivityPub = &configActivityPub{
			Enabled: false,
		}
		err := app.initConfig(false)
		require.NoError(t, err)
		assert.False(t, app.apEnabled())
	})

	t.Run("NilConfig", func(t *testing.T) {
		app := &goBlog{
			cfg: createDefaultTestConfig(t),
		}
		app.cfg.ActivityPub = nil
		err := app.initConfig(false)
		require.NoError(t, err)
		assert.False(t, app.apEnabled())
	})

	t.Run("PrivateMode", func(t *testing.T) {
		app := &goBlog{
			cfg: createDefaultTestConfig(t),
		}
		app.cfg.ActivityPub = &configActivityPub{
			Enabled: true,
		}
		app.cfg.PrivateMode = &configPrivateMode{
			Enabled: true,
		}
		err := app.initConfig(false)
		require.NoError(t, err)
		assert.False(t, app.apEnabled())
	})
}

func Test_apHandleWebfinger(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Server.PublicAddress = "https://example.com"
	app.cfg.Blogs = map[string]*configBlog{
		"default": {
			Path: "/",
		},
	}
	app.cfg.DefaultBlog = "default"
	app.cfg.ActivityPub = &configActivityPub{
		Enabled: true,
	}
	err := app.initConfig(false)
	require.NoError(t, err)
	_ = app.initTemplateStrings()
	app.prepareWebfinger()

	t.Run("ValidRequest", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "https://example.com/.well-known/webfinger?resource=acct:default@example.com", nil)
		rec := httptest.NewRecorder()

		app.apHandleWebfinger(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), `"subject":"acct:default@example.com"`)
		assert.Contains(t, rec.Body.String(), `"type":"application/activity+json"`)
		assert.Contains(t, rec.Body.String(), `"href":"https://example.com"`)
	})

	t.Run("MissingResource", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "https://example.com/.well-known/webfinger", nil)
		rec := httptest.NewRecorder()

		app.apHandleWebfinger(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("InvalidResource", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "https://example.com/.well-known/webfinger?resource=acct:nonexistent@example.com", nil)
		rec := httptest.NewRecorder()

		app.apHandleWebfinger(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func Test_handleWellKnownHostMeta(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "https://example.com/.well-known/host-meta", nil)
	rec := httptest.NewRecorder()

	handleWellKnownHostMeta(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/xrd+xml; charset=utf-8", rec.Header().Get(contentType))
	body := rec.Body.String()
	assert.Contains(t, body, `<XRD`)
	assert.Contains(t, body, `/.well-known/webfinger`)
}

func Test_apShowFollowers(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Server.PublicAddress = "https://example.com"
	app.cfg.Blogs = map[string]*configBlog{
		"testblog": {
			Path: "/",
		},
	}
	app.cfg.DefaultBlog = "testblog"
	app.cfg.ActivityPub = &configActivityPub{
		Enabled: true,
	}
	err := app.initConfig(false)
	require.NoError(t, err)
	_ = app.initTemplateStrings()

	// Add some test followers
	err = app.db.apAddFollower("testblog", "https://remote.example/users/alice", "https://remote.example/inbox", "@alice@remote.example")
	require.NoError(t, err)
	err = app.db.apAddFollower("testblog", "https://remote.example/users/bob", "https://remote.example/inbox", "@bob@remote.example")
	require.NoError(t, err)

	// Get followers to verify they were added
	followers, err := app.db.apGetAllFollowers("testblog")
	require.NoError(t, err)
	assert.Len(t, followers, 2)
}

func Test_apGetFollowersCollectionId(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Server.PublicAddress = "https://example.com"
	app.cfg.Blogs = map[string]*configBlog{
		"testblog": {
			Path: "/blog",
		},
	}
	err := app.initConfig(false)
	require.NoError(t, err)

	id := app.apGetFollowersCollectionId("testblog")
	assert.Equal(t, activitypub.IRI("https://example.com/activitypub/followers/testblog"), id)
}

func Test_apIri_and_apAPIri(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Server.PublicAddress = "https://example.com"
	app.cfg.Blogs = map[string]*configBlog{
		"testblog": {
			Path: "/blog",
		},
		"default": {
			Path: "/",
		},
	}
	err := app.initConfig(false)
	require.NoError(t, err)

	t.Run("BlogWithPath", func(t *testing.T) {
		blog := app.cfg.Blogs["testblog"]
		assert.Equal(t, "https://example.com/blog", app.apIri(blog))
		assert.Equal(t, activitypub.IRI("https://example.com/blog"), app.apAPIri(blog))
	})

	t.Run("BlogWithRootPath", func(t *testing.T) {
		blog := app.cfg.Blogs["default"]
		assert.Equal(t, "https://example.com", app.apIri(blog))
		assert.Equal(t, activitypub.IRI("https://example.com"), app.apAPIri(blog))
	})
}

func Test_apNewID(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Server.PublicAddress = "https://example.com"
	app.cfg.Blogs = map[string]*configBlog{
		"testblog": {
			Path: "/blog",
		},
	}
	err := app.initConfig(false)
	require.NoError(t, err)

	blog := app.cfg.Blogs["testblog"]
	id1 := app.apNewID(blog)
	id2 := app.apNewID(blog)

	// IDs should be unique
	assert.NotEqual(t, id1, id2)
	// IDs should start with the blog IRI
	assert.Contains(t, string(id1), "https://example.com/blog")
	assert.Contains(t, string(id2), "https://example.com/blog")
}

func Test_apRequestIsSuccess(t *testing.T) {
	assert.True(t, apRequestIsSuccess(200))
	assert.True(t, apRequestIsSuccess(201))
	assert.True(t, apRequestIsSuccess(202))
	assert.False(t, apRequestIsSuccess(400))
	assert.False(t, apRequestIsSuccess(401))
	assert.False(t, apRequestIsSuccess(404))
	assert.False(t, apRequestIsSuccess(500))
}

func Test_database_apFollowerFunctions(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	err := app.initConfig(false)
	require.NoError(t, err)

	t.Run("AddAndGetFollowers", func(t *testing.T) {
		// Add followers
		err := app.db.apAddFollower("blog1", "https://example1.com/users/alice", "https://example1.com/inbox", "@alice@example1.com")
		require.NoError(t, err)
		err = app.db.apAddFollower("blog1", "https://example2.com/users/bob", "https://example2.com/inbox", "@bob@example2.com")
		require.NoError(t, err)
		err = app.db.apAddFollower("blog2", "https://example3.com/users/charlie", "https://example3.com/inbox", "@charlie@example3.com")
		require.NoError(t, err)

		// Get followers for blog1
		followers, err := app.db.apGetAllFollowers("blog1")
		require.NoError(t, err)
		assert.Len(t, followers, 2)

		// Get followers for blog2
		followers, err = app.db.apGetAllFollowers("blog2")
		require.NoError(t, err)
		assert.Len(t, followers, 1)
	})

	t.Run("GetInboxes", func(t *testing.T) {
		inboxes, err := app.db.apGetAllInboxes("blog1")
		require.NoError(t, err)
		assert.Contains(t, inboxes, "https://example1.com/inbox")
		assert.Contains(t, inboxes, "https://example2.com/inbox")
	})

	t.Run("RemoveFollower", func(t *testing.T) {
		err := app.db.apRemoveFollower("blog1", "https://example1.com/users/alice")
		require.NoError(t, err)

		followers, err := app.db.apGetAllFollowers("blog1")
		require.NoError(t, err)
		assert.Len(t, followers, 1)
		assert.Equal(t, "https://example2.com/users/bob", followers[0].follower)
	})

	t.Run("RemoveInbox", func(t *testing.T) {
		err := app.db.apRemoveInbox("https://example2.com/inbox")
		require.NoError(t, err)

		inboxes, err := app.db.apGetAllInboxes("blog1")
		require.NoError(t, err)
		assert.NotContains(t, inboxes, "https://example2.com/inbox")
	})
}

func Test_apVerifySignature(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Server.PublicAddress = "https://example.com"
	app.cfg.Blogs = map[string]*configBlog{
		"testblog": {},
	}
	app.cfg.ActivityPub = &configActivityPub{
		Enabled: true,
	}
	err := app.initConfig(false)
	require.NoError(t, err)

	// Initialize ActivityPub
	app.httpClient = &http.Client{}
	err = app.initActivityPubBase()
	require.NoError(t, err)

	t.Run("MissingSignature", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "https://example.com/activitypub/inbox/testblog", nil)
		actor, err := app.apVerifySignature(req, "testblog")
		assert.Error(t, err)
		assert.Nil(t, actor)
	})
}

func Test_loadActivityPubPrivateKey(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	err := app.initConfig(false)
	require.NoError(t, err)
	require.NotNil(t, app.db)

	// Generate
	err = app.loadActivityPubPrivateKey()
	require.NoError(t, err)

	assert.NotNil(t, app.apPrivateKey)
	assert.NotEmpty(t, app.apPubKeyBytes)

	oldEncodedKey := x509.MarshalPKCS1PrivateKey(app.apPrivateKey)
	oldPemEncoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: oldEncodedKey})

	// Reset and reload
	err = app.loadActivityPubPrivateKey()
	require.NoError(t, err)

	assert.NotNil(t, app.apPrivateKey)
	assert.NotEmpty(t, app.apPubKeyBytes)

	newEncodedKey := x509.MarshalPKCS1PrivateKey(app.apPrivateKey)
	newPemEncoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: newEncodedKey})

	assert.Equal(t, string(oldPemEncoded), string(newPemEncoded))

}

func Test_webfinger(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Server.PublicAddress = "https://example.com"

	_ = app.initConfig(false)

	app.prepareWebfinger()

	req := httptest.NewRequest(http.MethodGet, "/.well-known/webfinger?resource=acct:default@example.com", nil)
	rec := httptest.NewRecorder()

	app.apHandleWebfinger(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func Test_apMoveFollowers(t *testing.T) {
	app := &goBlog{
		cfg:        createDefaultTestConfig(t),
		httpClient: newHttpClient(),
	}
	app.cfg.Server.PublicAddress = "https://example.com"
	app.cfg.Blogs = map[string]*configBlog{
		"testblog": {
			Path: "/",
		},
	}
	app.cfg.DefaultBlog = "testblog"
	app.cfg.ActivityPub = &configActivityPub{
		Enabled: true,
	}

	err := app.initConfig(false)
	require.NoError(t, err)
	err = app.initActivityPubBase()
	require.NoError(t, err)

	t.Run("BlogNotFound", func(t *testing.T) {
		err := app.apMoveFollowers("nonexistent", "https://target.example/users/new")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "blog not found")
	})

	t.Run("NoFollowers", func(t *testing.T) {
		// Create a mock server for the target account
		targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Return a valid actor with alsoKnownAs containing the blog
			actor := map[string]any{
				"@context":    "https://www.w3.org/ns/activitystreams",
				"type":        "Person",
				"id":          "https://target.example/users/new",
				"inbox":       "https://target.example/inbox",
				"alsoKnownAs": []string{"https://example.com"},
			}
			w.Header().Set("Content-Type", "application/activity+json")
			_ = json.NewEncoder(w).Encode(actor)
		}))
		defer targetServer.Close()

		// With no followers added, it should succeed but do nothing
		err := app.apMoveFollowers("testblog", targetServer.URL+"/users/new")
		// This should return nil since there are no followers (nothing to move)
		assert.NoError(t, err)
	})
}

func Test_apMoveType(t *testing.T) {
	// Test that MoveType is properly defined
	assert.Equal(t, activitypub.ActivityType("Move"), activitypub.MoveType)
}

func Test_activityWithTarget(t *testing.T) {
	// Test that Activity properly marshals/unmarshals Target field
	activity := activitypub.ActivityNew(activitypub.MoveType, activitypub.IRI("https://example.com/move/1"), activitypub.IRI("https://old.example/users/alice"))
	activity.Target = activitypub.IRI("https://new.example/users/alice")

	assert.Equal(t, activitypub.MoveType, activity.Type)
	assert.Equal(t, activitypub.IRI("https://old.example/users/alice"), activity.Object.GetLink())
	assert.Equal(t, activitypub.IRI("https://new.example/users/alice"), activity.Target.GetLink())
}

func Test_apMovedToSetting(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	err := app.initConfig(false)
	require.NoError(t, err)

	t.Run("SetAndGetMovedTo", func(t *testing.T) {
		// Initially, movedTo should be empty
		movedTo, err := app.getApMovedTo("default")
		require.NoError(t, err)
		assert.Empty(t, movedTo)

		// Set movedTo
		err = app.setApMovedTo("default", "https://newserver.example/users/newaccount")
		require.NoError(t, err)

		// Get movedTo
		movedTo, err = app.getApMovedTo("default")
		require.NoError(t, err)
		assert.Equal(t, "https://newserver.example/users/newaccount", movedTo)
	})

	t.Run("DeleteMovedTo", func(t *testing.T) {
		// Set movedTo
		err := app.setApMovedTo("default", "https://newserver.example/users/newaccount")
		require.NoError(t, err)

		// Delete movedTo
		err = app.deleteApMovedTo("default")
		require.NoError(t, err)

		// Verify it's deleted
		movedTo, err := app.getApMovedTo("default")
		require.NoError(t, err)
		assert.Empty(t, movedTo)
	})
}

func Test_apSendProfileUpdates_ObjectSet(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Server.PublicAddress = "https://example.com"
	app.cfg.DefaultBlog = "default"
	app.cfg.Blogs = map[string]*configBlog{
		"default": {
			Path:        "/",
			Title:       "Test Blog",
			Description: "A test blog",
		},
	}
	app.cfg.ActivityPub = &configActivityPub{Enabled: true}
	app.apPubKeyBytes = []byte("test-key")

	err := app.initConfig(false)
	require.NoError(t, err)
	_ = app.initTemplateStrings()

	err = app.db.apAddFollower("default", "https://remote.example/users/alice", "https://remote.example/inbox", "@alice@remote.example")
	require.NoError(t, err)

	app.apSendProfileUpdates()

	var qi *queueItem
	require.Eventually(t, func() bool {
		var err error
		qi, err = app.peekQueue(context.Background(), "ap")
		return err == nil && qi != nil
	}, time.Second, 50*time.Millisecond)

	var req apRequest
	err = gob.NewDecoder(bytes.NewReader(qi.content)).Decode(&req)
	require.NoError(t, err)

	item, err := activitypub.UnmarshalJSON(req.Activity)
	require.NoError(t, err)

	activity, err := activitypub.ToActivity(item)
	require.NoError(t, err)
	require.NotNil(t, activity.Object)
	assert.True(t, activity.Object.IsObject())

	obj, err := activitypub.ToObject(activity.Object)
	require.NoError(t, err)
	assert.Equal(t, activitypub.PersonType, obj.GetType())
	assert.Equal(t, activitypub.IRI("https://example.com"), obj.GetLink())
	assert.Equal(t, "Test Blog", obj.Name.First().String())
	assert.Equal(t, "A test blog", obj.Summary.First().String())
}

func Test_apGetFollowersCollectionIdForAddress(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Server.PublicAddress = "https://example.com"
	app.cfg.Server.AltAddresses = []string{"https://alt.example"}

	err := app.initConfig(false)
	require.NoError(t, err)

	id := app.apGetFollowersCollectionIdForAddress("default", "https://alt.example")
	assert.Equal(t, activitypub.IRI("https://alt.example/activitypub/followers/default"), id)

	id = app.apGetFollowersCollectionIdForAddress("default", "")
	assert.Equal(t, activitypub.IRI("https://example.com/activitypub/followers/default"), id)
}
