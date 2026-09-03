package handlers

import (
	"context"
	"fmt"
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

	if err != nil || !welcomeEnabled || welcomeText == "" {
		return
	}

	for _, newMember := range message.NewChatMembers {
		// Basic variable replacement
		text := strings.ReplaceAll(welcomeText, "{first}", newMember.FirstName)
		text = strings.ReplaceAll(text, "{username}", newMember.UserName)
		text = strings.ReplaceAll(text, "{id}", fmt.Sprintf("%d", newMember.ID))
		text = strings.ReplaceAll(text, "{chatname}", message.Chat.Title)

		bot.Send(tgbotapi.NewMessage(chatID, text))
	}
}

// HandleLeftMember triggers when someone leaves
func HandleLeftMember(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	chatID := message.Chat.ID

	var goodbyeEnabled bool
	var goodbyeText string

	err := database.Pool.QueryRow(context.Background(), 
		"SELECT goodbye_enabled, goodbye_text FROM chat_greetings WHERE chat_id = $1", chatID).
		Scan(&goodbyeEnabled, &goodbyeText)

	if err != nil || !goodbyeEnabled || goodbyeText == "" {
		return
	}

	leftMember := message.LeftChatMember
	text := strings.ReplaceAll(goodbyeText, "{first}", leftMember.FirstName)
	text = strings.ReplaceAll(text, "{username}", leftMember.UserName)

	bot.Send(tgbotapi.NewMessage(chatID, text))
}

// HandleGreetingCommand processes /welcome, /setwelcome, etc.
func HandleGreetingCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message, cmd string, args string) {
	chatID := message.Chat.ID

	if !isAdmin(bot, chatID, message.From.ID) {
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Only admins can configure greetings."))
		return
	}

	switch cmd {
	case "welcome", "goodbye":
		if args == "" {
			bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Usage: `/%s <on/off>`", cmd)))
			return
		}
		
		isEnabled := strings.ToLower(args) == "on" || strings.ToLower(args) == "yes"
		column := cmd + "_enabled"

		query := fmt.Sprintf(`
			INSERT INTO chat_greetings (chat_id, %[1]s) VALUES ($1, $2)
			ON CONFLICT (chat_id) DO UPDATE SET %[1]s = EXCLUDED.%[1]s;
		`, column)

		_, err := database.Pool.Exec(context.Background(), query, chatID, isEnabled)
		if err == nil {
			bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ **%s** is now set to: %t", cmd, isEnabled)))
		}

	case "setwelcome", "setgoodbye":
		if args == "" {
			bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Usage: `/%s <text>`\nYou can use {first}, {username}, {chatname}", cmd)))
			return
		}

		targetColumn := "welcome_text"
		if cmd == "setgoodbye" {
			targetColumn = "goodbye_text"
		}

		query := fmt.Sprintf(`
			INSERT INTO chat_greetings (chat_id, %[1]s) VALUES ($1, $2)
			ON CONFLICT (chat_id) DO UPDATE SET %[1]s = EXCLUDED.%[1]s;
		`, targetColumn)

		_, err := database.Pool.Exec(context.Background(), query, chatID, args)
		if err == nil {
			bot.Send(tgbotapi.NewMessage(chatID, "✅ Greeting message saved successfully!"))
		}
	}
}