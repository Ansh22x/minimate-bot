package handlers

import (
	"context"
	"fmt"
	"html"
	"strings"

	"minimate-bot/database"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// HandleNewMembers triggers when someone joins the chat
func HandleNewMembers(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	chatID := message.Chat.ID

	var welcomeEnabled bool
	var welcomeText string

	err := database.Pool.QueryRow(context.Background(),
		"SELECT welcome_enabled, welcome_text FROM chat_greetings WHERE chat_id = $1", chatID).
		Scan(&welcomeEnabled, &welcomeText)

	if err != nil || !welcomeEnabled || strings.TrimSpace(welcomeText) == "" {
		return
	}

	for _, newMember := range message.NewChatMembers {
		// Do not welcome the bot itself
		if newMember.ID == bot.Self.ID {
			continue
		}

		username := newMember.UserName
		if username != "" {
			username = "@" + username
		}

		text := strings.ReplaceAll(welcomeText, "{first}", newMember.FirstName)
		text = strings.ReplaceAll(text, "{username}", username)
		text = strings.ReplaceAll(text, "{id}", fmt.Sprintf("%d", newMember.ID))
		text = strings.ReplaceAll(text, "{chatname}", message.Chat.Title)

		bot.Send(tgbotapi.NewMessage(chatID, text))
	}
}

// HandleLeftMember triggers when someone leaves
func HandleLeftMember(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	if message.LeftChatMember == nil {
		return
	}

	chatID := message.Chat.ID

	var goodbyeEnabled bool
	var goodbyeText string

	err := database.Pool.QueryRow(context.Background(),
		"SELECT goodbye_enabled, goodbye_text FROM chat_greetings WHERE chat_id = $1", chatID).
		Scan(&goodbyeEnabled, &goodbyeText)

	if err != nil || !goodbyeEnabled || strings.TrimSpace(goodbyeText) == "" {
		return
	}

	leftMember := message.LeftChatMember
	username := leftMember.UserName
	if username != "" {
		username = "@" + username
	}

	text := strings.ReplaceAll(goodbyeText, "{first}", leftMember.FirstName)
	text = strings.ReplaceAll(text, "{username}", username)
	text = strings.ReplaceAll(text, "{id}", fmt.Sprintf("%d", leftMember.ID))
	text = strings.ReplaceAll(text, "{chatname}", message.Chat.Title)

	bot.Send(tgbotapi.NewMessage(chatID, text))
}

// HandleGreetingCommand processes /welcome, /setwelcome, /rmwelcome, /goodbye, etc.
func HandleGreetingCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message, cmd string, args string) {
	chatID := message.Chat.ID
	fromID := int64(0)
	if message.From != nil {
		fromID = message.From.ID
	} else if message.SenderChat != nil {
		fromID = message.SenderChat.ID
	}

	if !isAdmin(bot, chatID, fromID) {
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Only admins can configure greetings."))
		return
	}

	switch cmd {
	case "welcome", "goodbye":
		if strings.TrimSpace(args) == "" {
			msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Usage: <code>/%s &lt;on/off&gt;</code>", html.EscapeString(cmd)))
			msg.ParseMode = "HTML"
			bot.Send(msg)
			return
		}

		argLower := strings.ToLower(strings.TrimSpace(args))
		isEnabled := argLower == "on" || argLower == "yes" || argLower == "true" || argLower == "1"
		column := cmd + "_enabled"

		query := fmt.Sprintf(`
			INSERT INTO chat_greetings (chat_id, %[1]s) VALUES ($1, $2)
			ON CONFLICT (chat_id) DO UPDATE SET %[1]s = EXCLUDED.%[1]s;
		`, column)

		_, err := database.Pool.Exec(context.Background(), query, chatID, isEnabled)
		if err == nil {
			statusStr := "OFF"
			if isEnabled {
				statusStr = "ON"
			}
			msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ <b>%s</b> is now turned <b>%s</b>.", html.EscapeString(cmd), statusStr))
			msg.ParseMode = "HTML"
			bot.Send(msg)
		} else {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Database error updating greeting settings."))
		}

	case "setwelcome", "setgoodbye":
		if strings.TrimSpace(args) == "" {
			msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Usage: <code>/%s &lt;text&gt;</code>\nVariables: <code>{first}</code>, <code>{username}</code>, <code>{id}</code>, <code>{chatname}</code>", html.EscapeString(cmd)))
			msg.ParseMode = "HTML"
			bot.Send(msg)
			return
		}

		targetColumn := "welcome_text"
		targetEnabled := "welcome_enabled"
		if cmd == "setgoodbye" {
			targetColumn = "goodbye_text"
			targetEnabled = "goodbye_enabled"
		}

		query := fmt.Sprintf(`
			INSERT INTO chat_greetings (chat_id, %[1]s, %[2]s) VALUES ($1, $2, true)
			ON CONFLICT (chat_id) DO UPDATE SET %[1]s = EXCLUDED.%[1]s, %[2]s = true;
		`, targetColumn, targetEnabled)

		_, err := database.Pool.Exec(context.Background(), query, chatID, args)
		if err == nil {
			msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ <b>%s</b> message saved and enabled!", html.EscapeString(cmd)))
			msg.ParseMode = "HTML"
			bot.Send(msg)
		} else {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Database error saving greeting message."))
		}

	case "rmwelcome", "rmgoodbye":
		targetColumn := "welcome_text"
		targetEnabled := "welcome_enabled"
		if cmd == "rmgoodbye" {
			targetColumn = "goodbye_text"
			targetEnabled = "goodbye_enabled"
		}

		query := fmt.Sprintf(`
			INSERT INTO chat_greetings (chat_id, %[1]s, %[2]s) VALUES ($1, NULL, false)
			ON CONFLICT (chat_id) DO UPDATE SET %[1]s = NULL, %[2]s = false;
		`, targetColumn, targetEnabled)

		_, err := database.Pool.Exec(context.Background(), query, chatID)
		if err == nil {
			msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("🗑️ <b>%s</b> has been removed and disabled.", html.EscapeString(cmd)))
			msg.ParseMode = "HTML"
			bot.Send(msg)
		} else {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Database error removing greeting."))
		}

	case "welcomeclean", "cleanwelcome":
		msg := tgbotapi.NewMessage(chatID, "🧹 Welcome message auto-cleanup is scheduled for Phase 2.")
		bot.Send(msg)
	}
}