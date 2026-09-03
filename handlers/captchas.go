package handlers

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// HandleCaptchaCommand processes automated human verification
func HandleCaptchaCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message, cmd string, args string) {
	chatID := message.Chat.ID

	if !isAdmin(bot, chatID, message.From.ID) {
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Only admins can configure captchas."))
		return
	}

	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("🤖 **/%s:** Captcha verification and mathematical challenges will be activated in Phase 2.", cmd))
	msg.ParseMode = "Markdown"
	bot.Send(msg)
}