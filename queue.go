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
	_, err := a.db.Exec(
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
	if process == nil {
		return
	}

	endQueue := false
	queueContext, cancelQueueContext := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	a.shutdown.Add(func() {
		endQueue = true
		cancelQueueContext()
		wg.Wait()
	})

	wg.Add(1)
	go func() {
	queueLoop:
		for {
			if endQueue {
				break queueLoop
			}
			qi, err := a.peekQueue(queueContext, queueName)
			if err != nil {
				log.Println("queue peek error:", err.Error())
				continue queueLoop
			}
			if qi == nil {
				// No item in the queue, wait a moment
				select {
				case <-time.After(wait):
					continue queueLoop
				case <-queueContext.Done():
					break queueLoop
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
		log.Println("stopped queue:", queueName)
		wg.Done()
	}()
}
