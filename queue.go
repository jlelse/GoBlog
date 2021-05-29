package main

import (
	"database/sql"
	"errors"
	"time"

	"github.com/araddon/dateparse"
)

func enqueue(name string, content []byte, schedule time.Time) error {
	if len(content) == 0 {
		return errors.New("empty content")
	}
	_, err := appDb.exec("insert into queue (name, content, schedule) values (@name, @content, @schedule)",
		sql.Named("name", name), sql.Named("content", content), sql.Named("schedule", schedule.UTC().String()))
	return err
}

type queueItem struct {
	id       int
	name     string
	content  []byte
	schedule *time.Time
}

func (qi *queueItem) reschedule(dur time.Duration) error {
	_, err := appDb.exec("update queue set schedule = @schedule, content = @content where id = @id", sql.Named("schedule", qi.schedule.Add(dur).UTC().String()), sql.Named("content", qi.content), sql.Named("id", qi.id))
	return err
}

func (qi *queueItem) dequeue() error {
	_, err := appDb.exec("delete from queue where id = @id", sql.Named("id", qi.id))
	return err
}

func peekQueue(name string) (*queueItem, error) {
	row, err := appDb.queryRow("select id, name, content, schedule from queue where schedule <= @schedule and name = @name order by schedule asc limit 1", sql.Named("name", name), sql.Named("schedule", time.Now().UTC().String()))
	if err != nil {
		return nil, err
	}
	qi := &queueItem{}
	var timeString string
	if err = row.Scan(&qi.id, &qi.name, &qi.content, &timeString); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	t, err := dateparse.ParseLocal(timeString)
	if err != nil {
		return nil, err
	}
	t = t.Local()
	qi.schedule = &t
	return qi, nil
}
