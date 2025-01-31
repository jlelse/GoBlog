package main

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/wneessen/go-mail"
	"go.goblog.app/app/pkgs/bufferpool"
)

const defaultContactPath = "/contact"

func (a *goBlog) serveContactForm(w http.ResponseWriter, r *http.Request) {
	_, bc := a.getBlog(r)
	cc := bc.Contact
	a.render(w, r, a.renderContact, &renderData{
		Data: &contactRenderData{
			title:       cc.Title,
			description: cc.Description,
			privacy:     cc.PrivacyPolicy,
		},
	})
}

func (a *goBlog) sendContactSubmission(w http.ResponseWriter, r *http.Request) {
	// Get blog
	_, bc := a.getBlog(r)
	// Get form values and build message
	message := bufferpool.Get()
	defer bufferpool.Put(message)
	// Message
	formMessage := cleanHTMLText(r.FormValue("message"))
	if formMessage == "" {
		a.serveError(w, r, "Message is empty", http.StatusBadRequest)
		return
	}
	// Name
	if formName := cleanHTMLText(r.FormValue("name")); formName != "" {
		_, _ = fmt.Fprintf(message, "Name: %s\n", formName)
	}
	// Email
	formEmail := cleanHTMLText(r.FormValue("email"))
	if formEmail != "" {
		_, _ = fmt.Fprintf(message, "Email: %s\n", formEmail)
	}
	// Website
	if formWebsite := cleanHTMLText(r.FormValue("website")); formWebsite != "" {
		_, _ = fmt.Fprintf(message, "Website: %s\n", formWebsite)
	}
	// Add line break if message is not empty
	if message.Len() > 0 {
		_, _ = fmt.Fprintf(message, "\n")
	}
	// Add message text to message
	_, _ = message.WriteString(formMessage)
	// Send submission
	go func() {
		if err := a.sendContactEmail(bc.Contact, message.String(), formEmail); err != nil {
			a.error("Failed to send contact email", "err", err)
		}
	}()
	// Send notification
	go a.sendNotification(message.String())
	// Give feedback
	a.render(w, r, a.renderContactSent, &renderData{})
}

func (*goBlog) sendContactEmail(cc *configContact, body, replyTo string) error {
	// Check required config
	if cc == nil || cc.SMTPHost == "" || cc.EmailFrom == "" || cc.EmailTo == "" {
		return fmt.Errorf("email not send as config is missing")
	}
	// Create mail
	message := mail.NewMsg()
	if err := message.From(cc.EmailFrom); err != nil {
		return err
	}
	if err := message.To(cc.EmailTo); err != nil {
		return err
	}
	if replyTo != "" {
		if err := message.ReplyTo(replyTo); err != nil {
			return err
		}
	}
	message.SetDate()
	subject := cc.EmailSubject
	if subject == "" {
		subject = "New contact message"
	}
	message.Subject(subject)
	message.SetBodyString(mail.TypeTextPlain, body)
	// Deliver the mail via SMTP
	port := 587
	if cc.SMTPPort != 0 {
		port = cc.SMTPPort
	}
	client, err := mail.NewClient(
		cc.SMTPHost,
		mail.WithPort(port),
		mail.WithUsername(cc.SMTPUser),
		mail.WithPassword(cc.SMTPPassword),
		mail.WithSMTPAuth(mail.SMTPAuthAutoDiscover),
		mail.WithTLSPolicy(mail.TLSOpportunistic),
	)
	if err != nil {
		return err
	}
	if cc.SMTPSSL {
		client.SetSSLPort(true, false)
	}

	// For tests, don't use auto discover
	if testing.Testing() {
		client.SetSMTPAuth(mail.SMTPAuthPlain)
	}

	return client.DialAndSend(message)
}
