package main

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func Test_queue(t *testing.T) {

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

	err := db.enqueue("test", []byte(""), time.Now())
	require.Error(t, err)

	err = db.enqueue("test", []byte("1"), time.Now())
	require.NoError(t, err)

	err = db.enqueue("test", []byte("2"), time.Now())
	require.NoError(t, err)

	qi, err := db.peekQueue("abc")
	require.NoError(t, err)
	require.Nil(t, qi)

	qi, err = db.peekQueue("test")
	require.NoError(t, err)
	require.NotNil(t, qi)
	require.Equal(t, []byte("1"), qi.content)

	err = db.reschedule(qi, 1*time.Second)
	require.NoError(t, err)

	qi, err = db.peekQueue("test")
	require.NoError(t, err)
	require.NotNil(t, qi)
	require.Equal(t, []byte("2"), qi.content)

	err = db.dequeue(qi)
	require.NoError(t, err)

	qi, err = db.peekQueue("test")
	require.NoError(t, err)
	require.Nil(t, qi)

	time.Sleep(1 * time.Second)

	qi, err = db.peekQueue("test")
	require.NoError(t, err)
	require.NotNil(t, qi)
	require.Equal(t, []byte("1"), qi.content)

}
