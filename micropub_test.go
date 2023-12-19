package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/carlmjohnson/requests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.goblog.app/app/pkgs/contenttype"
	"go.hacdias.com/indielib/micropub"
)

func Test_micropubQuery(t *testing.T) {

	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	_ = app.initConfig(false)

	// Create a test post with tags
	err := app.createPost(&post{
		Path:    "/test/post",
		Content: "Test post",
		Parameters: map[string][]string{
			"tags": {"test", "test2"},
		},
	})
	require.NoError(t, err)

	type testCase struct {
		query      string
		want       string
		wantStatus int
	}

	testCases := []testCase{
		{
			query:      "config",
			want:       "{\"categories\":[\"test\",\"test2\"],\"channels\":[{\"uid\":\"default\",\"name\":\"default: My Blog\"},{\"uid\":\"default/posts\",\"name\":\"default/posts: posts\"}],\"media-endpoint\":\"http://localhost:8080/micropub/media\",\"visibility\":[\"private\",\"unlisted\",\"public\"]}\n",
			wantStatus: http.StatusOK,
		},
		{
			query:      "source&url=http://localhost:8080/test/post",
			want:       "{\"properties\":{\"category\":[\"test\",\"test2\"],\"content\":[\"---\\nblog: default\\npath: /test/post\\npriority: 0\\npublished: \\\"\\\"\\nsection: \\\"\\\"\\nstatus: published\\ntags:\\n    - test\\n    - test2\\nupdated: \\\"\\\"\\nvisibility: public\\n---\\nTest post\"],\"mp-channel\":[\"default\"],\"mp-slug\":[\"\"],\"post-status\":[\"published\"],\"published\":[\"\"],\"updated\":[\"\"],\"url\":[\"http://localhost:8080/test/post\"],\"visibility\":[\"public\"]},\"type\":[\"h-entry\"]}\n",
			wantStatus: http.StatusOK,
		},
		{
			query:      "source",
			want:       "{\"items\":[{\"properties\":{\"category\":[\"test\",\"test2\"],\"content\":[\"---\\nblog: default\\npath: /test/post\\npriority: 0\\npublished: \\\"\\\"\\nsection: \\\"\\\"\\nstatus: published\\ntags:\\n    - test\\n    - test2\\nupdated: \\\"\\\"\\nvisibility: public\\n---\\nTest post\"],\"mp-channel\":[\"default\"],\"mp-slug\":[\"\"],\"post-status\":[\"published\"],\"published\":[\"\"],\"updated\":[\"\"],\"url\":[\"http://localhost:8080/test/post\"],\"visibility\":[\"public\"]},\"type\":[\"h-entry\"]}]}\n",
			wantStatus: http.StatusOK,
		},
		{
			query:      "category",
			want:       "{\"categories\":[\"test\",\"test2\"]}\n",
			wantStatus: http.StatusOK,
		},
		{
			query:      "channel",
			want:       "{\"channels\":[{\"uid\":\"default\",\"name\":\"default: My Blog\"},{\"uid\":\"default/posts\",\"name\":\"default/posts: posts\"}]}\n",
			wantStatus: http.StatusOK,
		},
		{
			query:      "somethingelse",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tc := range testCases {
		req := httptest.NewRequest(http.MethodGet, "http://localhost:8080/micropub?q="+tc.query, nil)
		rec := httptest.NewRecorder()

		app.getMicropubImplementation().getHandler().ServeHTTP(rec, req)
		rec.Flush()

		assert.Equal(t, tc.wantStatus, rec.Code)
		if tc.want != "" {
			assert.Equal(t, tc.want, rec.Body.String())
		}
	}

}

func Test_micropubCreate(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	_ = app.initConfig(false)

	// Modify settings for easier testing
	bc := app.cfg.Blogs[app.cfg.DefaultBlog]
	sc := bc.Sections[bc.DefaultSection]
	sc.PathTemplate = `{{printf "/%v" .Slug}}`

	handler := addAllScopes(app.getMicropubImplementation().getHandler())

	t.Run("Normal (JSON)", func(t *testing.T) {
		postPath := "/create1"
		reqBody := `{"type":["h-entry"],"properties":{"content":["Test post"],"mp-slug":["create1"],"category":["Cool"]}}`

		recorder := httptest.NewRecorder()
		req, _ := requests.New().Post().BodyReader(strings.NewReader(reqBody)).ContentType(contenttype.JSONUTF8).Request(context.Background())
		handler.ServeHTTP(recorder, req)

		result := recorder.Result()
		require.Equal(t, http.StatusAccepted, result.StatusCode)
		require.Equal(t, "http://localhost:8080"+postPath, result.Header.Get("Location"))

		p, err := app.getPost(postPath)
		require.NoError(t, err)

		assert.Equal(t, "Test post", p.Content)
		assert.Equal(t, []string{"Cool"}, p.Parameters["tags"])
	})

	t.Run("Photo (JSON)", func(t *testing.T) {
		postPath := "/create2"
		reqBody := `{"type":["h-entry"],"properties":{"mp-slug":["create2"],"photo":["https://photos.example.com/123.jpg"]}}`

		recorder := httptest.NewRecorder()
		req, _ := requests.New().Post().BodyReader(strings.NewReader(reqBody)).ContentType(contenttype.JSONUTF8).Request(context.Background())
		handler.ServeHTTP(recorder, req)

		result := recorder.Result()
		require.Equal(t, http.StatusAccepted, result.StatusCode)
		require.Equal(t, "http://localhost:8080"+postPath, result.Header.Get("Location"))

		p, err := app.getPost(postPath)
		require.NoError(t, err)

		assert.Equal(t, "\n![](https://photos.example.com/123.jpg)", p.Content)
		assert.Equal(t, []string{"https://photos.example.com/123.jpg"}, p.Parameters["images"])
	})

	t.Run("Photo with alternative text (JSON)", func(t *testing.T) {
		postPath := "/create3"
		reqBody := `{"type":["h-entry"],"properties":{"mp-slug":["create3"],"photo":[{
			"value": "https://photos.example.com/123.jpg",
			"alt": "This is a photo"
		  }]}}`

		recorder := httptest.NewRecorder()
		req, _ := requests.New().Post().BodyReader(strings.NewReader(reqBody)).ContentType(contenttype.JSONUTF8).Request(context.Background())
		handler.ServeHTTP(recorder, req)

		result := recorder.Result()
		require.Equal(t, http.StatusAccepted, result.StatusCode)
		require.Equal(t, "http://localhost:8080"+postPath, result.Header.Get("Location"))

		p, err := app.getPost(postPath)
		require.NoError(t, err)

		assert.Equal(t, "\n![This is a photo](https://photos.example.com/123.jpg \"This is a photo\")", p.Content)
		assert.Equal(t, []string{"https://photos.example.com/123.jpg"}, p.Parameters["images"])
		assert.Equal(t, []string{"This is a photo"}, p.Parameters["imagealts"])
	})

	t.Run("Photo with alternative text (Form)", func(t *testing.T) {
		postPath := "/create4"

		bodyForm := url.Values{}
		bodyForm["h"] = []string{"entry"}
		bodyForm["mp-slug"] = []string{"create4"}
		bodyForm["photo"] = []string{"https://photos.example.com/123.jpg"}
		bodyForm["mp-photo-alt"] = []string{"This is a photo"}

		recorder := httptest.NewRecorder()
		req, _ := requests.New().Post().BodyForm(bodyForm).Request(context.Background())
		handler.ServeHTTP(recorder, req)

		result := recorder.Result()
		require.Equal(t, http.StatusAccepted, result.StatusCode)
		require.Equal(t, "http://localhost:8080"+postPath, result.Header.Get("Location"))

		p, err := app.getPost(postPath)
		require.NoError(t, err)

		assert.Equal(t, "\n![This is a photo](https://photos.example.com/123.jpg \"This is a photo\")", p.Content)
		assert.Equal(t, []string{"https://photos.example.com/123.jpg"}, p.Parameters["images"])
		assert.Equal(t, []string{"This is a photo"}, p.Parameters["imagealts"])
	})

	t.Run("Custom parameter (Form)", func(t *testing.T) {
		postPath := "/create5"

		bodyForm := url.Values{}
		bodyForm["h"] = []string{"entry"}
		bodyForm["mp-slug"] = []string{"create5"}
		bodyForm["random"] = []string{"Abc", "Def"}

		recorder := httptest.NewRecorder()
		req, _ := requests.New().Post().BodyForm(bodyForm).Request(context.Background())
		handler.ServeHTTP(recorder, req)

		result := recorder.Result()
		require.Equal(t, http.StatusAccepted, result.StatusCode)
		require.Equal(t, "http://localhost:8080"+postPath, result.Header.Get("Location"))

		p, err := app.getPost(postPath)
		require.NoError(t, err)

		assert.Equal(t, []string{"Abc", "Def"}, p.Parameters["random"])
	})
}

func Test_micropubUpdate(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	_ = app.initConfig(false)

	t.Run("Delete", func(t *testing.T) {
		postPath := "/delete1"

		err := app.createPost(&post{
			Path:    postPath,
			Content: "Test post",
			Parameters: map[string][]string{
				"tags":   {"test", "test2"},
				"random": {"abc", "def"},
			},
		})
		require.NoError(t, err)

		_, err = app.getMicropubImplementation().Update(&micropub.Request{
			URL: app.cfg.Server.PublicAddress + postPath,
			Updates: micropub.RequestUpdate{
				Delete: map[string]any{
					// Should filter the parameters
					"category": []any{"test"},
					// Should do nothing, because it's a custom parameter and thus ignored
					"random": []any{"def"},
					// Should do nothing
					"content": []any{},
				},
			},
		})
		require.NoError(t, err)

		p, err := app.getPost(postPath)
		require.NoError(t, err)

		assert.Equal(t, []string{"test2"}, p.Parameters["tags"])
		assert.Equal(t, []string{"abc", "def"}, p.Parameters["random"])
		assert.Equal(t, "Test post", p.Content)
	})

	t.Run("Replace", func(t *testing.T) {
		postPath := "/replace1"

		err := app.createPost(&post{
			Path:       postPath,
			Content:    "Test post",
			Status:     statusPublished,
			Visibility: visibilityPublic,
			Parameters: map[string][]string{
				"tags":   {"test", "test2"},
				"random": {"abc", "def"},
			},
		})
		require.NoError(t, err)

		_, err = app.getMicropubImplementation().Update(&micropub.Request{
			URL: app.cfg.Server.PublicAddress + postPath,
			Updates: micropub.RequestUpdate{
				Replace: map[string][]any{
					"category":    {"test"},
					"random":      {"def"},
					"content":     {"New test"},
					"post-status": {"published-deleted"},
					"visibility":  {"unlisted"},
				},
			},
		})
		require.NoError(t, err)

		p, err := app.getPost(postPath)
		require.NoError(t, err)

		assert.Equal(t, []string{"test"}, p.Parameters["tags"])
		assert.Equal(t, []string{"def"}, p.Parameters["random"])
		assert.Equal(t, "New test", p.Content)
		assert.Equal(t, statusPublished, p.Status)
		assert.Nil(t, p.Parameters["post-status"])
		assert.Equal(t, visibilityUnlisted, p.Visibility)
	})

	t.Run("Add", func(t *testing.T) {
		postPath := "/add1"

		err := app.createPost(&post{
			Path:    postPath,
			Content: "Test post",
			Parameters: map[string][]string{
				"tags": {"test"},
			},
		})
		require.NoError(t, err)

		_, err = app.getMicropubImplementation().Update(&micropub.Request{
			URL: app.cfg.Server.PublicAddress + postPath,
			Updates: micropub.RequestUpdate{
				Add: map[string][]any{
					"category":    {"test2"},
					"random":      {"abc"},
					"content":     {"Add", "Bbb"},
					"post-status": {"published"},
				},
			},
		})
		require.NoError(t, err)

		p, err := app.getPost(postPath)
		require.NoError(t, err)

		assert.Equal(t, []string{"test", "test2"}, p.Parameters["tags"])
		assert.Equal(t, []string{"abc"}, p.Parameters["random"])
		assert.Equal(t, "Test post", p.Content)
		assert.Nil(t, p.Parameters["post-status"])
	})
}
