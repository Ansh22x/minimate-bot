package handlers

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// handleCallback processes inline button clicks
func handleCallback(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
	// Acknowledge button click to remove loading state
	bot.Request(tgbotapi.NewCallback(query.ID, ""))

	var editMsg tgbotapi.EditMessageTextConfig

	switch query.Data {
	case "help_menu":
		// Construct the massive grid
		grid := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("👮 Admin", "help_admin"),
				tgbotapi.NewInlineKeyboardButtonData("🔨 Bans", "help_bans"),
				tgbotapi.NewInlineKeyboardButtonData("🔇 Mutes", "help_mutes"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("⚠️ Warns", "help_warns"),
				tgbotapi.NewInlineKeyboardButtonData("📝 Notes", "help_notes"),
				tgbotapi.NewInlineKeyboardButtonData("🔍 Filters", "help_filters"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("👋 Greetings", "help_greet"),
				tgbotapi.NewInlineKeyboardButtonData("📜 Rules", "help_rules"),
				tgbotapi.NewInlineKeyboardButtonData("🔒 Locks", "help_locks"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🌊 AntiFlood", "help_flood"),
				tgbotapi.NewInlineKeyboardButtonData("🚫 Blocklists", "help_block"),
				tgbotapi.NewInlineKeyboardButtonData("✅ Approval", "help_approve"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("⬅️ Back", "start_menu"),
			),
		)

		// Changed to NewEditMessageText
		editMsg = tgbotapi.NewEditMessageText(
			query.Message.Chat.ID,
			query.Message.MessageID,
			"🤖 **MiniMate Command Modules:**\nSelect a module below to see available commands.",
		)
		editMsg.ReplyMarkup = &grid
		editMsg.ParseMode = "Markdown"

	case "start_menu":
		// Return to original start menu
		startText := fmt.Sprintf("👋 Hey, %s!\n\n🤖 I’m 𝐌𝐢𝐧𝐢𝐌𝐚𝐭𝐞 — your smart\n🌐 Telegram Channel Management Assistant.", query.From.FirstName)
		
		markup := getStartMarkup(bot.Self.UserName)
		// Changed to NewEditMessageText
		editMsg = tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, startText)
		editMsg.ReplyMarkup = &markup
	}

	bot.Send(editMsg)
}

// getStartMarkup generates the main start menu buttons
func getStartMarkup(botUsername string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("➕ Add me to Group", fmt.Sprintf("https://t.me/%s?startgroup=true", botUsername)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📜 Commands", "help_menu"),
			tgbotapi.NewInlineKeyboardButtonURL("🌐 Website", "https://yourwebsite.com"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("👑 My Lord", "https://t.me/yourusername"), // Update to your TG handle
			tgbotapi.NewInlineKeyboardButtonURL("📢 Channel", "https://t.me/yourchannel"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("ℹ️ About", "about_bot"),
		),
	)
}