package handlers

import (
	"context"
	"fmt"
	"html"
	"strings"

	"minimate-bot/database"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// HandleWarnCommand manages user warnings
func HandleWarnCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message, cmd string, args string) {
	chatID := message.Chat.ID
	fromID := int64(0)
	if message.From != nil {
		fromID = message.From.ID
	} else if message.SenderChat != nil {
		fromID = message.SenderChat.ID
	}
	isUserAdmin := isAdmin(bot, chatID, fromID)

	switch cmd {
	case "warn", "dwarn":
		if !isUserAdmin {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Only admins can warn users."))
			return
		}
		if message.ReplyToMessage == nil || message.ReplyToMessage.From == nil {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Reply to a valid user message to warn them."))
			return
		}

		target := message.ReplyToMessage.From
		if target.ID == bot.Self.ID {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ I cannot warn myself."))
			return
		}
		if isAdmin(bot, chatID, target.ID) {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ You cannot issue warnings to an administrator."))
			return
		}

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
		if strings.TrimSpace(args) != "" {
			reason = strings.TrimSpace(args)
		}

		warnText := fmt.Sprintf("⚠️ <b>%s</b> has been warned.\n<b>Reason:</b> %s\n<b>Warnings:</b> %d/3",
			html.EscapeString(target.FirstName), html.EscapeString(reason), newWarnCount)
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

			banMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("🚫 <b>%s</b> reached 3 warnings and was banned.", html.EscapeString(target.FirstName)))
			banMsg.ParseMode = "HTML"
			bot.Send(banMsg)

			// Reset warns after ban
			database.Pool.Exec(context.Background(), "DELETE FROM user_warns WHERE chat_id = $1 AND user_id = $2", chatID, target.ID)
		}

	case "unwarn":
		if !isUserAdmin {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Only admins can remove warnings."))
			return
		}
		if message.ReplyToMessage == nil || message.ReplyToMessage.From == nil {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Reply to a user to reduce their warnings."))
			return
		}

		target := message.ReplyToMessage.From
		var count int
		query := `
			UPDATE user_warns 
			SET warn_count = GREATEST(0, warn_count - 1) 
			WHERE chat_id = $1 AND user_id = $2 
			RETURNING warn_count;
		`
		err := database.Pool.QueryRow(context.Background(), query, chatID, target.ID).Scan(&count)
		if err != nil {
			count = 0
		}

		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Removed a warning for <b>%s</b>. Warnings: %d/3", html.EscapeString(target.FirstName), count))
		msg.ParseMode = "HTML"
		bot.Send(msg)

	case "rmwarns":
		if !isUserAdmin {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Only admins can reset warnings."))
			return
		}
		if message.ReplyToMessage == nil || message.ReplyToMessage.From == nil {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Reply to a user to reset their warnings."))
			return
		}

		target := message.ReplyToMessage.From
		database.Pool.Exec(context.Background(), "DELETE FROM user_warns WHERE chat_id = $1 AND user_id = $2", chatID, target.ID)

		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ All warnings for <b>%s</b> have been reset.", html.EscapeString(target.FirstName)))
		msg.ParseMode = "HTML"
		bot.Send(msg)

	case "warns":
		var target *tgbotapi.User
		if message.ReplyToMessage != nil && message.ReplyToMessage.From != nil {
			target = message.ReplyToMessage.From
		} else if message.From != nil {
			target = message.From
		}

		if target == nil {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Unable to determine user identity."))
			return
		}

		var count int
		err := database.Pool.QueryRow(context.Background(), "SELECT warn_count FROM user_warns WHERE chat_id = $1 AND user_id = $2", chatID, target.ID).Scan(&count)
		if err != nil {
			count = 0
		}

		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("⚠️ <b>%s</b> has %d/3 warnings.", html.EscapeString(target.FirstName), count))
		msg.ParseMode = "HTML"
		bot.Send(msg)

	case "warnlimit", "warnmode":
		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("⚙️ Custom <b>/%s</b> configurations are scheduled for Phase 2.", html.EscapeString(cmd)))
		msg.ParseMode = "HTML"
		bot.Send(msg)
	}
}