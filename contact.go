package main

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"strconv"
	"time"
)

const defaultContactPath = "/contact"

func (a *goBlog) serveContactForm(w http.ResponseWriter, r *http.Request) {
	blog, bc := a.getBlog(r)
	cc := bc.Contact
	a.render(w, r, templateContact, &renderData{
		BlogString: blog,
		Data: map[string]interface{}{
			"title":       cc.Title,
			"description": cc.Description,
			"privacy":     cc.PrivacyPolicy,
		},
	})
}

func (a *goBlog) sendContactSubmission(w http.ResponseWriter, r *http.Request) {
	// Get blog
	blog, bc := a.getBlog(r)
	// Get form values and build message
	var message bytes.Buffer
	// Message
	formMessage := cleanHTMLText(r.FormValue("message"))
	if formMessage == "" {
		a.serveError(w, r, "Message is empty", http.StatusBadRequest)
		return
	}
	// Name
	if formName := cleanHTMLText(r.FormValue("name")); formName != "" {
		_, _ = fmt.Fprintf(&message, "Name: %s\n", formName)
	}
	// Email
	formEmail := cleanHTMLText(r.FormValue("email"))
	if formEmail != "" {
		_, _ = fmt.Fprintf(&message, "Email: %s\n", formEmail)
	}
	// Website
	if formWebsite := cleanHTMLText(r.FormValue("website")); formWebsite != "" {
		_, _ = fmt.Fprintf(&message, "Website: %s\n", formWebsite)
	}
	// Add line break if message is not empty
	if message.Len() > 0 {
		_, _ = fmt.Fprintf(&message, "\n")
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
	a.render(w, r, templateContact, &renderData{
		BlogString: blog,
		Data: map[string]interface{}{
			"sent": true,
		},
	})
}

func (a *goBlog) sendContactEmail(cc *configContact, body, replyTo string) error {
	// Check required config
	if cc == nil || cc.SMTPHost == "" || cc.EmailFrom == "" || cc.EmailTo == "" {
		return fmt.Errorf("email not send as config is missing")
	}
	// Build email
	var email bytes.Buffer
	_, _ = fmt.Fprintf(&email, "To: %s\n", cc.EmailTo)
	if replyTo != "" {
		_, _ = fmt.Fprintf(&email, "Reply-To: %s\n", replyTo)
	}
	_, _ = fmt.Fprintf(&email, "Date: %s\n", time.Now().UTC().Format(time.RFC1123Z))
	_, _ = fmt.Fprintf(&email, "Subject: New message\n\n")
	_, _ = fmt.Fprintf(&email, "%s\n", body)
	// Send email using SMTP
	auth := smtp.PlainAuth("", cc.SMTPUser, cc.SMTPPassword, cc.SMTPHost)
	port := cc.SMTPPort
	if port == 0 {
		port = 587
	}
	return smtp.SendMail(cc.SMTPHost+":"+strconv.Itoa(port), auth, cc.EmailFrom, []string{cc.EmailTo}, email.Bytes())
}
