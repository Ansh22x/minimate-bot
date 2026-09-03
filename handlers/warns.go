package handlers

import (
	"context"
	"fmt"

	"minimate-bot/database"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// HandleWarnCommand manages user warnings
func HandleWarnCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message, cmd string, args string) {
	chatID := message.Chat.ID
	isUserAdmin := isAdmin(bot, chatID, message.From.ID)

	switch cmd {
	case "warn", "dwarn":
		if !isUserAdmin {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Only admins can warn users."))
			return
		}
		if message.ReplyToMessage == nil {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Reply to a user to warn them."))
			return
		}

		target := message.ReplyToMessage.From
		var newWarnCount int

		query := `
			INSERT INTO user_warns (chat_id, user_id, warn_count) 
			VALUES ($1, $2, 1)
			ON CONFLICT (chat_id, user_id) 
			DO UPDATE SET warn_count = user_warns.warn_count + 1
			RETURNING warn_count;
		`
		err := database.Pool.QueryRow(context.Background(), query, chatID, target.ID).Scan(&newWarnCount)
		if err != nil {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Database error while issuing warning."))
			return
		}

		reason := "No reason provided."
		if args != "" {
			reason = args
		}

		warnText := fmt.Sprintf("⚠️ <b>%s</b> has been warned.\n<b>Reason:</b> %s\n<b>Warnings:</b> %d/3", target.FirstName, reason, newWarnCount)
		msg := tgbotapi.NewMessage(chatID, warnText)
		msg.ParseMode = "HTML"
		bot.Send(msg)

		// Delete the offending message if the command is /dwarn
		if cmd == "dwarn" {
			bot.Request(tgbotapi.NewDeleteMessage(chatID, message.ReplyToMessage.MessageID))
		}

		// Auto-ban logic if they hit 3 warnings
		if newWarnCount >= 3 {
			banConfig := tgbotapi.BanChatMemberConfig{
				ChatMemberConfig: tgbotapi.ChatMemberConfig{ChatID: chatID, UserID: target.ID},
			}
			bot.Request(banConfig)
			
			banMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("🚫 <b>%s</b> reached 3 warnings and was banned.", target.FirstName))
			banMsg.ParseMode = "HTML"
			bot.Send(banMsg)
			
			// Reset warns after ban
			database.Pool.Exec(context.Background(), "DELETE FROM user_warns WHERE chat_id = $1 AND user_id = $2", chatID, target.ID)
		}

	case "rmwarns":
		if !isUserAdmin {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Only admins can remove warnings."))
			return
		}
		if message.ReplyToMessage == nil {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Reply to a user to reset their warnings."))
			return
		}

		target := message.ReplyToMessage.From
		database.Pool.Exec(context.Background(), "DELETE FROM user_warns WHERE chat_id = $1 AND user_id = $2", chatID, target.ID)
		
		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ All warnings for <b>%s</b> have been removed.", target.FirstName))
		msg.ParseMode = "HTML"
		bot.Send(msg)

	case "warns":
		target := message.From
		if message.ReplyToMessage != nil {
			target = message.ReplyToMessage.From
		}

		var count int
		err := database.Pool.QueryRow(context.Background(), "SELECT warn_count FROM user_warns WHERE chat_id = $1 AND user_id = $2", chatID, target.ID).Scan(&count)
		if err != nil {
			count = 0
		}

		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("⚠️ <b>%s</b> has %d/3 warnings.", target.FirstName, count))
		msg.ParseMode = "HTML"
		bot.Send(msg)
	}
}