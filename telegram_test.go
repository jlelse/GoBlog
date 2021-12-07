package main

import (
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_configTelegram_enabled(t *testing.T) {
	if (&configTelegram{}).enabled() == true {
		t.Error("Telegram shouldn't be enabled")
	}

	var tg *configTelegram
	if tg.enabled() == true {
		t.Error("Telegram shouldn't be enabled")
	}

	if (&configTelegram{
		Enabled: true,
	}).enabled() == true {
		t.Error("Telegram shouldn't be enabled")
	}

	if (&configTelegram{
		Enabled: true,
		ChatID:  "abc",
	}).enabled() == true {
		t.Error("Telegram shouldn't be enabled")
	}

	if (&configTelegram{
		Enabled:  true,
		BotToken: "abc",
	}).enabled() == true {
		t.Error("Telegram shouldn't be enabled")
	}

	if (&configTelegram{
		Enabled:  true,
		BotToken: "abc",
		ChatID:   "abc",
	}).enabled() != true {
		t.Error("Telegram should be enabled")
	}
}

func Test_configTelegram_generateHTML(t *testing.T) {
	tg := &configTelegram{
		Enabled:  true,
		ChatID:   "abc",
		BotToken: "abc",
	}

	// Without Instant View

	expected := "Title\n\n<a href=\"https://example.com/s/1\">https://example.com/s/1</a>"
	if got := tg.generateHTML("Title", "https://example.com/test", "https://example.com/s/1"); got != expected {
		t.Errorf("Wrong result, got: %v", got)
	}

	// With Instant View

	tg.InstantViewHash = "abc"
	expected = "Title\n\n<a href=\"https://t.me/iv?rhash=abc&url=https%3A%2F%2Fexample.com%2Ftest\">https://example.com/s/1</a>"
	if got := tg.generateHTML("Title", "https://example.com/test", "https://example.com/s/1"); got != expected {
		t.Errorf("Wrong result, got: %v", got)
	}
}

func Test_configTelegram_send(t *testing.T) {
	fakeClient := newFakeHttpClient()

	fakeClient.setHandler(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if r.URL.String() == "https://api.telegram.org/botbottoken/getMe" {
			rw.WriteHeader(http.StatusOK)
			rw.Write([]byte(`{"ok":true,"result":{"id":123456789,"is_bot":true,"first_name":"Test","username":"testbot"}}`))
			return
		}
		rw.WriteHeader(http.StatusOK)
		rw.Write([]byte(`{"ok":true,"result":{"message_id":123,"from":{"id":123456789,"is_bot":true,"first_name":"Test","username":"testbot"},"chat":{"id":789,"first_name":"Test","username":"testbot"},"date":1564181818,"text":"Message"}}`))
	}))

	tg := &configTelegram{
		Enabled:  true,
		ChatID:   "chatid",
		BotToken: "bottoken",
	}

	app := &goBlog{
		httpClient: fakeClient.Client,
	}

	chatId, msgId, err := app.send(tg, "Message", "HTML")
	require.Nil(t, err)

	assert.Equal(t, 123, msgId)
	assert.Equal(t, int64(789), chatId)

	assert.NotNil(t, fakeClient.req)
	assert.Equal(t, http.MethodPost, fakeClient.req.Method)
	assert.Equal(t, "https://api.telegram.org/botbottoken/sendMessage", fakeClient.req.URL.String())

	req := fakeClient.req
	assert.Equal(t, "chatid", req.FormValue("chat_id"))
	assert.Equal(t, "HTML", req.FormValue("parse_mode"))
	assert.Equal(t, "Message", req.FormValue("text"))
}

func Test_goBlog_initTelegram(t *testing.T) {
	app := &goBlog{
		pPostHooks: []postHookFunc{},
	}

	app.initTelegram()

	if len(app.pPostHooks) != 1 {
		t.Error("Hook not registered")
	}
}

func Test_telegram(t *testing.T) {
	t.Run("Send post to Telegram", func(t *testing.T) {
		fakeClient := newFakeHttpClient()

		fakeClient.setHandler(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			if r.URL.String() == "https://api.telegram.org/botbottoken/getMe" {
				rw.WriteHeader(http.StatusOK)
				rw.Write([]byte(`{"ok":true,"result":{"id":123456789,"is_bot":true,"first_name":"Test","username":"testbot"}}`))
				return
			}
			rw.WriteHeader(http.StatusOK)
			rw.Write([]byte(`{"ok":true,"result":{"message_id":123,"from":{"id":123456789,"is_bot":true,"first_name":"Test","username":"testbot"},"chat":{"id":123456789,"first_name":"Test","username":"testbot"},"date":1564181818,"text":"Message"}}`))
		}))

		app := &goBlog{
			pPostHooks: []postHookFunc{},
			cfg: &config{
				Db: &configDb{
					File: filepath.Join(t.TempDir(), "test.db"),
				},
				Server: &configServer{
					PublicAddress: "https://example.com",
				},
				Blogs: map[string]*configBlog{
					"en": {
						Telegram: &configTelegram{
							Enabled:  true,
							ChatID:   "chatid",
							BotToken: "bottoken",
						},
					},
				},
			},
			httpClient: fakeClient.Client,
		}
		_ = app.initDatabase(false)

		app.initMarkdown()
		app.initTelegram()

		p := &post{
			Path:          "/test",
			RenderedTitle: "Title",
			Published:     time.Now().String(),
			Section:       "test",
			Blog:          "en",
			Status:        statusPublished,
		}

		app.pPostHooks[0](p)

		assert.Equal(t, "https://api.telegram.org/botbottoken/sendMessage", fakeClient.req.URL.String())

		req := fakeClient.req
		assert.Equal(t, "chatid", req.FormValue("chat_id"))
		assert.Equal(t, "HTML", req.FormValue("parse_mode"))
		assert.Equal(t, "Title\n\n<a href=\"https://example.com/s/1\">https://example.com/s/1</a>", req.FormValue("text"))
	})

	t.Run("Telegram disabled", func(t *testing.T) {
		fakeClient := newFakeHttpClient()

		app := &goBlog{
			pPostHooks: []postHookFunc{},
			cfg: &config{
				Db: &configDb{
					File: filepath.Join(t.TempDir(), "test.db"),
				},
				Server: &configServer{
					PublicAddress: "https://example.com",
				},
				Blogs: map[string]*configBlog{
					"en": {},
				},
			},
			httpClient: fakeClient.Client,
		}
		_ = app.initDatabase(false)

		app.initTelegram()

		p := &post{
			Path: "/test",
			Parameters: map[string][]string{
				"title": {"Title"},
			},
			Published: time.Now().String(),
			Section:   "test",
			Blog:      "en",
			Status:    statusPublished,
		}

		app.pPostHooks[0](p)

		assert.Nil(t, fakeClient.req)
	})
}
