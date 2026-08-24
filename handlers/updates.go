package handlers

import (
	"fmt"
	"log"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// HandleUpdate processes a single message on its own thread
func HandleUpdate(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	if update.Message == nil {
		return
	}

	start := time.Now()

	// Route Commands
	if update.Message.IsCommand() {
		handleCommand(bot, update.Message, start)
		return
	}
}

func handleCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message, start time.Time) {
	command := message.Command()
	chatID := message.Chat.ID

	var reply tgbotapi.MessageConfig
	sendReply := true

	switch command {
	// -------------------------
	// 1. GENERAL COMMANDS
	// -------------------------
	case "start":
		reply = tgbotapi.NewMessage(chatID, "Hello! I am Minimate, running at maximum compiled speed.")
	
	case "help":
		helpText := "🤖 **Minimate Modules:**\n\n" +
			"🛡️ Admin & Moderation\n" +
			"⚠️ Warnings\n" +
			"👋 Greetings & Welcomes\n" +
			"📝 Notes & Filters\n" +
			"🛑 Locks & Anti-Spam\n" +
			"🤖 Captchas\n" +
			"📜 Rules\n" +
			"🧰 Misc & Cleanup\n\n" +
			"More detailed help coming soon!"
		reply = tgbotapi.NewMessage(chatID, helpText)
		reply.ParseMode = "Markdown"
	
	case "info":
		infoText := fmt.Sprintf("👤 **User Info:**\nID: `%d`\nUsername: @%s\nFirst Name: %s", 
			message.From.ID, message.From.UserName, message.From.FirstName)
		reply = tgbotapi.NewMessage(chatID, infoText)
		reply.ParseMode = "Markdown"
	
	case "id":
		targetID := message.From.ID
		targetName := message.From.FirstName

		// If replying to someone, get their ID instead
		if message.ReplyToMessage != nil {
			targetID = message.ReplyToMessage.From.ID
			targetName = message.ReplyToMessage.From.FirstName
		}
		
		idText := fmt.Sprintf("👤 **%s's ID:** `%d`\n💬 **Chat ID:** `%d`", targetName, targetID, chatID)
		reply = tgbotapi.NewMessage(chatID, idText)
		reply.ParseMode = "Markdown"
	
	case "ping":
		apiStart := time.Now()
		msg := tgbotapi.NewMessage(chatID, "Pinging...")
		sentMsg, err := bot.Send(msg)
		if err == nil {
			apiDuration := time.Since(apiStart).Milliseconds()
			internalLatency := time.Since(start).Milliseconds()
			bot.Send(tgbotapi.NewEditMessageText(chatID, sentMsg.MessageID, fmt.Sprintf("Pong!\n\n• API Roundtrip: %dms\n• Internal Routing: %dms", apiDuration, internalLatency)))
		}
		sendReply = false // We handled sending inside the case

	case "connect", "disconnect", "connections":
		reply = tgbotapi.NewMessage(chatID, "Connection features will be available once the database is linked.")

	// -------------------------
	// 2. ADMIN & MODERATION
	// -------------------------
	case "promote", "demote", "adminlist", "invitelink", "title", "ban", "tban", "unban", "mute", "tmute", "unmute", "kick":
		reply = tgbotapi.NewMessage(chatID, fmt.Sprintf("Command /%s received. (Admin privileges & DB logic pending)", command))

	// -------------------------
	// 3. WARNINGS
	// -------------------------
	case "warn", "dwarn", "unwarn", "warns", "warnlimit", "warnmode", "rmwarns":
		reply = tgbotapi.NewMessage(chatID, fmt.Sprintf("Command /%s received. (Database logic pending)", command))

	// -------------------------
	// 4. GREETINGS & WELCOMES
	// -------------------------
	case "welcome", "setwelcome", "rmwelcome", "goodbye", "setgoodbye", "rmgoodbye", "welcomeclean", "cleanwelcome":
		reply = tgbotapi.NewMessage(chatID, fmt.Sprintf("Command /%s received. (Database logic pending)", command))

	// -------------------------
	// 5. NOTES & FILTERS
	// -------------------------
	case "get", "save", "clear", "notes", "filter", "stop", "filters":
		reply = tgbotapi.NewMessage(chatID, fmt.Sprintf("Command /%s received. (Database logic pending)", command))

	// -------------------------
	// 6. LOCKS & ANTI-SPAM
	// -------------------------
	case "lock", "unlock", "locks", "locktypes", "setflood", "floodmode", "antiflood":
		reply = tgbotapi.NewMessage(chatID, fmt.Sprintf("Command /%s received. (Database logic pending)", command))

	// -------------------------
	// 7. CAPTCHAS
	// -------------------------
	case "captcha", "captchamode", "captchatime", "captchakick":
		reply = tgbotapi.NewMessage(chatID, fmt.Sprintf("Command /%s received. (Database logic pending)", command))

	// -------------------------
	// 8. RULES
	// -------------------------
	case "rules", "setrules", "clearrules", "privaterules":
		reply = tgbotapi.NewMessage(chatID, fmt.Sprintf("Command /%s received. (Database logic pending)", command))

	// -------------------------
	// 9. MISC & CLEANUP
	// -------------------------
	case "purge", "del", "pin", "unpin", "unpinall", "setlang", "description":
		reply = tgbotapi.NewMessage(chatID, fmt.Sprintf("Command /%s received. (Admin privileges logic pending)", command))

	default:
		sendReply = false // Unknown command, ignore it
	}

	// Send the reply if a response was generated
	if sendReply {
		reply.ReplyToMessageID = message.MessageID
		_, err := bot.Send(reply)
		if err != nil {
			log.Printf("Failed to send reply for /%s: %v", command, err)
		}
	}
}
