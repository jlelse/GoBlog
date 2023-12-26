package main

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_AddReplyTitleAndContext(t *testing.T) {
	app := createMicropubTestEnv(t)

	bc := app.cfg.Blogs[app.cfg.DefaultBlog]
	bc.addReplyContext = true
	bc.addReplyTitle = true

	fhc := newFakeHttpClient()
	app.httpClient = fhc.Client
	fhc.setFakeResponse(http.StatusOK, `
	<!doctypehtml>
	<title>Microformats Entry Example</title>
	<article class=h-entry>
	<h1 class=p-name>My First Microformats Post</h1>
	<div class=e-content>
	<p>This is the main content of my post. It can contain <a href=#>links</a>, <strong>bold</strong> text, and more.</div>
	</article>
	`)

	err := app.createPost(&post{
		Path: "/testpost",
		Parameters: map[string][]string{
			"replylink": {"https://example.com"},
		},
		Content: "Test",
	})
	require.NoError(t, err)

	p, err := app.getPost("/testpost")
	require.NoError(t, err)

	assert.Equal(t, "My First Microformats Post", p.firstParameter("replytitle"))
	assert.Equal(t, "This is the main content of my post. It can contain links, bold text, and more.", p.firstParameter("replycontext"))
}
