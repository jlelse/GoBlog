package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_micropubQuery(t *testing.T) {

	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}

	_ = app.initConfig(false)
	_ = app.initCache()
	app.initMarkdown()
	app.initSessions()

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
			want:       "{\"channels\":[{\"name\":\"default: My Blog\",\"uid\":\"default\"},{\"name\":\"default/posts: posts\",\"uid\":\"default/posts\"}],\"media-endpoint\":\"http://localhost:8080/micropub/media\",\"visibility\":[\"public\",\"unlisted\",\"private\"]}",
			wantStatus: http.StatusOK,
		},
		{
			query:      "source&url=http://localhost:8080/test/post",
			want:       "{\"type\":[\"h-entry\"],\"properties\":{\"published\":[\"\"],\"updated\":[\"\"],\"post-status\":[\"published\"],\"visibility\":[\"public\"],\"category\":[\"test\",\"test2\"],\"content\":[\"---\\nblog: default\\npath: /test/post\\npriority: 0\\npublished: \\\"\\\"\\nsection: \\\"\\\"\\nstatus: published\\ntags:\\n    - test\\n    - test2\\nupdated: \\\"\\\"\\nvisibility: public\\n---\\nTest post\"],\"url\":[\"http://localhost:8080/test/post\"],\"mp-slug\":[\"\"],\"mp-channel\":[\"default\"]}}",
			wantStatus: http.StatusOK,
		},
		{
			query:      "source",
			want:       "{\"items\":[{\"type\":[\"h-entry\"],\"properties\":{\"published\":[\"\"],\"updated\":[\"\"],\"post-status\":[\"published\"],\"visibility\":[\"public\"],\"category\":[\"test\",\"test2\"],\"content\":[\"---\\nblog: default\\npath: /test/post\\npriority: 0\\npublished: \\\"\\\"\\nsection: \\\"\\\"\\nstatus: published\\ntags:\\n    - test\\n    - test2\\nupdated: \\\"\\\"\\nvisibility: public\\n---\\nTest post\"],\"url\":[\"http://localhost:8080/test/post\"],\"mp-slug\":[\"\"],\"mp-channel\":[\"default\"]}}]}",
			wantStatus: http.StatusOK,
		},
		{
			query:      "category",
			want:       "{\"categories\":[\"test\",\"test2\"]}",
			wantStatus: http.StatusOK,
		},
		{
			query:      "channel",
			want:       "{\"channels\":[{\"name\":\"default: My Blog\",\"uid\":\"default\"},{\"name\":\"default/posts: posts\",\"uid\":\"default/posts\"}]}",
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

		app.serveMicropubQuery(rec, req)
		rec.Flush()

		assert.Equal(t, tc.wantStatus, rec.Code)
		if tc.want != "" {
			assert.Equal(t, tc.want, rec.Body.String())
		}
	}

}
