package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/araddon/dateparse"
)

type queueItem struct {
	schedule time.Time
	name     string
	content  []byte
	id       int
}

func (a *goBlog) enqueue(name string, content []byte, schedule time.Time) error {
	if len(content) == 0 {
		return errors.New("empty content")
	}
	_, err := a.db.exec(
		"insert into queue (name, content, schedule) values (@name, @content, @schedule)",
		sql.Named("name", name),
		sql.Named("content", content),
		sql.Named("schedule", schedule.UTC().Format(time.RFC3339Nano)),
	)
	if err != nil {
		return err
	}
	return nil
}

func (a *goBlog) reschedule(qi *queueItem, dur time.Duration) error {
	_, err := a.db.exec(
		"update queue set schedule = @schedule, content = @content where id = @id",
		sql.Named("schedule", qi.schedule.Add(dur).UTC().Format(time.RFC3339Nano)),
		sql.Named("content", qi.content),
		sql.Named("id", qi.id),
	)
	return err
}

func (a *goBlog) dequeue(qi *queueItem) error {
	_, err := a.db.exec("delete from queue where id = @id", sql.Named("id", qi.id))
	return err
}

func (a *goBlog) peekQueue(ctx context.Context, name string) (*queueItem, error) {
	row, err := a.db.queryRowContext(
		ctx,
		"select id, name, content, schedule from queue where schedule <= @schedule and name = @name order by schedule asc limit 1",
		sql.Named("name", name),
		sql.Named("schedule", time.Now().UTC().Format(time.RFC3339Nano)),
	)
	if err != nil {
		return nil, err
	}
	qi := &queueItem{}
	var timeString string
	if err = row.Scan(&qi.id, &qi.name, &qi.content, &timeString); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	t, err := dateparse.ParseIn(timeString, time.UTC)
	if err != nil {
		return nil, err
	}
	qi.schedule = t
	return qi, nil
}

type queueProcessFunc func(qi *queueItem, dequeue func(), reschedule func(time.Duration))

func (a *goBlog) listenOnQueue(queueName string, wait time.Duration, process queueProcessFunc) {
	go func() {
		done := false
		var wg sync.WaitGroup
		wg.Add(1)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		a.shutdown.Add(func() {
			done = true
			cancel()
			wg.Wait()
			log.Println("Stopped queue:", queueName)
		})
		for !done {
			qi, err := a.peekQueue(ctx, queueName)
			if err != nil {
				// log.Println("queue peek error:", err.Error())
				continue
			}
			if qi == nil {
				// No item in the queue, wait a moment
				select {
				case <-time.After(wait):
					continue
				case <-ctx.Done():
					done = true
					continue
				}
			}
			process(
				qi,
				func() {
					if err := a.dequeue(qi); err != nil {
						log.Println("queue dequeue error:", err.Error())
					}
				},
				func(dur time.Duration) {
					if err := a.reschedule(qi, dur); err != nil {
						log.Println("queue reschedule error:", err.Error())
					}
				},
			)
		}
		wg.Done()
	}()
}
