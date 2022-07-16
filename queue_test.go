package main

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func Test_queue(t *testing.T) {

	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	_ = app.initConfig(false)

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

}
