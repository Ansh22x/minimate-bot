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
			sendHTMLMessage(bot, chatID, "❌ Only admins can warn users.")
			return
		}

		target, reason := ExtractTargetUser(bot, message, args)
		if target == nil {
			sendHTMLMessage(bot, chatID, "❌ Reply to a user's message or specify their <code>@username</code> / User ID to warn them.\n\n<i>Example:</i> <code>/warn @username [reason]</code> or <code>/warn [reason]</code> (as reply)")
			return
		}

		if target.ID == bot.Self.ID {
			sendHTMLMessage(bot, chatID, "❌ I cannot warn myself.")
			return
		}
		if isAdmin(bot, chatID, target.ID) {
			sendHTMLMessage(bot, chatID, "❌ You cannot issue warnings to an administrator.")
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
			sendHTMLMessage(bot, chatID, "❌ Database error while issuing warning.")
			return
		}

		reasonText := "No reason provided."
		if strings.TrimSpace(reason) != "" {
			reasonText = strings.TrimSpace(reason)
		}

		warnText := fmt.Sprintf("⚠️ <b>%s</b> has been warned.\n<b>Reason:</b> %s\n<b>Warnings:</b> %d/3",
			html.EscapeString(target.FirstName), html.EscapeString(reasonText), newWarnCount)
		sendHTMLMessage(bot, chatID, warnText)

		// Delete the offending message if the command is /dwarn and it was a reply
		if cmd == "dwarn" && message.ReplyToMessage != nil {
			bot.Request(tgbotapi.NewDeleteMessage(chatID, message.ReplyToMessage.MessageID))
		}

		// Auto-ban logic if they hit 3 warnings
		if newWarnCount >= 3 {
			banConfig := tgbotapi.BanChatMemberConfig{
				ChatMemberConfig: tgbotapi.ChatMemberConfig{ChatID: chatID, UserID: target.ID},
			}
			bot.Request(banConfig)

			banMsg := fmt.Sprintf("🚫 <b>%s</b> reached 3 warnings and was banned.", html.EscapeString(target.FirstName))
			sendHTMLMessage(bot, chatID, banMsg)

			// Reset warns after ban
			database.Pool.Exec(context.Background(), "DELETE FROM user_warns WHERE chat_id = $1 AND user_id = $2", chatID, target.ID)
		}

	case "unwarn", "rmwarn", "delwarn", "removewarn", "remwarn", "unwarns":
		if !isUserAdmin {
			sendHTMLMessage(bot, chatID, "❌ Only admins can remove warnings.")
			return
		}

		target, _ := ExtractTargetUser(bot, message, args)
		if target == nil {
			sendHTMLMessage(bot, chatID, "❌ Reply to a user's message or specify their <code>@username</code> / User ID to remove a warning.\n\n<i>Example:</i> <code>/unwarn @username</code> or <code>/rmwarn</code> (as reply)")
			return
		}

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

		sendHTMLMessage(bot, chatID, fmt.Sprintf("✅ Removed a warning for <b>%s</b>.\n<b>Warnings:</b> %d/3", html.EscapeString(target.FirstName), count))

	case "rmwarns", "resetwarns", "delwarns", "clearwarns", "resetwarn", "clearwarn", "removewarns", "removeallwarns":
		if !isUserAdmin {
			sendHTMLMessage(bot, chatID, "❌ Only admins can reset warnings.")
			return
		}

		target, _ := ExtractTargetUser(bot, message, args)
		if target == nil {
			sendHTMLMessage(bot, chatID, "❌ Reply to a user's message or specify their <code>@username</code> / User ID to reset all warnings.\n\n<i>Example:</i> <code>/resetwarns @username</code> or <code>/rmwarns</code> (as reply)")
			return
		}

		database.Pool.Exec(context.Background(), "DELETE FROM user_warns WHERE chat_id = $1 AND user_id = $2", chatID, target.ID)

		sendHTMLMessage(bot, chatID, fmt.Sprintf("✅ All warnings for <b>%s</b> have been reset to <code>0/3</code>.", html.EscapeString(target.FirstName)))

	case "warns":
		target, _ := ExtractTargetUser(bot, message, args)
		if target == nil {
			if message.From != nil {
				target = message.From
			} else {
				sendHTMLMessage(bot, chatID, "❌ Unable to determine user identity.")
				return
			}
		}

		var count int
		err := database.Pool.QueryRow(context.Background(), "SELECT warn_count FROM user_warns WHERE chat_id = $1 AND user_id = $2", chatID, target.ID).Scan(&count)
		if err != nil {
			count = 0
		}

		sendHTMLMessage(bot, chatID, fmt.Sprintf("⚠️ <b>%s</b> has %d/3 warnings.", html.EscapeString(target.FirstName), count))

	case "warnlimit", "warnmode":
		sendHTMLMessage(bot, chatID, fmt.Sprintf("⚙️ Custom <b>/%s</b> configurations are scheduled for Phase 2.", html.EscapeString(cmd)))
	}
}