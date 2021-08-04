package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
)

const telegramBaseURL = "https://api.telegram.org/bot"

func (a *goBlog) initTelegram() {
	a.pPostHooks = append(a.pPostHooks, func(p *post) {
		if tg := a.cfg.Blogs[p.Blog].Telegram; tg.enabled() && p.isPublishedSectionPost() {
			if html := tg.generateHTML(a.renderText(p.Title()), a.fullPostURL(p), a.shortPostURL(p)); html != "" {
				if err := a.send(tg, html, "HTML"); err != nil {
					log.Printf("Failed to send post to Telegram: %v", err)
				}
			}
		}
	})
}

func (tg *configTelegram) enabled() bool {
	if tg == nil || !tg.Enabled || tg.BotToken == "" || tg.ChatID == "" {
		return false
	}
	return true
}

func (tg *configTelegram) generateHTML(title, fullURL, shortURL string) string {
	if !tg.enabled() {
		return ""
	}
	replacer := strings.NewReplacer("<", "&lt;", ">", "&gt;", "&", "&amp;")
	var message bytes.Buffer
	if title != "" {
		message.WriteString(replacer.Replace(title))
		message.WriteString("\n\n")
	}
	if tg.InstantViewHash != "" {
		message.WriteString("<a href=\"https://t.me/iv?rhash=" + tg.InstantViewHash + "&url=" + url.QueryEscape(fullURL) + "\">")
		message.WriteString(replacer.Replace(shortURL))
		message.WriteString("</a>")
	} else {
		message.WriteString("<a href=\"" + shortURL + "\">")
		message.WriteString(replacer.Replace(shortURL))
		message.WriteString("</a>")
	}
	return message.String()
}

func (a *goBlog) send(tg *configTelegram, message, mode string) error {
	if !tg.enabled() {
		return nil
	}
	params := url.Values{}
	params.Add("chat_id", tg.ChatID)
	params.Add("text", message)
	if mode != "" {
		params.Add("parse_mode", mode)
	}
	tgURL, err := url.Parse(telegramBaseURL + tg.BotToken + "/sendMessage")
	if err != nil {
		return errors.New("failed to create Telegram request")
	}
	tgURL.RawQuery = params.Encode()
	req, _ := http.NewRequest(http.MethodPost, tgURL.String(), nil)
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to send Telegram message, status code %d", resp.StatusCode)
	}
	return nil
}
