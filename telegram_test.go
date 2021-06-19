package main

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
	fakeClient := getFakeHTTPClient()

	tg := &configTelegram{
		Enabled:  true,
		ChatID:   "chatid",
		BotToken: "bottoken",
	}

	app := &goBlog{
		httpClient: fakeClient,
	}

	fakeClient.setFakeResponse(200, "", nil)

	err := app.send(tg, "Message", "HTML")
	assert.Nil(t, err)

	assert.NotNil(t, fakeClient.req)
	assert.Nil(t, fakeClient.err)
	assert.Equal(t, http.MethodPost, fakeClient.req.Method)
	assert.Equal(t, "https://api.telegram.org/botbottoken/sendMessage?chat_id=chatid&parse_mode=HTML&text=Message", fakeClient.req.URL.String())
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
		fakeClient := getFakeHTTPClient()

		fakeClient.setFakeResponse(200, "", nil)

		app := &goBlog{
			pPostHooks: []postHookFunc{},
			cfg: &config{
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
			httpClient: fakeClient,
		}
		app.setInMemoryDatabase()

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

		assert.Equal(
			t,
			"https://api.telegram.org/botbottoken/sendMessage?chat_id=chatid&parse_mode=HTML&text=Title%0A%0A%3Ca+href%3D%22https%3A%2F%2Fexample.com%2Fs%2F1%22%3Ehttps%3A%2F%2Fexample.com%2Fs%2F1%3C%2Fa%3E",
			fakeClient.req.URL.String(),
		)
	})

	t.Run("Telegram disabled", func(t *testing.T) {
		fakeClient := getFakeHTTPClient()

		fakeClient.setFakeResponse(200, "", nil)

		app := &goBlog{
			pPostHooks: []postHookFunc{},
			cfg: &config{
				Server: &configServer{
					PublicAddress: "https://example.com",
				},
				Blogs: map[string]*configBlog{
					"en": {},
				},
			},
			httpClient: fakeClient,
		}
		app.setInMemoryDatabase()

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
