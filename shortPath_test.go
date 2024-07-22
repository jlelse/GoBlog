package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_shortenPath(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	_ = app.initConfig(false)

	db := app.db

	res1, err := db.shortenPath("/a")
	require.NoError(t, err)
	assert.Equal(t, "/s/1", res1)

	res2, err := db.shortenPath("/a")
	require.NoError(t, err)
	assert.Equal(t, "/s/1", res2)

	res3, err := db.shortenPath("/b")
	require.NoError(t, err)
	assert.Equal(t, "/s/2", res3)

	res4, err := db.shortenPath("/a")
	require.NoError(t, err)
	assert.Equal(t, "/s/1", res4)

	res5, err := db.shortenPath("/c")
	require.NoError(t, err)
	assert.Equal(t, "/s/3", res5)

	_, err = db.Exec("delete from shortpath where id = 2")
	require.NoError(t, err)

	res6, err := db.shortenPath("/d")
	require.NoError(t, err)
	assert.Equal(t, "/s/2", res6)

	_, err = db.Exec("delete from shortpath where id = 1")
	require.NoError(t, err)

	res7, err := db.shortenPath("/e")
	require.NoError(t, err)
	assert.Equal(t, "/s/1", res7)
}
