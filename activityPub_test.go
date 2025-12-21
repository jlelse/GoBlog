package main

import (
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"

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

	id := app.apGetFollowersCollectionId("testblog", app.cfg.Blogs["testblog"])
	assert.Equal(t, activitypub.IRI("https://example.com/blog/activitypub/followers/testblog"), id)
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
