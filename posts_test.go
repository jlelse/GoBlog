package main

import (
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_serveDate(t *testing.T) {

	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	_ = app.initConfig()
	_ = app.initDatabase(false)
	app.initComponents(false)
	app.d, _ = app.buildRouter()

	err := app.createPost(&post{
		Path:       "/testpost",
		Section:    "posts",
		Status:     "published",
		Published:  "2020-10-15T10:00:00Z",
		Parameters: map[string][]string{"title": {"Test Post"}},
		Content:    "Test Content",
	})

	require.NoError(t, err)

	req, _ := http.NewRequest(http.MethodGet, "http://localhost:8080/2020/10/15", nil)
	res, err := doHandlerRequest(req, app.d)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, res.StatusCode)

	resBody, _ := io.ReadAll(res.Body)
	resString := string(resBody)

	assert.Contains(t, resString, "Test Post")
	assert.Contains(t, resString, "<h1 class=p-name>2020-10-15</h1>")

	req, _ = http.NewRequest(http.MethodGet, "http://localhost:8080/2020/10", nil)
	res, err = doHandlerRequest(req, app.d)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, res.StatusCode)

	resBody, _ = io.ReadAll(res.Body)
	resString = string(resBody)

	assert.Contains(t, resString, "Test Post")
	assert.Contains(t, resString, "<h1 class=p-name>2020-10</h1>")

	req, _ = http.NewRequest(http.MethodGet, "http://localhost:8080/2020", nil)
	res, err = doHandlerRequest(req, app.d)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, res.StatusCode)

	resBody, _ = io.ReadAll(res.Body)
	resString = string(resBody)

	assert.Contains(t, resString, "Test Post")
	assert.Contains(t, resString, "<h1 class=p-name>2020</h1>")

	req, _ = http.NewRequest(http.MethodGet, "http://localhost:8080/x/10", nil)
	res, err = doHandlerRequest(req, app.d)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, res.StatusCode)

	resBody, _ = io.ReadAll(res.Body)
	resString = string(resBody)

	assert.Contains(t, resString, "Test Post")
	assert.Contains(t, resString, "<h1 class=p-name>XXXX-10</h1>")

	req, _ = http.NewRequest(http.MethodGet, "http://localhost:8080/x/x/15", nil)
	res, err = doHandlerRequest(req, app.d)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, res.StatusCode)

	resBody, _ = io.ReadAll(res.Body)
	resString = string(resBody)

	assert.Contains(t, resString, "Test Post")
	assert.Contains(t, resString, "<h1 class=p-name>XXXX-XX-15</h1>")

	req, _ = http.NewRequest(http.MethodGet, "http://localhost:8080/x", nil)
	res, err = doHandlerRequest(req, app.d)

	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, res.StatusCode)

}
