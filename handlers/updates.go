package handlers

import (
	"fmt"
	"log"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// HandleUpdate processes each incoming update concurrently
func HandleUpdate(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	// Handle new members joining (Welcomes)
	if update.Message != nil && len(update.Message.NewChatMembers) > 0 {
		HandleNewMembers(bot, update.Message)
		return
	}

	// Handle left members (Goodbyes)
	if update.Message != nil && update.Message.LeftChatMember != nil {
		HandleLeftMember(bot, update.Message)
		return
	}

	if update.Message == nil {
		return
	}

	start := time.Now()

	// Route Commands
	if update.Message.IsCommand() {
		handleCommand(bot, update.Message, start)
		return
	}

	// Route Regular Messages (Filters & Notes trigger)
	handlePassiveFilters(bot, update.Message)
}

func handleCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message, start time.Time) {
	command := strings.ToLower(message.Command())
	chatID := message.Chat.ID
	args := message.CommandArguments()

	var reply tgbotapi.MessageConfig
	sendReply := true

	switch command {
	// -------------------------
	// 1. GENERAL / SYSTEM
	// -------------------------
	case "start":
		reply = tgbotapi.NewMessage(chatID, "Hello! I am Minimate, running at maximum compiled speed.")

	case "ping":
		apiStart := time.Now()
		msg := tgbotapi.NewMessage(chatID, "Pinging...")
		msg.ReplyToMessageID = message.MessageID
		sentMsg, err := bot.Send(msg)
		if err == nil {
			apiDuration := time.Since(apiStart).Milliseconds()
			internalLatency := time.Since(start).Milliseconds()
			edit := tgbotapi.NewEditMessageText(
				chatID,
				sentMsg.MessageID,
				fmt.Sprintf("Pong!\n\n• API Roundtrip: %dms\n• Internal Routing: %dms", apiDuration, internalLatency),
			)
			bot.Send(edit)
		}
		sendReply = false

	case "help":
		helpText := "*Minimate Module Directory:*\n\n" +
			"🛡️ `/admin` - Moderation & management\n" +
			"⚠️ `/warns` - Warning configurations\n" +
			"👋 `/welcome` - Greeting setup\n" +
			"📝 `/filters` - Automated text responses\n" +
			"📜 `/rules` - Chat rules directory\n" +
			"🧰 `/purge` - Fast message cleanup\n\n" +
			"Use any command to view its specific parameters."
		reply = tgbotapi.NewMessage(chatID, helpText)
		reply.ParseMode = "Markdown"

	case "info":
		target := message.From
		if message.ReplyToMessage != nil {
			target = message.ReplyToMessage.From
		}
		infoText := fmt.Sprintf("*User Info:*\n• ID: `%d`\n• Username: @%s\n• First Name: %s",
			target.ID, target.UserName, target.FirstName)
		reply = tgbotapi.NewMessage(chatID, infoText)
		reply.ParseMode = "Markdown"

	case "id":
		targetID := message.From.ID
		targetName := message.From.FirstName
		if message.ReplyToMessage != nil {
			targetID = message.ReplyToMessage.From.ID
			targetName = message.ReplyToMessage.From.FirstName
		}
		reply = tgbotapi.NewMessage(chatID, fmt.Sprintf("*%s's ID:* `%d`\n*Chat ID:* `%d`", targetName, targetID, chatID))
		reply.ParseMode = "Markdown"

	case "connect", "disconnect", "connections":
		reply = tgbotapi.NewMessage(chatID, "Remote chat configuration is currently being indexed.")

	// -------------------------
	// 2. ADMIN & MODERATION
	// -------------------------
	case "ban":
		HandleBan(bot, message)
		sendReply = false

	case "tban":
		HandleTBan(bot, message, args)
		sendReply = false

	case "unban":
		HandleUnban(bot, message)
		sendReply = false

	case "mute":
		HandleMute(bot, message)
		sendReply = false

	case "tmute":
		HandleTMute(bot, message, args)
		sendReply = false

	case "unmute":
		HandleUnmute(bot, message)
		sendReply = false

	case "kick":
		HandleKick(bot, message)
		sendReply = false

	case "promote":
		HandlePromote(bot, message)
		sendReply = false

	case "demote":
		HandleDemote(bot, message)
		sendReply = false

	case "adminlist":
		HandleAdminList(bot, message)
		sendReply = false

	case "invitelink":
		HandleInviteLink(bot, message)
		sendReply = false

	case "title":
		HandleTitle(bot, message, args)
		sendReply = false

	// -------------------------
	// 3. WARNINGS
	// -------------------------
	case "warn", "dwarn", "unwarn", "warns", "warnlimit", "warnmode", "rmwarns":
		HandleWarnCommand(bot, message, command, args)
		sendReply = false

	// -------------------------
	// 4. GREETINGS & WELCOMES
	// -------------------------
	case "welcome", "setwelcome", "rmwelcome", "goodbye", "setgoodbye", "rmgoodbye", "welcomeclean", "cleanwelcome":
		HandleGreetingCommand(bot, message, command, args)
		sendReply = false

	// -------------------------
	// 5. NOTES & FILTERS
	// -------------------------
	case "get", "save", "clear", "notes", "filter", "stop", "filters":
		HandleFilterCommand(bot, message, command, args)
		sendReply = false

	// -------------------------
	// 6. LOCKS & ANTI-SPAM
	// -------------------------
	case "lock", "unlock", "locks", "locktypes", "setflood", "floodmode", "antiflood":
		HandleLockCommand(bot, message, command, args)
		sendReply = false

	// -------------------------
	// 7. CAPTCHAS
	// -------------------------
	case "captcha", "captchamode", "captchatime", "captchakick":
		HandleCaptchaCommand(bot, message, command, args)
		sendReply = false

	// -------------------------
	// 8. RULES
	// -------------------------
	case "rules", "setrules", "clearrules", "privaterules":
		HandleRulesCommand(bot, message, command, args)
		sendReply = false

	// -------------------------
	// 9. MISC & CLEANUP
	// -------------------------
	case "purge":
		HandlePurge(bot, message)
		sendReply = false

	case "del":
		HandleDel(bot, message)
		sendReply = false

	case "pin", "unpin", "unpinall":
		HandlePinCommand(bot, message, command)
		sendReply = false

	case "setlang", "description":
		reply = tgbotapi.NewMessage(chatID, fmt.Sprintf("Command /%s configured.", command))

	default:
		sendReply = false
	}

	if sendReply {
		reply.ReplyToMessageID = message.MessageID
		_, err := bot.Send(reply)
		if err != nil {
			log.Printf("Failed to dispatch /%s: %v", command, err)
		}
	}
}

// Temporary stubs for sub-handlers while building module files
func handlePassiveFilters(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {}
func HandleNewMembers(bot *tgbotapi.BotAPI, message *tgbotapi.Message)     {}
func HandleLeftMember(bot *tgbotapi.BotAPI, message *tgbotapi.Message)     {}

// ---------------------------------------------------------
// TEMPORARY STUBS (To prevent compiler errors until built)
// ---------------------------------------------------------
func HandleNewMembers(bot *tgbotapi.BotAPI, message *tgbotapi.Message)    {}
func HandleLeftMember(bot *tgbotapi.BotAPI, message *tgbotapi.Message)    {}
func HandleWarnCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message, cmd string, args string) {}
func HandleGreetingCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message, cmd string, args string) {}
func HandleLockCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message, cmd string, args string) {}
func HandleCaptchaCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message, cmd string, args string) {}
func HandleRulesCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message, cmd string, args string) {}