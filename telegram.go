package main

import (
	"bytes"
	"errors"
	"log"
	"net/url"
	"strconv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (a *goBlog) initTelegram() {
	a.pPostHooks = append(a.pPostHooks, func(p *post) {
		if tg := a.cfg.Blogs[p.Blog].Telegram; tg.enabled() && p.isPublishedSectionPost() {
			// Generate HTML
			html := tg.generateHTML(p.RenderedTitle, a.fullPostURL(p), a.shortPostURL(p))
			if html == "" {
				return
			}
			// Send message
			chatId, msgId, err := a.send(tg, html, tgbotapi.ModeHTML)
			if err != nil {
				log.Printf("Failed to send post to Telegram: %v", err)
				return
			}
			if chatId == 0 || msgId == 0 {
				// Not sent
				return
			}
			// Save chat and message id to post
			err = a.db.replacePostParam(p.Path, "telegramchat", []string{strconv.FormatInt(chatId, 10)})
			if err != nil {
				log.Printf("Failed to save Telegram chat id: %v", err)
			}
			err = a.db.replacePostParam(p.Path, "telegrammsg", []string{strconv.Itoa(msgId)})
			if err != nil {
				log.Printf("Failed to save Telegram message id: %v", err)
			}
		}
	})
	a.pUpdateHooks = append(a.pUpdateHooks, func(p *post) {
		if tg := a.cfg.Blogs[p.Blog].Telegram; tg.enabled() && p.isPublishedSectionPost() {
			tgChat := p.firstParameter("telegramchat")
			tgMsg := p.firstParameter("telegrammsg")
			if tgChat == "" || tgMsg == "" {
				// Not send to Telegram
				return
			}
			// Parse tgChat to int64
			chatId, err := strconv.ParseInt(tgChat, 10, 64)
			if err != nil {
				log.Printf("Failed to parse Telegram chat ID: %v", err)
				return
			}
			// Parse tgMsg to int
			messageId, err := strconv.Atoi(tgMsg)
			if err != nil {
				log.Printf("Failed to parse Telegram message ID: %v", err)
				return
			}
			// Generate HTML
			html := tg.generateHTML(p.RenderedTitle, a.fullPostURL(p), a.shortPostURL(p))
			if html == "" {
				return
			}
			// Send update
			err = a.sendUpdate(tg, chatId, messageId, html, "HTML")
			if err != nil {
				log.Printf("Failed to send update to Telegram: %v", err)
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

func (a *goBlog) send(tg *configTelegram, message, mode string) (int64, int, error) {
	if !tg.enabled() {
		return 0, 0, nil
	}
	bot, err := tgbotapi.NewBotAPIWithClient(tg.BotToken, tgbotapi.APIEndpoint, a.httpClient)
	if err != nil {
		return 0, 0, err
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
		return 0, 0, err
	}
	return res.Chat.ID, res.MessageID, nil
}

func (a *goBlog) sendUpdate(tg *configTelegram, chatId int64, messageId int, message, mode string) error {
	if !tg.enabled() {
		return nil
	}
	bot, err := tgbotapi.NewBotAPIWithClient(tg.BotToken, tgbotapi.APIEndpoint, a.httpClient)
	if err != nil {
		return err
	}
	chat, err := bot.GetChat(tgbotapi.ChatInfoConfig{
		ChatConfig: tgbotapi.ChatConfig{
			SuperGroupUsername: tg.ChatID,
		},
	})
	if err != nil {
		return err
	}
	if chat.ID != chatId {
		return errors.New("chat id mismatch")
	}
	msg := tgbotapi.EditMessageTextConfig{
		BaseEdit: tgbotapi.BaseEdit{
			ChatID:    chatId,
			MessageID: messageId,
		},
		Text:      message,
		ParseMode: mode,
	}
	_, err = bot.Send(msg)
	if err != nil {
		return err
	}
	return nil
}
