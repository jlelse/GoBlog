package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/carlmjohnson/requests"
	"go.goblog.app/app/pkgs/builderpool"
)

func (a *goBlog) initTelegram() {
	a.pPostHooks = append(a.pPostHooks, func(p *post) { a.tgPost(p, false) })
	a.pUpdateHooks = append(a.pUpdateHooks, a.tgUpdate)
	a.pDeleteHooks = append(a.pDeleteHooks, a.tgDelete)
	a.pUndeleteHooks = append(a.pUndeleteHooks, func(p *post) { a.tgPost(p, true) })
}

func (tg *configTelegram) enabled() bool {
	if tg == nil || !tg.Enabled || tg.BotToken == "" || tg.ChatID == "" {
		return false
	}
	return true
}

const (
	telegramChatParam = "telegramchat"
	telegramMsgParam  = "telegrammsg"
)

func (a *goBlog) tgPost(p *post, silent bool) {
	if tg := a.getBlogFromPost(p).Telegram; tg.enabled() && p.isPublicPublishedSectionPost() {
		tgChat := p.firstParameter(telegramChatParam)
		tgMsg := p.firstParameter(telegramMsgParam)
		if tgChat != "" && tgMsg != "" {
			// Already posted
			return
		}
		// Generate HTML
		html := tg.generateHTML(p.RenderedTitle, a.shortPostURL(p))
		if html == "" {
			return
		}
		// Send message
		chatId, msgId, err := a.sendTelegram(tg, html, "HTML", silent)
		if err != nil {
			a.error("Failed to send post to Telegram", "err", err)
			return
		}
		if chatId == 0 || msgId == 0 {
			// Not sent
			return
		}
		// Save chat and message id to post
		if err := a.db.replacePostParam(p.Path, telegramChatParam, []string{strconv.FormatInt(chatId, 10)}); err != nil {
			a.error("Failed to save Telegram chat id", "err", err)
		}
		if err := a.db.replacePostParam(p.Path, telegramMsgParam, []string{strconv.Itoa(msgId)}); err != nil {
			a.error("Failed to save Telegram message id", "err", err)
		}
	}
}

func (a *goBlog) tgUpdate(p *post) {
	if tg := a.getBlogFromPost(p).Telegram; tg.enabled() {
		tgChat := p.firstParameter(telegramChatParam)
		tgMsg := p.firstParameter(telegramMsgParam)
		if tgChat == "" || tgMsg == "" {
			// Not send to Telegram
			return
		}
		// Generate HTML
		html := tg.generateHTML(p.RenderedTitle, a.shortPostURL(p))
		if html == "" {
			return
		}
		// Send update
		if err := a.updateTelegram(tg, tgChat, tgMsg, html, "HTML"); err != nil {
			a.error("Failed to send update to Telegram", "err", err)
		}
	}
}

func (a *goBlog) tgDelete(p *post) {
	if tg := a.getBlogFromPost(p).Telegram; tg.enabled() {
		tgChat := p.firstParameter(telegramChatParam)
		tgMsg := p.firstParameter(telegramMsgParam)
		if tgChat == "" || tgMsg == "" {
			// Not send to Telegram
			return
		}
		// Delete message
		if err := a.deleteTelegram(tg, tgChat, tgMsg); err != nil {
			a.error("Failed to delete Telegram message", "err", err)
		}
		// Delete chat and message id from post
		if err := a.db.replacePostParam(p.Path, telegramChatParam, []string{}); err != nil {
			a.error("Failed to remove Telegram chat id", "err", err)
		}
		if err := a.db.replacePostParam(p.Path, telegramMsgParam, []string{}); err != nil {
			a.error("Failed to remove Telegram message id", "err", err)
		}
	}
}

func (tg *configTelegram) generateHTML(title, shortURL string) string {
	if !tg.enabled() {
		return ""
	}
	message := builderpool.Get()
	defer builderpool.Put(message)
	tgReplacer := strings.NewReplacer("<", "&lt;", ">", "&gt;", "&", "&amp;")
	if title != "" {
		tgReplacer.WriteString(message, title)
		message.WriteString("\n\n")
	}
	message.WriteString("<a href=\"" + shortURL + "\">")
	tgReplacer.WriteString(message, shortURL)
	message.WriteString("</a>")
	return message.String()
}

type telegramMessageResult struct {
	OK          bool   `json:"ok"`
	Description string `json:"description"`
	Result      struct {
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		MessageID int `json:"message_id"`
	} `json:"result"`
}

func (a *goBlog) sendTelegram(tg *configTelegram, message, mode string, silent bool) (int64, int, error) {
	if !tg.enabled() {
		return 0, 0, nil
	}

	telegramURL := "https://api.telegram.org/bot" + tg.BotToken + "/sendMessage"
	result := &telegramMessageResult{}
	if err := requests.URL(telegramURL).Client(a.httpClient).
		Param("chat_id", tg.ChatID).
		Param("text", message).
		Param("parse_mode", mode).
		Param("disable_notification", strconv.FormatBool(silent)).
		ToJSON(result).
		Fetch(context.Background()); err != nil {
		return 0, 0, err
	}

	if !result.OK {
		return 0, 0, fmt.Errorf("error from Telegram API: %s", result.Description)
	}

	return result.Result.Chat.ID, result.Result.MessageID, nil
}

func (a *goBlog) updateTelegram(tg *configTelegram, chatID, messageID, message, mode string) error {
	if !tg.enabled() {
		return nil
	}

	telegramURL := "https://api.telegram.org/bot" + tg.BotToken + "/editMessageText"
	result := &telegramMessageResult{}
	if err := requests.URL(telegramURL).Client(a.httpClient).
		Param("chat_id", chatID).
		Param("message_id", messageID).
		Param("text", message).
		Param("parse_mode", mode).
		ToJSON(result).
		Fetch(context.Background()); err != nil {
		return err
	}

	if !result.OK {
		return fmt.Errorf("error from Telegram API: %s", result.Description)
	}

	return nil
}

func (a *goBlog) deleteTelegram(tg *configTelegram, chatID, messageID string) error {
	if !tg.enabled() {
		return nil
	}

	telegramURL := "https://api.telegram.org/bot" + tg.BotToken + "/deleteMessage"
	result := &telegramMessageResult{}
	if err := requests.URL(telegramURL).Client(a.httpClient).
		Param("chat_id", chatID).
		Param("message_id", messageID).
		ToJSON(result).
		Fetch(context.Background()); err != nil {
		return err
	}

	if !result.OK {
		return fmt.Errorf("error from Telegram API: %s", result.Description)
	}

	return nil
}
