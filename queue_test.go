package main

import (
	"context"
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

	time1 := time.Now()

	err := db.enqueue("test", []byte(""), time.Now())
	require.Error(t, err)

	err = db.enqueue("test", []byte("1"), time1)
	require.NoError(t, err)

	err = db.enqueue("test", []byte("2"), time.Now())
	require.NoError(t, err)

	qi, err := db.peekQueue(context.Background(), "abc")
	require.NoError(t, err)
	require.Nil(t, qi)

	qi, err = db.peekQueue(context.Background(), "test")
	require.NoError(t, err)
	require.NotNil(t, qi)
	require.Equal(t, []byte("1"), qi.content)
	require.Equal(t, time1.UTC(), qi.schedule.UTC())

	err = db.reschedule(qi, 1*time.Second)
	require.NoError(t, err)

	qi, err = db.peekQueue(context.Background(), "test")
	require.NoError(t, err)
	require.NotNil(t, qi)
	require.Equal(t, []byte("2"), qi.content)

	err = db.dequeue(qi)
	require.NoError(t, err)

	qi, err = db.peekQueue(context.Background(), "test")
	require.NoError(t, err)
	require.Nil(t, qi)

	time.Sleep(1 * time.Second)

	qi, err = db.peekQueue(context.Background(), "test")
	require.NoError(t, err)
	require.NotNil(t, qi)
	require.Equal(t, []byte("1"), qi.content)

}

func Benchmark_queue(b *testing.B) {
	app := &goBlog{
		cfg: &config{
			Db: &configDb{
				File: filepath.Join(b.TempDir(), "test.db"),
			},
		},
	}
	_ = app.initDatabase(false)
	defer app.db.close()
	db := app.db

	err := db.enqueue("test", []byte("1"), time.Now())
	require.NoError(b, err)

	b.Run("Peek with item", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = db.peekQueue(context.Background(), "test")
		}
	})

	b.Run("Peek without item", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = db.peekQueue(context.Background(), "abc")
		}
	})
}
