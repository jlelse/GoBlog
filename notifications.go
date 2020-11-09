package main

import (
	"log"
)

func sendNotification(text string) {
	log.Println("Notification:", text)
	if appConfig.Notifications.Telegram.Enabled {
		err := sendTelegramMessage(text, appConfig.Notifications.Telegram.BotToken, appConfig.Notifications.Telegram.ChatID)
		if err != nil {
			log.Println("Failed to send Telegram notification:", err.Error())
		}
	}
}
