package main

import (
	"errors"
	"log"
	"net/http"
	"net/url"
)

func sendNotification(text string) {
	log.Println("Notification:", text)
	if appConfig.Notifications.Telegram.Enabled {
		err := sendTelegramMessage(text)
		if err != nil {
			log.Println("Failed to send Telegram message:", err.Error())
		}
	}
}

const telegramBaseURL = "https://api.telegram.org/bot"

func sendTelegramMessage(text string) error {
	params := url.Values{}
	params.Add("chat_id", appConfig.Notifications.Telegram.ChatID)
	params.Add("text", text)
	tgURL, err := url.Parse(telegramBaseURL + appConfig.Notifications.Telegram.BotToken + "/sendMessage")
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
