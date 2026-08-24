package handlers

import (
	"log"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// HandleUpdate processes a single message on its own thread
func HandleUpdate(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	if update.Message == nil {
		return
	}

	// Route Commands
	if update.Message.IsCommand() {
		handleCommand(bot, update.Message)
		return
	}
}

func handleCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	var reply tgbotapi.MessageConfig

	switch message.Command() {
	case "start":
		reply = tgbotapi.NewMessage(message.Chat.ID, "🌊 Hello! I am Minimate, running at maximum compiled speed.")
	case "ping":
		reply = tgbotapi.NewMessage(message.Chat.ID, "🏓 Pong! Zero latency.")
	default:
		return // Ignore unknown commands
	}

	_, err := bot.Send(reply)
	if err != nil {
		log.Printf("Failed to send message: %v", err)
	}
}