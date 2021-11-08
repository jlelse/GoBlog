package mocksmtp

import (
	"fmt"
	"net/smtp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_mocksmtp(t *testing.T) {
	// Start mock SMTP server
	port, rd, cancel, err := StartMockSMTPServer()
	require.NoError(t, err)
	defer cancel()

	// Send mail
	err = smtp.SendMail(
		fmt.Sprintf("127.0.0.1:%d", port),
		smtp.PlainAuth("", "user", "pass", "127.0.0.1"),
		"admin@example.com",
		[]string{"user@example.com"},
		[]byte("From: admin@example.com\nTo: user@example.com\nSubject: Test\nMIME-version: 1.0\nContent-Type: text/html; charset=\"UTF-8\"\n\nThis is a test mail."),
	)
	require.NoError(t, err)

	// Get received data
	assert.Contains(t, rd.Froms, "admin@example.com")
	assert.Contains(t, rd.Rcpts, "user@example.com")
	assert.Contains(t, rd.Usernames, "user")
	assert.Contains(t, rd.Passwords, "pass")
	if assert.Len(t, rd.Froms, 1) {
		assert.Contains(t, string(rd.Datas[0]), "This is a test mail")
	}
}
