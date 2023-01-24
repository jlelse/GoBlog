package main

import (
	"errors"
	"log"
	"net/url"
	"strconv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.goblog.app/app/pkgs/builderpool"
)

func (a *goBlog) initTelegram() {
	a.pPostHooks = append(a.pPostHooks, a.tgPost(false))
	a.pUpdateHooks = append(a.pUpdateHooks, a.tgUpdate)
	a.pDeleteHooks = append(a.pDeleteHooks, a.tgDelete)
	a.pUndeleteHooks = append(a.pUndeleteHooks, a.tgPost(true))
}

func (tg *configTelegram) enabled() bool {
	if tg == nil || !tg.Enabled || tg.BotToken == "" || tg.ChatID == "" {
		return false
	}
	return true
}

func (a *goBlog) tgPost(silent bool) func(*post) {
	return func(p *post) {
		if tg := a.getBlogFromPost(p).Telegram; tg.enabled() && p.isPublicPublishedSectionPost() {
			tgChat := p.firstParameter("telegramchat")
			tgMsg := p.firstParameter("telegrammsg")
			if tgChat != "" && tgMsg != "" {
				// Already posted
				return
			}
			// Generate HTML
			html := tg.generateHTML(p.RenderedTitle, a.fullPostURL(p), a.shortPostURL(p))
			if html == "" {
				return
			}
			// Send message
			chatId, msgId, err := a.sendTelegram(tg, html, tgbotapi.ModeHTML, silent)
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
	}
}

func (a *goBlog) tgUpdate(p *post) {
	if tg := a.getBlogFromPost(p).Telegram; tg.enabled() {
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
		err = a.updateTelegram(tg, chatId, messageId, html, "HTML")
		if err != nil {
			log.Printf("Failed to send update to Telegram: %v", err)
		}
	}
}

func (a *goBlog) tgDelete(p *post) {
	if tg := a.getBlogFromPost(p).Telegram; tg.enabled() {
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
		// Delete message
		err = a.deleteTelegram(tg, chatId, messageId)
		if err != nil {
			log.Printf("Failed to delete Telegram message: %v", err)
		}
		// Delete chat and message id from post
		err = a.db.replacePostParam(p.Path, "telegramchat", []string{})
		if err != nil {
			log.Printf("Failed to remove Telegram chat id: %v", err)
		}
		err = a.db.replacePostParam(p.Path, "telegrammsg", []string{})
		if err != nil {
			log.Printf("Failed to remove Telegram message id: %v", err)
		}
	}
}

func (tg *configTelegram) generateHTML(title, fullURL, shortURL string) (html string) {
	if !tg.enabled() {
		return ""
	}
	message := builderpool.Get()
	defer builderpool.Put(message)
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
	html = message.String()
	return
}

func (a *goBlog) sendTelegram(tg *configTelegram, message, mode string, silent bool) (int64, int, error) {
	if !tg.enabled() {
		return 0, 0, nil
	}
	bot, err := tgbotapi.NewBotAPIWithClient(tg.BotToken, tgbotapi.APIEndpoint, a.httpClient)
	if err != nil {
		return 0, 0, err
	}
	msg := tgbotapi.MessageConfig{
		BaseChat: tgbotapi.BaseChat{
			ChannelUsername:     tg.ChatID,
			DisableNotification: silent,
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

func (a *goBlog) updateTelegram(tg *configTelegram, chatId int64, messageId int, message, mode string) error {
	if !tg.enabled() {
		return nil
	}
	bot, err := tgbotapi.NewBotAPIWithClient(tg.BotToken, tgbotapi.APIEndpoint, a.httpClient)
	if err != nil {
		return err
	}
	// Check if chat is still the configured one
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
	// Send update
	msg := tgbotapi.EditMessageTextConfig{
		BaseEdit: tgbotapi.BaseEdit{
			ChatID:    chatId,
			MessageID: messageId,
		},
		Text:      message,
		ParseMode: mode,
	}
	_, err = bot.Send(msg)
	return err
}

func (a *goBlog) deleteTelegram(tg *configTelegram, chatId int64, messageId int) error {
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
	msg := tgbotapi.DeleteMessageConfig{
		ChatID:    chatId,
		MessageID: messageId,
	}
	_, err = bot.Send(msg)
	return err
}
