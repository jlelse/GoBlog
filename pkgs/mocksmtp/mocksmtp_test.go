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
	port, getRecievedData, err := StartMockSMTPServer()
	require.NoError(t, err)

	// Send mail
	err = smtp.SendMail(
		fmt.Sprintf("127.0.0.1:%d", port),
		smtp.PlainAuth("", "user", "pass", "127.0.0.1"),
		"admin@smtp.example.com",
		[]string{"user@smtp.example.com"},
		[]byte("From: admin@smtp.example.com\nTo: user@smtp.example.com\nSubject: Test\nMIME-version: 1.0\nContent-Type: text/html; charset=\"UTF-8\"\n\nThis is a test mail."),
	)
	require.NoError(t, err)

	// Get received data
	recievedData, err := getRecievedData()
	require.NoError(t, err)
	assert.Contains(t, recievedData, "From:")
	assert.Contains(t, recievedData, "This is a test mail")
}
