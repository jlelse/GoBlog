package main

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/microcosm-cc/bluemonday"
)

const defaultContactPath = "/contact"

func (a *goBlog) serveContactForm(w http.ResponseWriter, r *http.Request) {
	blog := r.Context().Value(blogKey).(string)
	cc := a.cfg.Blogs[blog].Contact
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
	// Get form values
	strict := bluemonday.StrictPolicy()
	// Name
	formName := strings.TrimSpace(strict.Sanitize(r.FormValue("name")))
	// Email
	formEmail := strings.TrimSpace(strict.Sanitize(r.FormValue("email")))
	// Website
	formWebsite := strings.TrimSpace(strict.Sanitize(r.FormValue("website")))
	// Message
	formMessage := strings.TrimSpace(strict.Sanitize(r.FormValue("message")))
	if formMessage == "" {
		a.serveError(w, r, "Message is empty", http.StatusBadRequest)
		return
	}
	// Build message
	var message bytes.Buffer
	if formName != "" {
		_, _ = fmt.Fprintf(&message, "Name: %s", formName)
		_, _ = fmt.Fprintln(&message)
	}
	if formEmail != "" {
		_, _ = fmt.Fprintf(&message, "Email: %s", formEmail)
		_, _ = fmt.Fprintln(&message)
	}
	if formWebsite != "" {
		_, _ = fmt.Fprintf(&message, "Website: %s", formWebsite)
		_, _ = fmt.Fprintln(&message)
	}
	if message.Len() > 0 {
		_, _ = fmt.Fprintln(&message)
	}
	_, _ = message.WriteString(formMessage)
	// Send submission
	blog := r.Context().Value(blogKey).(string)
	if cc := a.cfg.Blogs[blog].Contact; cc != nil && cc.SMTPHost != "" && cc.EmailFrom != "" && cc.EmailTo != "" {
		// Build email
		var email bytes.Buffer
		if ef := cc.EmailFrom; ef != "" {
			_, _ = fmt.Fprintf(&email, "From: %s <%s>", defaultIfEmpty(a.cfg.Blogs[blog].Title, "GoBlog"), cc.EmailFrom)
			_, _ = fmt.Fprintln(&email)
		}
		_, _ = fmt.Fprintf(&email, "To: %s", cc.EmailTo)
		_, _ = fmt.Fprintln(&email)
		if formEmail != "" {
			_, _ = fmt.Fprintf(&email, "Reply-To: %s", formEmail)
			_, _ = fmt.Fprintln(&email)
		}
		_, _ = fmt.Fprintf(&email, "Date: %s", time.Now().UTC().Format(time.RFC1123Z))
		_, _ = fmt.Fprintln(&email)
		_, _ = fmt.Fprintln(&email, "Subject: New message")
		_, _ = fmt.Fprintln(&email)
		_, _ = fmt.Fprintln(&email, message.String())
		// Send email
		auth := smtp.PlainAuth("", cc.SMTPUser, cc.SMTPPassword, cc.SMTPHost)
		port := cc.SMTPPort
		if port == 0 {
			port = 587
		}
		if err := smtp.SendMail(cc.SMTPHost+":"+strconv.Itoa(port), auth, cc.EmailFrom, []string{cc.EmailTo}, email.Bytes()); err != nil {
			log.Println("Failed to send mail:", err.Error())
		}
	} else {
		log.Println("New contact submission not send as email, config missing")
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
