package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
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
	if err := a.sendContactEmail(bc.Contact, message.String(), formEmail); err != nil {
		log.Println(err.Error())
	}
	// Send notification
	a.sendNotification(message.String())
	// Give feedback
	a.render(w, r, a.renderContact, &renderData{
		Data: &contactRenderData{
			sent: true,
		},
	})
}

func (*goBlog) sendContactEmail(cc *configContact, body, replyTo string) error {
	// Check required config
	if cc == nil || cc.SMTPHost == "" || cc.EmailFrom == "" || cc.EmailTo == "" {
		return fmt.Errorf("email not send as config is missing")
	}
	// Build email
	email := bufferpool.Get()
	defer bufferpool.Put(email)
	_, _ = fmt.Fprintf(email, "To: %s\n", cc.EmailTo)
	if replyTo != "" {
		_, _ = fmt.Fprintf(email, "Reply-To: %s\n", replyTo)
	}
	_, _ = fmt.Fprintf(email, "Date: %s\n", time.Now().UTC().Format(time.RFC1123Z))
	_, _ = fmt.Fprintf(email, "From: %s\n", cc.EmailFrom)
	subject := cc.EmailSubject
	if subject == "" {
		subject = "New contact message"
	}
	_, _ = fmt.Fprintf(email, "Subject: %s\n\n", subject)
	_, _ = fmt.Fprintf(email, "%s\n", body)
	// Send email using SMTP
	auth := sasl.NewPlainClient("", cc.SMTPUser, cc.SMTPPassword)
	port := cc.SMTPPort
	if port == 0 {
		port = 587
	}
	return smtp.SendMail(cc.SMTPHost+":"+strconv.Itoa(port), auth, cc.EmailFrom, []string{cc.EmailTo}, email)
}
