package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.goblog.app/app/pkgs/contenttype"
)

func Test_blogStats(t *testing.T) {

	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}

	app.cfg.Blogs = map[string]*configBlog{
		"en": {
			Lang: "en",
			BlogStats: &configBlogStats{
				Enabled: true,
				Path:    "/stats",
			},
			Sections: map[string]*configSection{
				"test": {},
			},
		},
	}
	app.cfg.DefaultBlog = "en"

	_ = app.initConfig(false)
	_ = app.initTemplateStrings()

	// Insert post

	err := app.createPost(&post{
		Content:    "This is a simple **test** post",
		Blog:       "en",
		Section:    "test",
		Published:  "2020-06-01",
		Status:     statusPublished,
		Visibility: visibilityPublic,
	})
	require.NoError(t, err)

	err = app.createPost(&post{
		Content:    "This is another simple **test** post",
		Blog:       "en",
		Section:    "test",
		Published:  "2021-05-01",
		Status:     statusPublished,
		Visibility: visibilityPublic,
	})
	require.NoError(t, err)

	err = app.createPost(&post{
		Content:    "This is a private post, that doesn't count",
		Blog:       "en",
		Section:    "test",
		Published:  "2021-05-01",
		Status:     statusPublished,
		Visibility: visibilityPrivate,
	})
	require.NoError(t, err)

	err = app.createPost(&post{
		Content:    "Unlisted posts don't count as well",
		Blog:       "en",
		Section:    "test",
		Published:  "2021-05-01",
		Status:     statusPublished,
		Visibility: visibilityUnlisted,
	})
	require.NoError(t, err)

	// Test stats

	sd, err := app.db.getBlogStats("en")
	require.NoError(t, err)
	require.NotNil(t, sd)

	require.NotNil(t, sd.Total)
	assert.Equal(t, "2", sd.Total.Posts)
	assert.Equal(t, "12", sd.Total.Words)
	assert.Equal(t, "48", sd.Total.Chars)

	assert.Equal(t, "0", sd.NoDate.Posts)

	// 2021
	require.NotNil(t, sd.Years)
	row := sd.Years[0]
	require.NotNil(t, row)
	assert.Equal(t, "2021", row.Name)
	assert.Equal(t, "1", row.Posts)
	assert.Equal(t, "6", row.Words)
	assert.Equal(t, "27", row.Chars)

	// 2021-05
	require.NotNil(t, sd.Months)
	require.NotEmpty(t, sd.Months["2021"])
	row = sd.Months["2021"][0]
	require.NotNil(t, row)
	assert.Equal(t, "05", row.Name)
	assert.Equal(t, "1", row.Posts)
	assert.Equal(t, "6", row.Words)
	assert.Equal(t, "27", row.Chars)

	// 2020
	require.NotNil(t, sd.Years)
	row = sd.Years[1]
	require.NotNil(t, row)
	assert.Equal(t, "2020", row.Name)
	assert.Equal(t, "1", row.Posts)
	assert.Equal(t, "6", row.Words)
	assert.Equal(t, "21", row.Chars)

	// Test HTML

	t.Run("Test stats page", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/abc", nil)
		req = req.WithContext(context.WithValue(req.Context(), blogKey, "en"))

		rec := httptest.NewRecorder()

		app.serveBlogStats(rec, req)

		res := rec.Result()
		resBody, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		resString := string(resBody)

		assert.Equal(t, http.StatusOK, res.StatusCode)
		assert.Contains(t, resString, "class=statsyear data-year=2021")
		assert.Contains(t, res.Header.Get(contentType), contenttype.HTML)
	})

}

func Test_blogStatsNoDate(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Blogs = map[string]*configBlog{
		"en": {
			Lang: "en",
			BlogStats: &configBlogStats{
				Enabled: true,
			},
			Sections: map[string]*configSection{
				"test": {},
			},
		},
	}
	app.cfg.DefaultBlog = "en"
	_ = app.initConfig(false)

	err := app.db.savePost(&post{
		Path:       "/test/nodate",
		Content:    "Post without a date",
		Blog:       "en",
		Section:    "test",
		Status:     statusPublished,
		Visibility: visibilityPublic,
	}, &postCreationOptions{new: true})
	require.NoError(t, err)

	sd, err := app.db.getBlogStats("en")
	require.NoError(t, err)
	require.NotNil(t, sd)

	assert.Equal(t, "1", sd.Total.Posts)
	assert.Equal(t, "4", sd.Total.Words)
	assert.Equal(t, "1", sd.NoDate.Posts)
	assert.Equal(t, "4", sd.NoDate.Words)
	assert.Empty(t, sd.Years)
}

