package telegrambot

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/carlmjohnson/requests"
	"go.goblog.app/app/pkgs/plugintypes"
)

type plugin struct {
	app plugintypes.App

	// Telegram bot token
	botToken string
	// Allowed Telegram users
	allowedUsers []string
	// Generated authorization token for the bot
	authorizationToken string
}

func GetPlugin() (
	plugintypes.SetApp,
	plugintypes.SetConfig,
	plugintypes.Middleware,
	plugintypes.Exec,
) {
	p := &plugin{}
	return p, p, p, p
}

func (p *plugin) SetApp(app plugintypes.App) {
	p.app = app
}

func (p *plugin) SetConfig(config map[string]any) {
	if botToken, ok := config["token"]; ok {
		if botTokenStr, ok := botToken.(string); ok {
			p.botToken = botTokenStr
		}
	}
	if allowedUsers, ok := config["allowed"]; ok {
		if allowedUsersStr, ok := allowedUsers.(string); ok {
			p.allowedUsers = strings.Split(allowedUsersStr, ",")
		}
	}
}

func (p *plugin) Exec() {
	go func() {
		// Delay execution by 10 seconds to allow the app to start
		time.Sleep(10 * time.Second)
		// Generate a random alphanumeric authorization token for the bot
		charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
		b := make([]byte, 16)
		for i := range b {
			randomByte := make([]byte, 1)
			_, err := rand.Read(randomByte)
			if err != nil {
				log.Println("telegrambot: Error generating authorization token:", err)
				return
			}
			b[i] = charset[randomByte[0]%byte(len(charset))]
		}
		p.authorizationToken = string(b)
		// Register the bot with the Telegram API
		err := requests.URL(fmt.Sprintf("https://api.telegram.org/bot%s/getMe", p.botToken)).
			Method(http.MethodGet).
			Fetch(context.Background())
		if err != nil {
			log.Println("telegrambot: Error registering bot:", err)
			return
		}
		// Register the bot with GoBlog
		err = requests.URL(fmt.Sprintf("https://api.telegram.org/bot%s/setWebhook", p.botToken)).
			Method(http.MethodPost).
			BodyJSON(map[string]any{
				"url": p.app.GetFullAddress("/x/telegrambot/" + p.authorizationToken),
			}).
			Fetch(context.Background())
		if err != nil {
			log.Println("telegrambot: Error setting webhook:", err)
			return
		}
		// Log the authorization token
		log.Println("telegrambot: Authorization token: " + p.authorizationToken)
		// Log the allowed users
		log.Println("telegrambot: Allowed users: " + strings.Join(p.allowedUsers, ", "))
	}()
}

func (p *plugin) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/x/telegrambot/"+p.authorizationToken {
			p.handleTelegramBotRequest(w, r)
		} else {
			next.ServeHTTP(w, r)
		}
	})
}

func (p *plugin) Prio() int {
	return 1000
}

type TelegramMessage struct {
	MessageID int           `json:"message_id,omitempty"`
	From      *TelegramUser `json:"from,omitempty"`
	Chat      *TelegramChat `json:"chat,omitempty"`
	// For text messages
	Text string `json:"text,omitempty"`
	// For files
	Document *TelegramDocument `json:"document,omitempty"`
}

