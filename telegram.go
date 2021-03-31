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

func initTelegram() {
	enable := false
	for _, b := range appConfig.Blogs {
		if tg := b.Telegram; tg != nil && tg.Enabled && tg.BotToken != "" && tg.ChatID != "" {
			enable = true
		}
	}
	if enable {
		postHooks[postPostHook] = append(postHooks[postPostHook], func(p *post) {
			if p.isPublishedSectionPost() {
				p.tgPost()
			}
		})
	}
}

func (p *post) tgPost() {
	tg := appConfig.Blogs[p.Blog].Telegram
	if tg == nil || !tg.Enabled || tg.BotToken == "" || tg.ChatID == "" {
		return
	}
	replacer := strings.NewReplacer("<", "&lt;", ">", "&gt;", "&", "&amp;")
	var message bytes.Buffer
	if title := p.title(); title != "" {
		message.WriteString(replacer.Replace(title))
		message.WriteString("\n\n")
	}
	if tg.InstantViewHash != "" {
		message.WriteString("<a href=\"https://t.me/iv?rhash=" + tg.InstantViewHash + "&url=" + url.QueryEscape(p.fullURL()) + "\">")
		message.WriteString(replacer.Replace(p.shortURL()))
		message.WriteString("</a>")
	} else {
		message.WriteString("<a href=\"" + p.shortURL() + "\">")
		message.WriteString(replacer.Replace(p.shortURL()))
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
