package main

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_queue(t *testing.T) {
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	_ = app.initConfig(false)

	t.Run("Basic Queue Operations", func(t *testing.T) {
		time1 := time.Now()
		err := app.enqueue("test", []byte(""), time.Now())
		require.Error(t, err)

		err = app.enqueue("test", []byte("1"), time1)
		require.NoError(t, err)

		err = app.enqueue("test", []byte("2"), time.Now())
		require.NoError(t, err)

		qi, err := app.peekQueue(context.Background(), "abc")
		require.NoError(t, err)
		require.Nil(t, qi)

		qi, err = app.peekQueue(context.Background(), "test")
		require.NoError(t, err)
		require.NotNil(t, qi)
		require.Equal(t, []byte("1"), qi.content)
		require.Equal(t, time1.UTC(), qi.schedule.UTC())

		err = app.reschedule(qi, 1*time.Second)
		require.NoError(t, err)

		qi, err = app.peekQueue(context.Background(), "test")
		require.NoError(t, err)
		require.NotNil(t, qi)
		require.Equal(t, []byte("2"), qi.content)

		err = app.dequeue(qi)
		require.NoError(t, err)

		qi, err = app.peekQueue(context.Background(), "test")
		require.NoError(t, err)
		require.Nil(t, qi)

		time.Sleep(1 * time.Second)

		qi, err = app.peekQueue(context.Background(), "test")
		require.NoError(t, err)
		require.NotNil(t, qi)
		require.Equal(t, []byte("1"), qi.content)
	})

	t.Run("Listen On Queue", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		processedItems := make([]string, 0)
		var mu sync.Mutex

		app.listenOnQueue("test_listen", 100*time.Millisecond, func(qi *queueItem, dequeue func(), reschedule func(time.Duration)) {
			mu.Lock()
			processedItems = append(processedItems, string(qi.content))
			mu.Unlock()
			dequeue()
		})

		// Enqueue items
		err := app.enqueue("test_listen", []byte("item1"), time.Now())
		require.NoError(t, err)
		err = app.enqueue("test_listen", []byte("item2"), time.Now().Add(200*time.Millisecond))
		require.NoError(t, err)

		// Wait for processing
		<-ctx.Done()

		mu.Lock()
		assert.ElementsMatch(t, []string{"item1", "item2"}, processedItems)
		mu.Unlock()
	})

	t.Run("Reschedule Item", func(t *testing.T) {
		err := app.enqueue("test_reschedule", []byte("reschedule_item"), time.Now())
		require.NoError(t, err)

		qi, err := app.peekQueue(context.Background(), "test_reschedule")
		require.NoError(t, err)
		require.NotNil(t, qi)

		originalSchedule := qi.schedule

		err = app.reschedule(qi, 1*time.Second)
		require.NoError(t, err)

		time.Sleep(1 * time.Second)

		qi, err = app.peekQueue(context.Background(), "test_reschedule")
		require.NoError(t, err)
		require.NotNil(t, qi)
		assert.True(t, qi.schedule.After(originalSchedule))
	})
}
