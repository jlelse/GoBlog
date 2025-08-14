package main

import (
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotifications(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	setup := func(t *testing.T) *goBlog {
		app := &goBlog{cfg: createDefaultTestConfig(t)}
		err := app.initConfig(false)
		must.NoError(err)
		return app
	}

	t.Run("db operations", func(t *testing.T) {
		app := setup(t)
		db := app.db
		must.NotNil(db)

		// Save a notification
		n := &notification{Time: time.Now().Unix(), Text: "test1"}
		must.NoError(db.saveNotification(n))

		// Ensure it can be retrieved
		got, err := db.getNotifications(&notificationsRequestConfig{})
		must.NoError(err)
		is.Len(got, 1)
		is.Equal("test1", got[0].Text)

		// Insert a second notification
		n2 := &notification{Time: time.Now().Add(1 * time.Minute).Unix(), Text: "test2"}
		must.NoError(db.saveNotification(n2))

		// Count without limits
		cnt, err := db.countNotifications(&notificationsRequestConfig{})
		must.NoError(err)
		is.Equal(2, cnt)

		// Count with limit/offset via buildNotificationsQuery path
		q, args := buildNotificationsQuery(&notificationsRequestConfig{limit: 1, offset: 1})
		// Query should contain limit/offset and args length should be 2
		is.Contains(q, "limit @limit offset @offset")
		is.Len(args, 2)

		// Delete first notification by id
		list, err := db.getNotifications(&notificationsRequestConfig{})
		must.NoError(err)
		must.Len(list, 2)
		idToDelete := list[0].ID
		must.NoError(db.deleteNotification(idToDelete))

		cnt, err = db.countNotifications(&notificationsRequestConfig{})
		must.NoError(err)
		is.Equal(1, cnt)

		// Delete all
		must.NoError(db.deleteAllNotifications())
		cnt, err = db.countNotifications(&notificationsRequestConfig{})
		must.NoError(err)
		is.Equal(0, cnt)
	})

	t.Run("send ntfy request is executed", func(t *testing.T) {
		app := setup(t)
		fc := newFakeHttpClient()
		app.httpClient = fc.Client
		defer fc.clean()

		fc.setFakeResponse(http.StatusOK, "OK")
		// Configure only ntfy so only one outbound request is made
		app.cfg.Notifications = &configNotifications{Ntfy: &configNtfy{Enabled: true, Topic: "testtopic"}}

		app.sendNotification("hello ntfy")

		// Inspect captured request
		fc.mu.Lock()
		req := fc.req
		fc.mu.Unlock()
		must.NotNil(req)
		is.Equal(http.MethodPost, req.Method)
		body, _ := io.ReadAll(req.Body)
		is.Equal("hello ntfy", string(body))
		is.Contains(req.URL.Path, "/testtopic")
	})

	t.Run("send telegram request is executed", func(t *testing.T) {
		app := setup(t)
		fc := newFakeHttpClient()
		app.httpClient = fc.Client
		defer fc.clean()

		// Telegram expects a JSON response with ok/result
		fc.setFakeResponse(http.StatusOK, `{"ok":true,"result":{"chat":{"id":12345},"message_id":678}}`)
		// Configure only telegram
		app.cfg.Notifications = &configNotifications{Telegram: &configTelegram{Enabled: true, BotToken: "TOKEN123", ChatID: "999"}}

		app.sendNotification("hello tg")

		fc.mu.Lock()
		req := fc.req
		fc.mu.Unlock()
		must.NotNil(req)
		// requests sends parameters as query for this client; accept GET here
		is.Equal(http.MethodGet, req.Method)
		// Check presence of chat_id and text in query string
		q := req.URL.RawQuery
		is.Contains(q, "chat_id=999")
		is.Contains(q, "text=hello+tg")
		is.Contains(req.URL.Path, "/botTOKEN123/sendMessage")
	})

	t.Run("ntfy failure does not prevent telegram and errors are handled", func(t *testing.T) {
		app := setup(t)
		fc := newFakeHttpClient()
		app.httpClient = fc.Client
		defer fc.clean()

		// capture both requests
		var mu sync.Mutex
		var recs []struct {
			Method   string
			Path     string
			Body     string
			RawQuery string
			Status   int
		}

		fc.setHandler(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			var b []byte
			if r.Body != nil {
				b, _ = io.ReadAll(r.Body)
			}
			mu.Lock()
			recs = append(recs, struct {
				Method   string
				Path     string
				Body     string
				RawQuery string
				Status   int
			}{Method: r.Method, Path: r.URL.Path, Body: string(b), RawQuery: r.URL.RawQuery})
			mu.Unlock()
			if strings.Contains(r.URL.Path, "testtopic") {
				rw.WriteHeader(http.StatusInternalServerError)
				_, _ = rw.Write([]byte("fail"))
				return
			}
			// telegram
			rw.WriteHeader(http.StatusOK)
			_, _ = rw.Write([]byte(`{"ok":true,"result":{"chat":{"id":12345},"message_id":678}}`))
		}))

		// Configure both services
		app.cfg.Notifications = &configNotifications{
			Ntfy:     &configNtfy{Enabled: true, Topic: "testtopic"},
			Telegram: &configTelegram{Enabled: true, BotToken: "TOKEN123", ChatID: "999"},
		}

		app.sendNotification("hello fail")

		// Allow goroutines to finish by checking captured requests
		mu.Lock()
		defer mu.Unlock()
		must.True(len(recs) >= 2, "expected at least 2 requests, got %d", len(recs))
		// Ensure one of them is ntfy (POST body) and one is telegram (sendMessage path)
		var sawNtfy, sawTg bool
		for _, r := range recs {
			if strings.Contains(r.Path, "testtopic") {
				sawNtfy = true
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "hello fail", r.Body)
			}
			if strings.Contains(r.Path, "/sendMessage") {
				sawTg = true
				// telegram uses query
				assert.Contains(t, r.RawQuery, "chat_id=999")
			}
		}
		must.True(sawNtfy)
		must.True(sawTg)
	})

	t.Run("telegram failure does not prevent ntfy and errors are handled", func(t *testing.T) {
		app := setup(t)
		fc := newFakeHttpClient()
		app.httpClient = fc.Client
		defer fc.clean()

		var mu sync.Mutex
		var recs []struct {
			Method   string
			Path     string
			Body     string
			RawQuery string
		}

		fc.setHandler(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			var b []byte
			if r.Body != nil {
				b, _ = io.ReadAll(r.Body)
			}
			mu.Lock()
			recs = append(recs, struct {
				Method   string
				Path     string
				Body     string
				RawQuery string
			}{Method: r.Method, Path: r.URL.Path, Body: string(b), RawQuery: r.URL.RawQuery})
			mu.Unlock()
			if strings.Contains(r.URL.Path, "/sendMessage") {
				rw.WriteHeader(http.StatusInternalServerError)
				_, _ = rw.Write([]byte("tg fail"))
				return
			}
			// ntfy
			rw.WriteHeader(http.StatusOK)
			_, _ = rw.Write([]byte("OK"))
		}))

		// Configure both services
		app.cfg.Notifications = &configNotifications{
			Ntfy:     &configNtfy{Enabled: true, Topic: "testtopic"},
			Telegram: &configTelegram{Enabled: true, BotToken: "TOKEN123", ChatID: "999"},
		}

		app.sendNotification("hello both")

		mu.Lock()
		defer mu.Unlock()
		must.True(len(recs) >= 2, "expected at least 2 requests, got %d", len(recs))
		var sawNtfy, sawTg bool
		for _, r := range recs {
			if strings.Contains(r.Path, "testtopic") {
				sawNtfy = true
				assert.Equal(t, "POST", r.Method)
			}
			if strings.Contains(r.Path, "/sendMessage") {
				sawTg = true
				assert.Contains(t, r.RawQuery, "chat_id=999")
			}
		}
		must.True(sawNtfy)
		must.True(sawTg)
	})
}
