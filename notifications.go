package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/vcraescu/go-paginator"
)

type notification struct {
	ID   int
	Time int64
	Text string
}

func sendNotification(text string) {
	n := &notification{
		Time: time.Now().Unix(),
		Text: text,
	}
	if err := saveNotification(n); err != nil {
		log.Println("Failed to save notification:", err.Error())
	}
	if an := appConfig.Notifications; an != nil {
		if tg := an.Telegram; tg != nil && tg.Enabled {
			err := sendTelegramMessage(n.Text, "", tg.BotToken, tg.ChatID)
			if err != nil {
				log.Println("Failed to send Telegram notification:", err.Error())
			}
		}
	}
}

func saveNotification(n *notification) error {
	if _, err := appDbExec("insert into notifications (time, text) values (@time, @text)", sql.Named("time", n.Time), sql.Named("text", n.Text)); err != nil {
		return err
	}
	return nil
}

type notificationsRequestConfig struct {
	offset, limit int
}

func buildNotificationsQuery(config *notificationsRequestConfig) (query string, args []interface{}) {
	args = []interface{}{}
	query = "select id, time, text from notifications order by id desc"
	if config.limit != 0 || config.offset != 0 {
		query += " limit @limit offset @offset"
		args = append(args, sql.Named("limit", config.limit), sql.Named("offset", config.offset))
	}
	return
}

func getNotifications(config *notificationsRequestConfig) ([]*notification, error) {
	notifications := []*notification{}
	query, args := buildNotificationsQuery(config)
	rows, err := appDbQuery(query, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		n := &notification{}
		err = rows.Scan(&n.ID, &n.Time, &n.Text)
		if err != nil {
			return nil, err
		}
		notifications = append(notifications, n)
	}
	return notifications, nil
}

func countNotifications(config *notificationsRequestConfig) (count int, err error) {
	query, params := buildNotificationsQuery(config)
	query = "select count(*) from (" + query + ")"
	row, err := appDbQueryRow(query, params...)
	if err != nil {
		return
	}
	err = row.Scan(&count)
	return
}

type notificationsPaginationAdapter struct {
	config *notificationsRequestConfig
	nums   int64
}

func (p *notificationsPaginationAdapter) Nums() (int64, error) {
	if p.nums == 0 {
		nums, _ := countNotifications(p.config)
		p.nums = int64(nums)
	}
	return p.nums, nil
}

func (p *notificationsPaginationAdapter) Slice(offset, length int, data interface{}) error {
	modifiedConfig := *p.config
	modifiedConfig.offset = offset
	modifiedConfig.limit = length

	notifications, err := getNotifications(&modifiedConfig)
	reflect.ValueOf(data).Elem().Set(reflect.ValueOf(&notifications).Elem())
	return err
}

func notificationsAdmin(notificationPath string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// Adapter
		pageNoString := chi.URLParam(r, "page")
		pageNo, _ := strconv.Atoi(pageNoString)
		p := paginator.New(&notificationsPaginationAdapter{config: &notificationsRequestConfig{}}, 10)
		p.SetPage(pageNo)
		var notifications []*notification
		err := p.Results(&notifications)
		if err != nil {
			serveError(w, r, err.Error(), http.StatusInternalServerError)
			return
		}
		// Navigation
		var hasPrev, hasNext bool
		var prevPage, nextPage int
		var prevPath, nextPath string
		hasPrev, _ = p.HasPrev()
		if hasPrev {
			prevPage, _ = p.PrevPage()
		} else {
			prevPage, _ = p.Page()
		}
		if prevPage < 2 {
			prevPath = notificationPath
		} else {
			prevPath = fmt.Sprintf("%s/page/%d", notificationPath, prevPage)
		}
		hasNext, _ = p.HasNext()
		if hasNext {
			nextPage, _ = p.NextPage()
		} else {
			nextPage, _ = p.Page()
		}
		nextPath = fmt.Sprintf("%s/page/%d", notificationPath, nextPage)
		// Render
		render(w, r, templateNotificationsAdmin, &renderData{
			Data: map[string]interface{}{
				"Notifications": notifications,
				"HasPrev":       hasPrev,
				"HasNext":       hasNext,
				"Prev":          slashIfEmpty(prevPath),
				"Next":          slashIfEmpty(nextPath),
			},
		})
	}
}
