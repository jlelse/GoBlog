package mocksmtp

import (
	"io"
	"io/ioutil"

	"github.com/emersion/go-smtp"
)

// ReceivedValues contains all the data received from the SMTP server
type ReceivedValues struct {
	Usernames []string
	Passwords []string
	Froms     []string
	Rcpts     []string
	Datas     [][]byte
}

type backend struct {
	values *ReceivedValues
}

var _ smtp.Backend = &backend{}

func (b *backend) Login(_ *smtp.ConnectionState, username, password string) (smtp.Session, error) {
	b.values.Usernames = append(b.values.Usernames, username)
	b.values.Passwords = append(b.values.Passwords, password)
	return &session{
		values: b.values,
	}, nil
}

func (b *backend) AnonymousLogin(_ *smtp.ConnectionState) (smtp.Session, error) {
	return &session{
		values: b.values,
	}, nil
}

type session struct {
	values *ReceivedValues
}

var _ smtp.Session = &session{}

func (s *session) Mail(from string, _ smtp.MailOptions) error {
	s.values.Froms = append(s.values.Froms, from)
	return nil
}

func (s *session) Rcpt(to string) error {
	s.values.Rcpts = append(s.values.Rcpts, to)
	return nil
}

func (s *session) Data(r io.Reader) error {
	if b, err := ioutil.ReadAll(r); err != nil {
		return err
	} else {
		s.values.Datas = append(s.values.Datas, b)
	}
	return nil
}

func (s *session) Reset() {}

func (s *session) Logout() error {
	return nil
}
