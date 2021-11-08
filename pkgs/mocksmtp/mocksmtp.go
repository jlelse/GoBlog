// This package contains code to mock an SMTP server and test mail sending.

package mocksmtp

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"net/textproto"
	"strconv"
)

// Inspired by https://play.golang.org/p/8mfrqNVWTPK

type ReceivedGetter func() (string, error)

func StartMockSMTPServer() (port int, rg ReceivedGetter, err error) {

	// Default server responses
	serverResponses := []string{
		"220 smtp.example.com Service ready",
		"250-ELHO -> ok",
		"250-Show Options for ESMTP",
		"250-8BITMIME",
		"250-SIZE",
		"250-AUTH LOGIN PLAIN",
		"250 HELP",
		"235 AUTH -> ok",
		"250 MAIL FROM -> ok",
		"250 RCPT TO -> ok",
		"354 DATA",
		"250 ... -> ok",
		"221 QUIT",
	}

	// Channel to check if error occured or done
	var errOrDone = make(chan error)

	// Received data buffer
	var buffer bytes.Buffer
	bufferWriter := bufio.NewWriter(&buffer)

	// Start server on random port
	mockSmtpServer, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, nil, err
	}

	// Get port from listener
	_, portStr, err := net.SplitHostPort(mockSmtpServer.Addr().String())
	if err != nil {
		return 0, nil, err
	}
	port, err = strconv.Atoi(portStr)
	if err != nil {
		return 0, nil, err
	}

	// Run mock SMTP server
	go func() {
		defer close(errOrDone)
		defer bufferWriter.Flush()
		defer mockSmtpServer.Close()

		conn, err := mockSmtpServer.Accept()
		if err != nil {
			errOrDone <- err
			return
		}
		defer conn.Close()

		tc := textproto.NewConn(conn)
		defer tc.Close()

		for _, res := range serverResponses {
			if res == "" {
				break
			}

			_ = tc.PrintfLine("%s", res)

			if len(res) >= 4 && res[3] == '-' {
				continue
			}

			if res == "221 QUIT" {
				return
			}

			for {
				msg, err := tc.ReadLine()
				if err != nil {
					errOrDone <- err
					return
				}

				_, _ = fmt.Fprintf(bufferWriter, "%s\n", msg)

				if res != "354 DATA" || msg == "." {
					break
				}
			}
		}
	}()

	// Define function to get received data
	getReceivedData := func() (string, error) {
		err, hasErr := <-errOrDone
		if hasErr {
			return "", err
		}
		return buffer.String(), nil
	}

	// Return port and function
	return port, getReceivedData, nil
}
