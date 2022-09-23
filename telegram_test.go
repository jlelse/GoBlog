package main

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_configTelegram_enabled(t *testing.T) {
	assert.False(t, (&configTelegram{}).enabled())
	var tg *configTelegram
	assert.False(t, tg.enabled())

	assert.False(t, (&configTelegram{Enabled: true}).enabled())
	assert.False(t, (&configTelegram{Enabled: true, ChatID: "abc"}).enabled())
	assert.False(t, (&configTelegram{Enabled: true, BotToken: "abc"}).enabled())

	assert.True(t, (&configTelegram{Enabled: true, ChatID: "abc", BotToken: "abc"}).enabled())
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
			_, _ = rw.Write([]byte(`{"ok":true,"result":{"id":123456789,"is_bot":true,"first_name":"Test","username":"testbot"}}`))
			return
		}
		rw.WriteHeader(http.StatusOK)
		_, _ = rw.Write([]byte(`{"ok":true,"result":{"message_id":123,"from":{"id":123456789,"is_bot":true,"first_name":"Test","username":"testbot"},"chat":{"id":789,"first_name":"Test","username":"testbot"},"date":1564181818,"text":"Message"}}`))
	}))

	tg := &configTelegram{
		Enabled:  true,
		ChatID:   "chatid",
		BotToken: "bottoken",
	}

	app := &goBlog{
		httpClient: fakeClient.Client,
	}

	chatId, msgId, err := app.sendTelegram(tg, "Message", "HTML", false)
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
				_, _ = rw.Write([]byte(`{"ok":true,"result":{"id":123456789,"is_bot":true,"first_name":"Test","username":"testbot"}}`))
				return
			}
			rw.WriteHeader(http.StatusOK)
			_, _ = rw.Write([]byte(`{"ok":true,"result":{"message_id":123,"from":{"id":123456789,"is_bot":true,"first_name":"Test","username":"testbot"},"chat":{"id":123456789,"first_name":"Test","username":"testbot"},"date":1564181818,"text":"Message"}}`))
		}))

		cfg := createDefaultTestConfig(t)
		cfg.Blogs = map[string]*configBlog{
			"en": createDefaultBlog(),
		}
		cfg.Blogs["en"].Telegram = &configTelegram{
			Enabled:  true,
			ChatID:   "chatid",
			BotToken: "bottoken",
		}

		app := &goBlog{
			cfg:        cfg,
			httpClient: fakeClient.Client,
		}
		_ = app.initConfig(false)

		app.initMarkdown()
		app.initTelegram()

		p := &post{
			Path:          "/test",
			RenderedTitle: "Title",
			Published:     time.Now().String(),
			Section:       "test",
			Blog:          "en",
			Status:        statusPublished,
			Visibility:    visibilityPublic,
		}

		app.pPostHooks[0](p)

		assert.Equal(t, "https://api.telegram.org/botbottoken/sendMessage", fakeClient.req.URL.String())

		req := fakeClient.req
		assert.Equal(t, "chatid", req.FormValue("chat_id"))
		assert.Equal(t, "HTML", req.FormValue("parse_mode"))
		assert.Equal(t, "Title\n\n<a href=\"http://localhost:8080/s/1\">http://localhost:8080/s/1</a>", req.FormValue("text"))
	})

	t.Run("Telegram disabled", func(t *testing.T) {
		fakeClient := newFakeHttpClient()

		app := &goBlog{
			cfg:        createDefaultTestConfig(t),
			httpClient: fakeClient.Client,
		}

		_ = app.initConfig(false)

		app.initTelegram()

		app.postPostHooks(&post{
			Path: "/test",
			Parameters: map[string][]string{
				"title": {"Title"},
			},
			Published:  time.Now().String(),
			Section:    "test",
			Blog:       "default",
			Status:     statusPublished,
			Visibility: visibilityPublic,
		})

		assert.Nil(t, fakeClient.req)
	})
}
