package main

import (
	"bytes"
	"log"
	"net/url"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (a *goBlog) initTelegram() {
	a.pPostHooks = append(a.pPostHooks, func(p *post) {
		if tg := a.cfg.Blogs[p.Blog].Telegram; tg.enabled() && p.isPublishedSectionPost() {
			if html := tg.generateHTML(p.RenderedTitle, a.fullPostURL(p), a.shortPostURL(p)); html != "" {
				if _, err := a.send(tg, html, "HTML"); err != nil {
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
	var message bytes.Buffer
	if title != "" {
		message.WriteString(tgbotapi.EscapeText(tgbotapi.ModeHTML, title))
		message.WriteString("\n\n")
	}
	if tg.InstantViewHash != "" {
		message.WriteString("<a href=\"https://t.me/iv?rhash=" + tg.InstantViewHash + "&url=" + url.QueryEscape(fullURL) + "\">")
		message.WriteString(tgbotapi.EscapeText(tgbotapi.ModeHTML, shortURL))
		message.WriteString("</a>")
	} else {
		message.WriteString("<a href=\"" + shortURL + "\">")
		message.WriteString(tgbotapi.EscapeText(tgbotapi.ModeHTML, shortURL))
		message.WriteString("</a>")
	}
	return message.String()
}

func (a *goBlog) send(tg *configTelegram, message, mode string) (int, error) {
	if !tg.enabled() {
		return 0, nil
	}
	bot, err := tgbotapi.NewBotAPIWithClient(tg.BotToken, tgbotapi.APIEndpoint, a.httpClient)
	if err != nil {
		return 0, err
	}
	msg := tgbotapi.MessageConfig{
		BaseChat: tgbotapi.BaseChat{
			ChannelUsername: tg.ChatID,
		},
		Text:      message,
		ParseMode: mode,
	}
	res, err := bot.Send(msg)
	if err != nil {
		return 0, err
	}
	return res.MessageID, nil
}
