package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_apAddFollowerManually(t *testing.T) {
	app := &goBlog{
		cfg:        createDefaultTestConfig(t),
		httpClient: newHttpClient(),
	}
	app.cfg.Server.PublicAddress = "https://example.com"
	app.cfg.Blogs = map[string]*configBlog{
		"testblog": {Path: "/"},
	}
	app.cfg.DefaultBlog = "testblog"
	app.cfg.ActivityPub = &configActivityPub{Enabled: true}

	err := app.initConfig(false)
	require.NoError(t, err)
	err = app.initActivityPubBase()
	require.NoError(t, err)

	t.Run("BlogNotFound", func(t *testing.T) {
		err := app.apAddFollowerManually("nonexistent", "https://remote.example/users/alice")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "blog not found")
	})

	t.Run("InvalidHandleFormat", func(t *testing.T) {
		err := app.apAddFollowerManually("testblog", "@onlyuser")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid handle format")
	})

	t.Run("AddByActorIRI", func(t *testing.T) {
		// Create a test server returning a valid actor
		var actorServerURL string
		actorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actor := map[string]any{
				"@context":          "https://www.w3.org/ns/activitystreams",
				"type":              "Person",
				"id":                actorServerURL + r.URL.String(),
				"inbox":             actorServerURL + "/inbox",
				"preferredUsername": "alice",
			}
			w.Header().Set("Content-Type", "application/activity+json")
			_ = json.NewEncoder(w).Encode(actor)
		}))
		actorServerURL = actorServer.URL
		defer actorServer.Close()

		err := app.apAddFollowerManually("testblog", actorServer.URL+"/users/alice")
		require.NoError(t, err)

		// Verify follower was added
		followers, err := app.db.apGetAllFollowers("testblog")
		require.NoError(t, err)
		found := false
		for _, f := range followers {
			if f.follower == actorServer.URL+"/users/alice" {
				found = true
				assert.Contains(t, f.username, "alice")
				break
			}
		}
		assert.True(t, found, "Follower should be in the database")

		// Cleanup
		_ = app.db.apRemoveFollower("testblog", actorServer.URL+"/users/alice")
	})

	t.Run("AddByActorIRIWithSharedInbox", func(t *testing.T) {
		var actorServerURL string
		actorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actor := map[string]any{
				"@context":          "https://www.w3.org/ns/activitystreams",
				"type":              "Person",
				"id":                actorServerURL + r.URL.String(),
				"inbox":             actorServerURL + "/users/bob/inbox",
				"preferredUsername": "bob",
				"endpoints": map[string]string{
					"sharedInbox": actorServerURL + "/inbox",
				},
			}
			w.Header().Set("Content-Type", "application/activity+json")
			_ = json.NewEncoder(w).Encode(actor)
		}))
		actorServerURL = actorServer.URL
		defer actorServer.Close()

		err := app.apAddFollowerManually("testblog", actorServer.URL+"/users/bob")
		require.NoError(t, err)

		// Verify follower was added with shared inbox
		inboxes, err := app.db.apGetAllInboxes("testblog")
		require.NoError(t, err)
		assert.Contains(t, inboxes, actorServer.URL+"/inbox")

		// Cleanup
		_ = app.db.apRemoveFollower("testblog", actorServer.URL+"/users/bob")
	})

	t.Run("ActorNotFound", func(t *testing.T) {
		goneServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer goneServer.Close()

		err := app.apAddFollowerManually("testblog", goneServer.URL+"/users/nobody")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to fetch remote actor")
	})

	t.Run("ActorWithNoInbox", func(t *testing.T) {
		var actorServerURL string
		actorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actor := map[string]any{
				"@context":          "https://www.w3.org/ns/activitystreams",
				"type":              "Person",
				"id":                actorServerURL + r.URL.String(),
				"preferredUsername": "noinbox",
			}
			w.Header().Set("Content-Type", "application/activity+json")
			_ = json.NewEncoder(w).Encode(actor)
		}))
		actorServerURL = actorServer.URL
		defer actorServer.Close()

		err := app.apAddFollowerManually("testblog", actorServer.URL+"/users/noinbox")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "has no inbox")
	})

	t.Run("AddDuplicateUpdates", func(t *testing.T) {
		var actorServerURL string
		actorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actor := map[string]any{
				"@context":          "https://www.w3.org/ns/activitystreams",
				"type":              "Person",
				"id":                actorServerURL + r.URL.String(),
				"inbox":             actorServerURL + "/inbox",
				"preferredUsername": "charlie",
			}
			w.Header().Set("Content-Type", "application/activity+json")
			_ = json.NewEncoder(w).Encode(actor)
		}))
		actorServerURL = actorServer.URL
		defer actorServer.Close()

		// Add once
		err := app.apAddFollowerManually("testblog", actorServer.URL+"/users/charlie")
		require.NoError(t, err)

		// Add again — should update (upsert), not error
		err = app.apAddFollowerManually("testblog", actorServer.URL+"/users/charlie")
		require.NoError(t, err)

		// Should still be just one entry
		followers, err := app.db.apGetAllFollowers("testblog")
		require.NoError(t, err)
		count := 0
		for _, f := range followers {
			if f.follower == actorServer.URL+"/users/charlie" {
				count++
			}
		}
		assert.Equal(t, 1, count)

		// Cleanup
		_ = app.db.apRemoveFollower("testblog", actorServer.URL+"/users/charlie")
	})
}

