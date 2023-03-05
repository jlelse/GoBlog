package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	mail "github.com/xhit/go-simple-mail/v2"
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
			log.Println(err.Error())
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
	// Connect to SMTP
	smtpServer := mail.NewSMTPClient()
	smtpServer.Host = cc.SMTPHost
	port := cc.SMTPPort
	if port == 0 {
		port = 587
	}
	smtpServer.Port = port
	smtpServer.Username = cc.SMTPUser
	smtpServer.Password = cc.SMTPPassword
	smtpServer.KeepAlive = false
	smtpClient, err := smtpServer.Connect()
	if err != nil {
		return err
	}
	// Build email
	msg := mail.NewMSG()
	msg.AddTo(cc.EmailTo)
	msg.SetFrom(cc.EmailFrom)
	if replyTo != "" {
		msg.SetReplyTo(replyTo)
	}
	msg.SetDate(time.Now().UTC().Format("2006-01-02 15:04:05 MST"))
	subject := cc.EmailSubject
	if subject == "" {
		subject = "New contact message"
	}
	msg.SetSubject(subject)
	msg.SetBody(mail.TextPlain, body)
	// Send mail
	return msg.Send(smtpClient)
}