type TelegramDocument struct {
	FileID   string `json:"file_id,omitempty"`
	FileName string `json:"file_name,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
}

type TelegramUser struct {
	ID       int    `json:"id,omitempty"`
	Username string `json:"username,omitempty"`
}

type TelegramChat struct {
	ID int `json:"id,omitempty"`
}

type TelegramSendMessage struct {
	ChatID          int                      `json:"chat_id,omitempty"`
	Text            string                   `json:"text,omitempty"`
	ReplyParameters *TelegramReplyParameters `json:"reply_parameters,omitempty"`
}

type TelegramReplyParameters struct {
	MessageID int `json:"message_id,omitempty"`
}

func (p *plugin) handleTelegramBotRequest(_ http.ResponseWriter, r *http.Request) {
	// Decode the incoming message
	var update struct {
		Message *TelegramMessage `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		log.Println("telegrambot: Error decoding update:", err)
		return
	}
	if update.Message == nil {
		return
	}
	// Check allowed users
	if !(slices.Contains(p.allowedUsers, update.Message.From.Username) || slices.Contains(p.allowedUsers, strconv.Itoa(update.Message.From.ID))) {
		p.sendMessage(update.Message.Chat.ID, "Sorry, you are not allowed to use this bot!", update.Message.MessageID)
		return
	}
	// Check if the message is a command
	if strings.HasPrefix(update.Message.Text, "/") {
		parts := strings.SplitN(update.Message.Text, " ", 2)
		command := parts[0][1:]
		if command == "start" {
			p.sendMessage(update.Message.Chat.ID, "Welcome to the GoBlog Telegram Bot!", update.Message.MessageID)
		} else if command == "help" {
			p.sendMessage(update.Message.Chat.ID, "Available commands: /start, /help", update.Message.MessageID)
		} else {
			p.sendMessage(update.Message.Chat.ID, "Unknown command: "+command, update.Message.MessageID)
		}
		return
	} else if update.Message.Document != nil {
		// Handle files: Download the file and upload it to the media storage
		// Get file path
		var fileResponse struct {
			OK     bool `json:"ok"`
			Result struct {
				FilePath string `json:"file_path"`
			} `json:"result"`
		}
		err := requests.URL(fmt.Sprintf("https://api.telegram.org/bot%s/getFile", p.botToken)).
			Param("file_id", update.Message.Document.FileID).
			ToJSON(&fileResponse).
			Fetch(context.Background())
		if err != nil {
			log.Println("telegrambot: Error getting file path:", err)
			p.sendMessage(update.Message.Chat.ID, "Error getting file path. Please check the logs.", update.Message.MessageID)
			return
		}
		// Download the file
		fileDownload := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", p.botToken, fileResponse.Result.FilePath)
		fileReader, fileWriter := io.Pipe()
		go func() {
			fileWriter.CloseWithError(
				requests.URL(fileDownload).
					ToWriter(fileWriter).
					Fetch(context.Background()),
			)
		}()
		// Upload the file to the media storage
		location, err := p.app.UploadMedia(fileReader, update.Message.Document.FileName, update.Message.Document.MimeType)
		if err != nil {
			log.Println("telegrambot: Error uploading file:", err)
			p.sendMessage(update.Message.Chat.ID, "Error uploading file. Please check the logs.", update.Message.MessageID)
			return
		}
		// Send back the file URL
		p.sendMessage(update.Message.Chat.ID, "File uploaded: "+location, update.Message.MessageID)
	} else if update.Message.Text != "" {
		// Handle regular messages: Take the text and create a post
		post, err := p.app.CreatePost(update.Message.Text)
		if err != nil {
			log.Println("telegrambot: Error creating post:", err)
			p.sendMessage(update.Message.Chat.ID, "Error creating the post. Please check the logs.", update.Message.MessageID)
			return
		}
		// Send back the post URL
		p.sendMessage(update.Message.Chat.ID, "Post created: "+p.app.GetFullAddress(post.GetPath()), update.Message.MessageID)
	}
}

func (p *plugin) sendMessage(chatID int, text string, replyTo int) {
	msg := &TelegramSendMessage{
		ChatID: chatID,
		Text:   text,
	}
	if replyTo != 0 {
		msg.ReplyParameters = &TelegramReplyParameters{
			MessageID: replyTo,
		}
	}
	err := requests.URL(fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", p.botToken)).
		Method(http.MethodPost).
		BodyJSON(msg).
		Fetch(context.Background())
	if err != nil {
		log.Println("Error sending message:", err)
		return
	}
}
