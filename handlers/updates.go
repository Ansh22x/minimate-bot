package handlers

import (
	"fmt"
	"log"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Track bot boot time for the /start command
var botStartTime = time.Now()

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
		startText := fmt.Sprintf(`╭━━━━━━━━━━━━━━━━━━━━━━╮
       🌸 𝐌𝐢𝐧𝐢𝐌𝐚𝐭𝐞 🌸
╰━━━━━━━━━━━━━━━━━━━━━━╯

👋 Hey, %s!

🤖 I’m 𝐌𝐢𝐧𝐢𝐌𝐚𝐭𝐞 — your smart
🌐 Telegram Channel Management Assistant.

⚡ Fast • Reliable • Easy to Use
🛡️ Manage your channel with simple commands
📢 Automate • Moderate • Organize

━━━━━━━━━━━━━━━━━━━━━━

💡 𝐖𝐡𝐚𝐭 𝐂𝐚𝐧 𝐈 𝐃𝐨?

🔹 Channel Management
🔹 Auto Moderation
🔹 Admin Tools
🔹 Custom Commands
🔹 Post & Message Management
🔹 And many more useful features!

━━━━━━━━━━━━━━━━━━━━━━

🚀 𝐆𝐞𝐭 𝐒𝐭𝐚𝐫𝐭𝐞𝐝

💡 Use /help to see all available commands.

🌸 𝐌𝐢𝐧𝐢𝐌𝐚𝐭𝐞 — Making Telegram
Management Simple & Easy.`, message.From.FirstName)

		// Create a new video message
		videoMsg := tgbotapi.NewVideo(chatID, tgbotapi.FilePath("Intro.mp4"))
		videoMsg.Caption = startText
		
		// Send the video
		_, err := bot.Send(videoMsg)
		if err != nil {
			log.Printf("Failed to send start video: %v", err)
			// Fallback to text if the video fails to load or path is wrong
			fallback := tgbotapi.NewMessage(chatID, startText)
			bot.Send(fallback)
		}
		
		sendReply = false // Prevent the default text reply handler from firing


	case "help", "commands":
		helpText := `🤖 <b>Minimate Commands</b>

<b>👥 General:</b>
• <code>/start</code>, <code>/ping</code>, <code>/help</code> — Basics
• <code>/info</code>, <code>/id</code> — User & chat details
• <code>/rules</code> — View chat rules
• <code>/warns</code> — Check your strikes
• <code>/filters</code>, <code>/notes</code> — List saved items
• <code>/get &lt;name&gt;</code> — Read a note

<b>🛡️ Admin-Only:</b>
<i>(Strictly restricted to group administrators)</i>

• <b>Mod:</b> <code>/ban</code>, <code>/tban &lt;time&gt;</code>, <code>/unban</code>, <code>/kick</code>
• <b>Mute:</b> <code>/mute</code>, <code>/tmute &lt;time&gt;</code>, <code>/unmute</code>
• <b>Clean:</b> <code>/purge</code>, <code>/del</code>
• <b>Roles:</b> <code>/promote</code>, <code>/demote</code>, <code>/adminlist</code>
• <b>Warns:</b> <code>/warn</code>, <code>/dwarn</code>, <code>/rmwarns</code>
• <b>Chat:</b> <code>/pin</code>, <code>/unpin</code>, <code>/unpinall</code>, <code>/invitelink</code>
• <b>Rules:</b> <code>/setrules</code>, <code>/clearrules</code>
• <b>Greet:</b> <code>/welcome</code>, <code>/goodbye</code>, <code>/setwelcome</code>, <code>/setgoodbye</code>
• <b>Filters:</b> <code>/filter</code>, <code>/stop</code>, <code>/save</code>, <code>/clear</code>`

		msg := tgbotapi.NewMessage(chatID, helpText)
		msg.ParseMode = "HTML"
		bot.Send(msg)
		sendReply = false

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
