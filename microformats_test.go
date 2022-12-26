package main

import (
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_parseMicroformats(t *testing.T) {

	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	err := app.initConfig(false)
	require.NoError(t, err)

	testHtmlBytes, err := os.ReadFile("testdata/wmtest.html")
	require.NoError(t, err)
	testHtml := string(testHtmlBytes)

	mockClient := newFakeHttpClient()
	mockClient.setFakeResponse(http.StatusOK, testHtml)

	app.httpClient = mockClient.Client

	m, err := app.parseMicroformats("https://example.net/articles/micropub-crossposting-to-twitter-and-enabling-tweetstorms", false)
	require.NoError(t, err)

	assert.Equal(t, "Micropub, Crossposting to Twitter, and Enabling “Tweetsto…", m.Title)
	assert.NotEmpty(t, m.Content)
	assert.Equal(t, "Test Blogger", m.Author)
	assert.Equal(t, "https://example.net/articles/micropub-crossposting-to-twitter-and-enabling-tweetstorms", m.Url)

}
