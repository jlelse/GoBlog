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
	enable := false
	for _, b := range a.cfg.Blogs {
		if tg := b.Telegram; tg != nil && tg.Enabled && tg.BotToken != "" && tg.ChatID != "" {
			enable = true
		}
	}
	if enable {
		a.pPostHooks = append(a.pPostHooks, func(p *post) {
			if p.isPublishedSectionPost() {
				tgPost(a.cfg.Blogs[p.Blog].Telegram, p.title(), a.fullPostURL(p), a.shortPostURL(p))
			}
		})
	}
}

func tgPost(tg *configTelegram, title, fullURL, shortURL string) {
	if tg == nil || !tg.Enabled || tg.BotToken == "" || tg.ChatID == "" {
		return
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
	if err := sendTelegramMessage(message.String(), "HTML", tg.BotToken, tg.ChatID); err != nil {
		log.Println(err.Error())
	}
}

func sendTelegramMessage(message, mode, token, chat string) error {
	params := url.Values{}
	params.Add("chat_id", chat)
	params.Add("text", message)
	if mode != "" {
		params.Add("parse_mode", mode)
	}
	tgURL, err := url.Parse(telegramBaseURL + token + "/sendMessage")
	if err != nil {
		return errors.New("failed to create Telegram request")
	}
	tgURL.RawQuery = params.Encode()
	req, _ := http.NewRequest(http.MethodPost, tgURL.String(), nil)
	resp, err := appHttpClient.Do(req)
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
