package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/araddon/dateparse"
)

type queueItem struct {
	id       int
	name     string
	content  []byte
	schedule time.Time
}

type queueProcessFunc func(qi *queueItem, dequeue func(), reschedule func(time.Duration))

func (a *goBlog) enqueue(name string, content []byte, schedule time.Time) error {
	if len(content) == 0 {
		return errors.New("empty content")
	}
	_, err := a.db.Exec(
		"insert into queue (name, content, schedule) values (@name, @content, @schedule)",
		sql.Named("name", name),
		sql.Named("content", content),
		sql.Named("schedule", schedule.UTC().Format(time.RFC3339Nano)),
	)
	return err
}

func (a *goBlog) reschedule(qi *queueItem, dur time.Duration) error {
	_, err := a.db.Exec(
		"update queue set schedule = @schedule, content = @content where id = @id",
		sql.Named("schedule", qi.schedule.Add(dur).UTC().Format(time.RFC3339Nano)),
		sql.Named("content", qi.content),
		sql.Named("id", qi.id),
	)
	return err
}

func (a *goBlog) dequeue(qi *queueItem) error {
	_, err := a.db.Exec("delete from queue where id = @id", sql.Named("id", qi.id))
	return err
}

func (a *goBlog) peekQueue(ctx context.Context, name string) (*queueItem, error) {
	row, err := a.db.QueryRowContext(
		ctx,
		"select id, name, content, schedule from queue where schedule <= @schedule and name = @name order by schedule asc limit 1",
		sql.Named("name", name),
		sql.Named("schedule", time.Now().UTC().Format(time.RFC3339Nano)),
	)
	if err != nil {
		return nil, err
	}

	var qi queueItem
	var timeString string
	if err := row.Scan(&qi.id, &qi.name, &qi.content, &timeString); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan queue item: %w", err)
	}

	t, err := dateparse.ParseIn(timeString, time.UTC)
	if err != nil {
		return nil, fmt.Errorf("parse schedule time: %w", err)
	}
	qi.schedule = t

	return &qi, nil
}

func (a *goBlog) listenOnQueue(queueName string, wait time.Duration, process queueProcessFunc) {
	if process == nil {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	a.shutdown.Add(func() {
		cancel()
		wg.Wait()
	})

	wg.Add(1)
	go func() {
		a.processQueue(ctx, queueName, wait, process)
		wg.Done()
		a.info("stopped queue", "name", queueName)
	}()
}

func (a *goBlog) processQueue(ctx context.Context, queueName string, wait time.Duration, process queueProcessFunc) {
	for ctx.Err() == nil {
		select {
		case <-ctx.Done():
			return
		default:
			if err := a.processQueueItem(ctx, queueName, wait, process); err != nil {
				a.error("process queue item", "err", err)
			}
		}
	}
}

func (a *goBlog) processQueueItem(ctx context.Context, queueName string, wait time.Duration, process queueProcessFunc) error {
	qi, err := a.peekQueue(ctx, queueName)
	if err != nil {
		return fmt.Errorf("peek queue: %w", err)
	}

	if qi == nil {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(wait):
			return nil
		}
	}

	process(
		qi,
		func() {
			if err := a.dequeue(qi); err != nil {
				a.error("queue dequeue error", "err", err)
			}
		},
		func(dur time.Duration) {
			if err := a.reschedule(qi, dur); err != nil {
				a.error("queue reschedule error", "err", err)
			}
		},
	)

	return nil
}