func Test_apCheckFollowers(t *testing.T) {
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
		results, err := app.apCheckFollowers("nonexistent")
		assert.Error(t, err)
		assert.Nil(t, results)
		assert.Contains(t, err.Error(), "blog not found")
	})

	t.Run("NoFollowers", func(t *testing.T) {
		results, err := app.apCheckFollowers("testblog")
		assert.NoError(t, err)
		assert.Nil(t, results)
	})

	t.Run("ActiveFollower", func(t *testing.T) {
		// Create a test server returning a valid actor
		activeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actor := map[string]any{
				"@context":          "https://www.w3.org/ns/activitystreams",
				"type":              "Person",
				"id":                r.URL.String(),
				"inbox":             "https://active.example/inbox",
				"preferredUsername": "active",
			}
			w.Header().Set("Content-Type", "application/activity+json")
			_ = json.NewEncoder(w).Encode(actor)
		}))
		defer activeServer.Close()

		err := app.db.apAddFollower("testblog", activeServer.URL+"/users/active", "https://active.example/inbox", "@active@active.example")
		require.NoError(t, err)
		defer app.db.apRemoveFollower("testblog", activeServer.URL+"/users/active")

		results, err := app.apCheckFollowers("testblog")
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, "ok", results[0].status)
	})

	t.Run("GoneFollower", func(t *testing.T) {
		// Create a test server that returns 404/410
		goneServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusGone)
		}))
		defer goneServer.Close()

		err := app.db.apAddFollower("testblog", goneServer.URL+"/users/gone", "https://gone.example/inbox", "@gone@gone.example")
		require.NoError(t, err)
		defer app.db.apRemoveFollower("testblog", goneServer.URL+"/users/gone")

		results, err := app.apCheckFollowers("testblog")
		require.NoError(t, err)

		var goneResult *apFollowerCheckResult
		for _, r := range results {
			if r.follower.follower == goneServer.URL+"/users/gone" {
				goneResult = r
				break
			}
		}
		require.NotNil(t, goneResult)
		assert.Equal(t, "gone", goneResult.status)
		assert.NotNil(t, goneResult.err)
	})

	t.Run("MovedFollower", func(t *testing.T) {
		// Create a test server that returns a moved actor
		movedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actor := map[string]any{
				"@context":          "https://www.w3.org/ns/activitystreams",
				"type":              "Person",
				"id":                r.URL.String(),
				"inbox":             "https://moved.example/inbox",
				"preferredUsername": "moved",
				"movedTo":           "https://newserver.example/users/moved",
			}
			w.Header().Set("Content-Type", "application/activity+json")
			_ = json.NewEncoder(w).Encode(actor)
		}))
		defer movedServer.Close()

		err := app.db.apAddFollower("testblog", movedServer.URL+"/users/moved", "https://moved.example/inbox", "@moved@moved.example")
		require.NoError(t, err)
		defer app.db.apRemoveFollower("testblog", movedServer.URL+"/users/moved")

		results, err := app.apCheckFollowers("testblog")
		require.NoError(t, err)

		var movedResult *apFollowerCheckResult
		for _, r := range results {
			if r.follower.follower == movedServer.URL+"/users/moved" {
				movedResult = r
				break
			}
		}
		require.NotNil(t, movedResult)
		assert.Equal(t, "moved", movedResult.status)
		assert.Equal(t, "https://newserver.example/users/moved", movedResult.movedTo)
	})

	t.Run("MixedFollowers", func(t *testing.T) {
		// Active follower server
		activeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actor := map[string]any{
				"@context":          "https://www.w3.org/ns/activitystreams",
				"type":              "Person",
				"id":                r.URL.String(),
				"inbox":             "https://active2.example/inbox",
				"preferredUsername": "active2",
			}
			w.Header().Set("Content-Type", "application/activity+json")
			_ = json.NewEncoder(w).Encode(actor)
		}))
		defer activeServer.Close()

		// Gone follower server
		goneServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusGone)
		}))
		defer goneServer.Close()

		// Moved follower server
		movedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actor := map[string]any{
				"@context":          "https://www.w3.org/ns/activitystreams",
				"type":              "Person",
				"id":                r.URL.String(),
				"inbox":             "https://moved2.example/inbox",
				"preferredUsername": "moved2",
				"movedTo":           "https://newplace.example/users/moved2",
			}
			w.Header().Set("Content-Type", "application/activity+json")
			_ = json.NewEncoder(w).Encode(actor)
		}))
		defer movedServer.Close()

		// Add all three followers
		err := app.db.apAddFollower("testblog", activeServer.URL+"/users/active2", activeServer.URL+"/inbox", "@active2@active2.example")
		require.NoError(t, err)
		err = app.db.apAddFollower("testblog", goneServer.URL+"/users/gone2", goneServer.URL+"/inbox", "@gone2@gone2.example")
		require.NoError(t, err)
		err = app.db.apAddFollower("testblog", movedServer.URL+"/users/moved2", movedServer.URL+"/inbox", "@moved2@moved2.example")
		require.NoError(t, err)
		defer func() {
			_ = app.db.apRemoveFollower("testblog", activeServer.URL+"/users/active2")
			_ = app.db.apRemoveFollower("testblog", goneServer.URL+"/users/gone2")
			_ = app.db.apRemoveFollower("testblog", movedServer.URL+"/users/moved2")
		}()

		results, err := app.apCheckFollowers("testblog")
		require.NoError(t, err)
		require.Len(t, results, 3)

		statusCounts := map[string]int{}
		for _, r := range results {
			statusCounts[r.status]++
		}
		assert.Equal(t, 1, statusCounts["ok"])
		assert.Equal(t, 1, statusCounts["gone"])
		assert.Equal(t, 1, statusCounts["moved"])
	})
}
