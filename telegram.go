package main

import (
	"bytes"
	"errors"
	"net/http"
	"net/url"
)

const telegramBaseURL = "https://api.telegram.org/bot"

func (p *post) tgPost() {
	if appConfig.Blogs[p.Blog].Telegram == nil || !appConfig.Blogs[p.Blog].Telegram.Enabled {
		return
	}
	var message bytes.Buffer
	if title := p.title(); title != "" {
		message.WriteString(title)
		message.WriteString("\n\n")
	}
	message.WriteString(appConfig.Server.PublicAddress + p.Path)
	sendTelegramMessage(message.String(), appConfig.Blogs[p.Blog].Telegram.BotToken, appConfig.Blogs[p.Blog].Telegram.ChatID)
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
	if err != nil || resp.StatusCode != http.StatusOK {
		return errors.New("failed to send Telegram message")
	}
	return nil
}
