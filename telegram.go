package main

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
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
	var message bytes.Buffer
	if title := p.title(); title != "" {
		message.WriteString(title)
		message.WriteString("\n\n")
	}
	message.WriteString(p.shortURL())
	if err := sendTelegramMessage(message.String(), tg.BotToken, tg.ChatID); err != nil {
		log.Println(err.Error())
	}
}

func sendTelegramMessage(text, bottoken, chatID string) error {
	params := url.Values{}
	params.Add("chat_id", chatID)
	params.Add("text", text)
	tgURL, err := url.Parse(telegramBaseURL + bottoken + "/sendMessage")
	if err != nil {
		return errors.New("failed to create Telegram request")
	}
	tgURL.RawQuery = params.Encode()
	req, _ := http.NewRequest(http.MethodPost, tgURL.String(), nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	} else if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to send Telegram message, status code %d", resp.StatusCode)
	}
	return nil
}
