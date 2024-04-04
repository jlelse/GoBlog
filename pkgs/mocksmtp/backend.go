package mocksmtp

import (
	"io"

	"github.com/emersion/go-sasl"
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

var _ smtp.Backend = (*backend)(nil)

func (b *backend) NewSession(_ *smtp.Conn) (smtp.Session, error) {
	return &session{
		values: b.values,
	}, nil
}

type session struct {
	values *ReceivedValues
}

var _ smtp.Session = (*session)(nil)
var _ smtp.AuthSession = (*session)(nil)

func (s *session) AuthMechanisms() []string {
	return []string{sasl.Plain}
}

func (s *session) Auth(mech string) (sasl.Server, error) {
	return sasl.NewPlainServer(func(identity, username, password string) error {
		s.values.Usernames = append(s.values.Usernames, username)
		s.values.Passwords = append(s.values.Passwords, password)
		return nil
	}), nil
}

func (s *session) Mail(from string, _ *smtp.MailOptions) error {
	s.values.Froms = append(s.values.Froms, from)
	return nil
}

func (s *session) Rcpt(to string, _ *smtp.RcptOptions) error {
	s.values.Rcpts = append(s.values.Rcpts, to)
	return nil
}

func (s *session) Data(r io.Reader) error {
	if b, err := io.ReadAll(r); err != nil {
		return err
	} else {
		s.values.Datas = append(s.values.Datas, b)
	}
	return nil
}

func (*session) Reset() {}

func (*session) Logout() error {
	return nil
}
