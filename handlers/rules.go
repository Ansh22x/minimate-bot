package handlers

import (
	"context"
	"fmt"
	"html"
	"strings"

	"minimate-bot/database"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// HandleRulesCommand manages group rules
func HandleRulesCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message, cmd string, args string) {
	chatID := message.Chat.ID
	fromID := int64(0)
	if message.From != nil {
		fromID = message.From.ID
	} else if message.SenderChat != nil {
		fromID = message.SenderChat.ID
	}

	switch cmd {
	case "rules":
		var rulesText string
		err := database.Pool.QueryRow(context.Background(),
			"SELECT rules_text FROM chat_rules WHERE chat_id = $1", chatID).Scan(&rulesText)

		if err != nil || strings.TrimSpace(rulesText) == "" {
			msg := tgbotapi.NewMessage(chatID, "📜 There are no rules set for this group yet.")
			msg.ParseMode = "HTML"
			SafeSend(bot, msg)
			return
		}

		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("📜 <b>Rules for %s:</b>\n\n%s",
			html.EscapeString(message.Chat.Title), html.EscapeString(rulesText)))
		msg.ParseMode = "HTML"
		SafeSend(bot, msg)

	case "setrules":
		if !isAdmin(bot, chatID, fromID) {
			msg := tgbotapi.NewMessage(chatID, "❌ Only admins can set rules.")
			msg.ParseMode = "HTML"
			SafeSend(bot, msg)
			return
		}
		if strings.TrimSpace(args) == "" {
			msg := tgbotapi.NewMessage(chatID, "❌ Usage: <code>/setrules &lt;text&gt;</code>")
			msg.ParseMode = "HTML"
			SafeSend(bot, msg)
			return
		}

		query := `
			INSERT INTO chat_rules (chat_id, rules_text) VALUES ($1, $2)
			ON CONFLICT (chat_id) DO UPDATE SET rules_text = EXCLUDED.rules_text;
		`
		_, err := database.Pool.Exec(context.Background(), query, chatID, strings.TrimSpace(args))
		if err == nil {
			msg := tgbotapi.NewMessage(chatID, "✅ Rules have been updated successfully!")
			msg.ParseMode = "HTML"
			SafeSend(bot, msg)
		} else {
			msg := tgbotapi.NewMessage(chatID, "❌ Database error updating rules.")
			msg.ParseMode = "HTML"
			SafeSend(bot, msg)
		}

	case "clearrules":
		if !isAdmin(bot, chatID, fromID) {
			msg := tgbotapi.NewMessage(chatID, "❌ Only admins can clear rules.")
			msg.ParseMode = "HTML"
			SafeSend(bot, msg)
			return
		}
		_, err := database.Pool.Exec(context.Background(), "DELETE FROM chat_rules WHERE chat_id = $1", chatID)
		if err == nil {
			msg := tgbotapi.NewMessage(chatID, "🗑️ Rules have been cleared.")
			msg.ParseMode = "HTML"
			SafeSend(bot, msg)
		}

	case "privaterules":
		if message.From == nil {
			msg := tgbotapi.NewMessage(chatID, "❌ Cannot send private rules to a channel.")
			msg.ParseMode = "HTML"
			SafeSend(bot, msg)
			return
		}

		var rulesText string
		err := database.Pool.QueryRow(context.Background(),
			"SELECT rules_text FROM chat_rules WHERE chat_id = $1", chatID).Scan(&rulesText)

		if err != nil || strings.TrimSpace(rulesText) == "" {
			msg := tgbotapi.NewMessage(chatID, "📜 There are no rules set for this group yet.")
			msg.ParseMode = "HTML"
			SafeSend(bot, msg)
			return
		}

		pmMsg := tgbotapi.NewMessage(message.From.ID, fmt.Sprintf("📜 <b>Rules for %s:</b>\n\n%s",
			html.EscapeString(message.Chat.Title), html.EscapeString(rulesText)))
		pmMsg.ParseMode = "HTML"
		_, err = SafeSend(bot, pmMsg)
		if err != nil {
			msg := tgbotapi.NewMessage(chatID, "❌ Could not PM you rules. Please start a private chat with me first.")
			msg.ParseMode = "HTML"
			SafeSend(bot, msg)
		} else {
			msg := tgbotapi.NewMessage(chatID, "📬 Rules have been sent to your PM.")
			msg.ParseMode = "HTML"
			SafeSend(bot, msg)
		}
	}
}