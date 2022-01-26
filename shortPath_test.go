package main

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_shortenPath(t *testing.T) {
	app := &goBlog{
		cfg: &config{
			Db: &configDb{
				File: filepath.Join(t.TempDir(), "test.db"),
			},
		},
	}
	_ = app.initDatabase(false)
	defer app.db.close()
	db := app.db

	res1, err := db.shortenPath("/a")
	require.NoError(t, err)

	db.spc.Delete("/a")

	res2, err := db.shortenPath("/a")
	require.NoError(t, err)

	res3, err := db.shortenPath("/b")
	require.NoError(t, err)

	assert.Equal(t, res1, res2)
	assert.Equal(t, "/s/1", res1)

	assert.NotEqual(t, res1, res3)
	assert.Equal(t, "/s/2", res3)

	db.spc.Delete("/a")
	_, _ = db.exec("delete from shortpath where id = 1")

	res4, err := db.shortenPath("/c")
	require.NoError(t, err)
	assert.Equal(t, "/s/1", res4)
}
