// This package contains code to mock an SMTP server and test mail sending.

package mocksmtp

import (
	"log"
	"net"
	"strconv"

	"github.com/emersion/go-smtp"
)

// Start a mock SMTP server on a random port
//
// Returns:
// port: the port the server is listening on,
// receivedValues: struct to read the received values like username, password, data,
// cancelFunc: function to stop the server,
// err: something went wrong
func StartMockSMTPServer() (port int, receivedValues *ReceivedValues, cancelFunc func(), err error) {

	// Start server on random port
	mockSmtpServer, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, nil, nil, err
	}

	// Get port from listener
	_, portStr, err := net.SplitHostPort(mockSmtpServer.Addr().String())
	if err != nil {
		return 0, nil, nil, err
	}
	port, err = strconv.Atoi(portStr)
	if err != nil {
		return 0, nil, nil, err
	}

	// Define received values
	receivedValues = &ReceivedValues{}

	// Init SMTP server
	s := smtp.NewServer(&backend{
		values: receivedValues,
	})
	s.Addr = mockSmtpServer.Addr().String()
	s.Domain = "127.0.0.1"
	s.AllowInsecureAuth = true

	// Start SMTP server
	go func() {
		if err := s.Serve(mockSmtpServer); err != nil {
			log.Fatal(err)
		}
	}()

	// Create cancel function
	cancelFunc = func() {
		_ = s.Close()
	}

	// Return port and function
	return port, receivedValues, cancelFunc, nil

}
