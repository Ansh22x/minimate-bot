package handlers

import (
	"context"
	"fmt"

	"minimate-bot/database"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// HandleRulesCommand manages group rules
func HandleRulesCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message, cmd string, args string) {
	chatID := message.Chat.ID

	switch cmd {
	case "rules":
		var rulesText string
		err := database.Pool.QueryRow(context.Background(), 
			"SELECT rules_text FROM chat_rules WHERE chat_id = $1", chatID).Scan(&rulesText)

		if err != nil || rulesText == "" {
			bot.Send(tgbotapi.NewMessage(chatID, "📜 There are no rules set for this group yet."))
			return
		}

		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("📜 **Rules for %s:**\n\n%s", message.Chat.Title, rulesText))
		msg.ParseMode = "Markdown"
		bot.Send(msg)

	case "setrules":
		if !isAdmin(bot, chatID, message.From.ID) {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Only admins can set rules."))
			return
		}
		if args == "" {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Usage: `/setrules <text>`"))
			return
		}

		query := `
			INSERT INTO chat_rules (chat_id, rules_text) VALUES ($1, $2)
			ON CONFLICT (chat_id) DO UPDATE SET rules_text = EXCLUDED.rules_text;
		`
		_, err := database.Pool.Exec(context.Background(), query, chatID, args)
		if err == nil {
			bot.Send(tgbotapi.NewMessage(chatID, "✅ Rules have been updated successfully!"))
		}

	case "clearrules":
		if !isAdmin(bot, chatID, message.From.ID) {
			return
		}
		_, err := database.Pool.Exec(context.Background(), "DELETE FROM chat_rules WHERE chat_id = $1", chatID)
		if err == nil {
			bot.Send(tgbotapi.NewMessage(chatID, "🗑️ Rules have been cleared."))
		}
	}
}