func Test_blogStatsMultipleMonths(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Blogs = map[string]*configBlog{
		"en": {
			Lang: "en",
			BlogStats: &configBlogStats{
				Enabled: true,
			},
			Sections: map[string]*configSection{
				"test": {},
			},
		},
	}
	app.cfg.DefaultBlog = "en"
	_ = app.initConfig(false)

	for _, date := range []string{"2023-01-15", "2023-03-20", "2023-03-25"} {
		err := app.createPost(&post{
			Content:    "Test content for stats",
			Blog:       "en",
			Section:    "test",
			Published:  date,
			Status:     statusPublished,
			Visibility: visibilityPublic,
		})
		require.NoError(t, err)
	}

	sd, err := app.db.getBlogStats("en")
	require.NoError(t, err)
	require.NotNil(t, sd)

	assert.Equal(t, "3", sd.Total.Posts)
	require.Len(t, sd.Years, 1)
	assert.Equal(t, "2023", sd.Years[0].Name)
	assert.Equal(t, "3", sd.Years[0].Posts)

	require.Len(t, sd.Months["2023"], 2)
	assert.Equal(t, "03", sd.Months["2023"][0].Name)
	assert.Equal(t, "2", sd.Months["2023"][0].Posts)
	assert.Equal(t, "01", sd.Months["2023"][1].Name)
	assert.Equal(t, "1", sd.Months["2023"][1].Posts)
}

func Test_blogStatsEmptyBlog(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Blogs = map[string]*configBlog{
		"en": {
			Lang: "en",
			BlogStats: &configBlogStats{
				Enabled: true,
			},
			Sections: map[string]*configSection{
				"test": {},
			},
		},
	}
	app.cfg.DefaultBlog = "en"
	_ = app.initConfig(false)

	sd, err := app.db.getBlogStats("en")
	require.NoError(t, err)
	require.NotNil(t, sd)

	assert.Empty(t, sd.Years)
	assert.Empty(t, sd.Months)
	assert.Equal(t, "0", sd.Total.Posts)
	assert.Equal(t, "0", sd.NoDate.Posts)
}

func Test_blogStatsWordsPerPost(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Blogs = map[string]*configBlog{
		"en": {
			Lang: "en",
			BlogStats: &configBlogStats{
				Enabled: true,
			},
			Sections: map[string]*configSection{
				"test": {},
			},
		},
	}
	app.cfg.DefaultBlog = "en"
	_ = app.initConfig(false)

	err := app.createPost(&post{
		Content:    "one two three four",
		Blog:       "en",
		Section:    "test",
		Published:  "2023-06-01",
		Status:     statusPublished,
		Visibility: visibilityPublic,
	})
	require.NoError(t, err)

	err = app.createPost(&post{
		Content:    "five six",
		Blog:       "en",
		Section:    "test",
		Published:  "2023-06-15",
		Status:     statusPublished,
		Visibility: visibilityPublic,
	})
	require.NoError(t, err)

	sd, err := app.db.getBlogStats("en")
	require.NoError(t, err)

	assert.Equal(t, "2", sd.Total.Posts)
	assert.Equal(t, "6", sd.Total.Words)
	assert.Equal(t, "3", sd.Total.WordsPerPost)
}

func createBlogStatsTestApp(b *testing.B, numPosts int) *goBlog {
	b.Helper()
	app := &goBlog{
		cfg: createDefaultTestConfig(b),
	}
	app.cfg.Blogs = map[string]*configBlog{
		"en": {
			Lang: "en",
			BlogStats: &configBlogStats{
				Enabled: true,
			},
			Sections: map[string]*configSection{
				"test": {},
			},
		},
	}
	app.cfg.DefaultBlog = "en"
	_ = app.initConfig(false)

	for i := range numPosts {
		year := 2020 + (i / 120)
		month := (i%12 + 1)
		day := (i%28 + 1)
		err := app.createPost(&post{
			Content:    fmt.Sprintf("This is test post number %d with some content to count words and characters for benchmarking.", i),
			Blog:       "en",
			Section:    "test",
			Published:  fmt.Sprintf("%04d-%02d-%02d", year, month, day),
			Status:     statusPublished,
			Visibility: visibilityPublic,
		})
		if err != nil {
			b.Fatal(err)
		}
	}

	return app
}

func Benchmark_blogStats(b *testing.B) {
	for _, n := range []int{10, 100, 500, 1500} {
		b.Run(fmt.Sprintf("posts=%d", n), func(b *testing.B) {
			app := createBlogStatsTestApp(b, n)
			b.ResetTimer()
			for b.Loop() {
				_, err := app.db.getBlogStats("en")
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
