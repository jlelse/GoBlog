package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.goblog.app/app/pkgs/contenttype"
	"go.goblog.app/app/pkgs/mocksmtp"
)

func Test_contact(t *testing.T) {
	// Start the SMTP server
	port, rd, cancel, err := mocksmtp.StartMockSMTPServer()
	require.NoError(t, err)
	defer cancel()

	// Init everything
	app := &goBlog{
		cfg: createDefaultTestConfig(t),
	}
	app.cfg.Blogs = map[string]*configBlog{
		"en": {
			Lang: "en",
			// Config for contact
			Contact: &configContact{
				Enabled:      true,
				SMTPPort:     port,
				SMTPHost:     "127.0.0.1",
				SMTPUser:     "user",
				SMTPPassword: "pass",
				EmailTo:      "to@example.org",
				EmailFrom:    "from@example.org",
				EmailSubject: "Neue Kontaktnachricht",
			},
		},
	}
	app.cfg.DefaultBlog = "en"

	_ = app.initConfig(false)
	app.initSessions()
	_ = app.initTemplateStrings()

	// Make contact form request
	rec := httptest.NewRecorder()
	data := url.Values{}
	data.Add("name", "Test User")
	data.Add("email", "test@example.net")
	data.Add("website", "https://test.example.com")
	data.Add("message", "This is a test contact message")
	req := httptest.NewRequest(http.MethodPost, "/contact", strings.NewReader(data.Encode()))
	req.Header.Add(contentType, contenttype.WWWForm)
	app.sendContactSubmission(rec, req.WithContext(context.WithValue(req.Context(), blogKey, "en")))
	require.Equal(t, http.StatusOK, rec.Code)

	// Wait a second
	time.Sleep(500 * time.Millisecond)

	// Check sent mail
	assert.Contains(t, rd.Usernames, "user")
	assert.Contains(t, rd.Passwords, "pass")
	assert.Contains(t, rd.Froms, "from@example.org")
	assert.Contains(t, rd.Rcpts, "to@example.org")
	if assert.Len(t, rd.Datas, 1) {
		assert.Contains(t, string(rd.Datas[0]), "This is a test contact message")
		assert.Contains(t, string(rd.Datas[0]), "test@example.net")
		assert.Contains(t, string(rd.Datas[0]), "https://test.example.com")
		assert.Contains(t, string(rd.Datas[0]), "Test User")
		assert.Contains(t, string(rd.Datas[0]), "Neue Kontaktnachricht")
	}

}
