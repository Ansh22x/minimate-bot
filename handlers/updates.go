package handlers

import (
	"fmt"
	"html"
	"log"
	"strings"
	"sync"
	"time"

	"minimate-bot/config"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Track bot boot time
var botStartTime = time.Now()

// Cache for Intro video file ID
var (
	startVideoFileID string
	videoFileIDMutex sync.RWMutex
)

// HandleUpdate processes each incoming update concurrently
func HandleUpdate(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	// 1. Handle Captcha Button Taps
	if update.CallbackQuery != nil && strings.HasPrefix(update.CallbackQuery.Data, "captcha_verify:") {
		HandleCaptchaCallback(bot, update.CallbackQuery)
		return
	}

	// 2. Handle Inline Keyboard Menu Tabs
	if update.CallbackQuery != nil {
		handleMenuCallback(bot, update.CallbackQuery)
		return
	}

	// Guard against updates with no message
	if update.Message == nil {
		return
	}

	// Automatically track active group in database
	RecordChatActivity(update.Message.Chat)

	// 3. Handle Security Locks (Anti-Link, Anti-Forward, Media blocker)
	if CheckMessageLocks(bot, update.Message) {
		return // Message was deleted by security shield
	}

	// 4. Handle new members joining (Welcomes & Captcha Challenge)
	if len(update.Message.NewChatMembers) > 0 {
		locks := getLocks(update.Message.Chat.ID)
		for _, newMember := range update.Message.NewChatMembers {
			// If unauthorized bot lock is active, auto-ban the bot
			if newMember.IsBot && locks.LockBots && newMember.ID != bot.Self.ID {
				bot.Request(tgbotapi.BanChatMemberConfig{
					ChatMemberConfig: tgbotapi.ChatMemberConfig{ChatID: update.Message.Chat.ID, UserID: newMember.ID},
				})
				continue
			}

			// Trigger Captcha verification for new human members
			if !newMember.IsBot && newMember.ID != bot.Self.ID {
				HandleCaptchaOnJoin(bot, update.Message.Chat.ID, &newMember)
			}
		}

		HandleNewMembers(bot, update.Message)
		return
	}

	// 5. Handle left members (Goodbyes)
	if update.Message.LeftChatMember != nil {
		HandleLeftMember(bot, update.Message)
		return
	}

	start := time.Now()

	// 6. Route Commands
	if update.Message.IsCommand() {
		handleCommand(bot, update.Message, start)
		return
	}

	// 7. Route Regular Messages (Filters & Notes trigger)
	handlePassiveFilters(bot, update.Message)
}

// ----------------------------------------------------
// INLINE KEYBOARD MENU BUILDERS
// ----------------------------------------------------

func getStartKeyboard(botUsername string) tgbotapi.InlineKeyboardMarkup {
	addURL := fmt.Sprintf("https://t.me/%s?startgroup=true", botUsername)
	ownerURL := fmt.Sprintf("https://t.me/%s", config.OwnerUsername)

	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("👥 Member Commands", "tab_member_cmds"),
			tgbotapi.NewInlineKeyboardButtonData("🔨 Moderation", "tab_admin_mod"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔐 Security & Locks", "tab_admin_locks"),
			tgbotapi.NewInlineKeyboardButtonData("🧹 Tools & Greetings", "tab_admin_tools"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("👑 VIP Status", "tab_vip"),
			tgbotapi.NewInlineKeyboardButtonData("ℹ️ Bot Info", "tab_info"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("➕ Add to Group", addURL),
			tgbotapi.NewInlineKeyboardButtonURL("💬 Contact Support", ownerURL),
		),
	)
}

func getCommandsDirectoryKeyboard(botUsername string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("👥 Member", "tab_member_cmds"),
			tgbotapi.NewInlineKeyboardButtonData("🔨 Moderation", "tab_admin_mod"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔐 Locks & Captcha", "tab_admin_locks"),
			tgbotapi.NewInlineKeyboardButtonData("🧹 Tools & Greetings", "tab_admin_tools"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("👑 VIP Status", "tab_vip"),
			tgbotapi.NewInlineKeyboardButtonData("🔙 « Main Menu", "tab_home"),
		),
	)
}

func getSubmenuKeyboard(botUsername string) tgbotapi.InlineKeyboardMarkup {
	addURL := fmt.Sprintf("https://t.me/%s?startgroup=true", botUsername)

	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("👥 Member Cmds", "tab_member_cmds"),
			tgbotapi.NewInlineKeyboardButtonData("🔨 Moderation", "tab_admin_mod"),
			tgbotapi.NewInlineKeyboardButtonData("🔐 Locks", "tab_admin_locks"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🧹 Tools", "tab_admin_tools"),
			tgbotapi.NewInlineKeyboardButtonData("👑 VIP", "tab_vip"),
			tgbotapi.NewInlineKeyboardButtonData("🔙 « Main Menu", "tab_home"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("➕ Add to Group", addURL),
		),
	)
}

func getHomeText(firstName string) string {
	return fmt.Sprintf(`╭━━━━━━━━━━━━━━━━━━━━╮
   🌸 <b>𝐌𝐢𝐧𝐢𝐌𝐚𝐭𝐞 𝐏𝐫𝐨</b> 🌸
╰━━━━━━━━━━━━━━━━━━━━╯

👋 Hey, <b>%s</b>!

<blockquote expandable>🤖 <b>Next-Gen Telegram Group Management</b>
⚡ Fast • Reliable • Zero-Latency
🛡️ Next-Gen Anti-Raid & Security Locks
✨ Premium Animated UI & Smart Math Captcha</blockquote>

👇 <i>Click any category tab to view commands:</i>`, html.EscapeString(firstName))
}

// ----------------------------------------------------
// INLINE MENU CALLBACK HANDLER
// ----------------------------------------------------

func handleMenuCallback(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
	if query == nil {
		return
	}

	callbackResponse := tgbotapi.NewCallback(query.ID, "")
	bot.Request(callbackResponse)

	if query.Message == nil {
		return
	}

	chatID := query.Message.Chat.ID
	messageID := query.Message.MessageID
	userFirstName := query.From.FirstName
	botUsername := bot.Self.UserName

	var newText string
	var keyboard tgbotapi.InlineKeyboardMarkup

	switch query.Data {
	case "tab_home":
		newText = getHomeText(userFirstName)
		keyboard = getStartKeyboard(botUsername)

	case "tab_commands":
		newText = fmt.Sprintf(`📋 <b>𝐂𝐨𝐦𝐦𝐚𝐧𝐝𝐬 𝐃𝐢𝐫𝐞𝐜𝐭𝐨𝐫𝐲 𝐂𝐚𝐭𝐞𝐠𝐨𝐫𝐢𝐞𝐬</b>

<blockquote expandable>Select a category below to explore specific tools and usage:

• <b>👥 Member Commands:</b> General utilities, ID, rules, notes, filters
• <b>🔨 Moderation:</b> Ban, mute, kick, warns, unban
• <b>🔐 Locks & Captcha:</b> Anti-link, anti-forward, math verification
• <b>🧹 Tools & Greetings:</b> Purge, pin, welcome cards, group rules
• <b>👑 VIP Status:</b> Premium group subscription details</blockquote>`)
		keyboard = getCommandsDirectoryKeyboard(botUsername)

	case "tab_member_cmds":
		newText = fmt.Sprintf(`👥 <b>𝐆𝐞𝐧𝐞𝐫𝐚𝐥 & 𝐌𝐞𝐦𝐛𝐞𝐫 𝐂𝐨𝐦𝐦𝐚𝐧𝐝𝐬</b>

<blockquote expandable>• <code>/start</code> — Main bot menu
• <code>/ping</code> — Latency & status
• <code>/help</code> — Commands directory
• <code>/id</code> / <code>/info</code> — User & chat ID
• <code>/rules</code> — Group rules (or <code>/privaterules</code>)
• <code>/warns</code> — Check warning count
• <code>/filters</code> — Group auto-replies
• <code>/notes</code> / <code>/get &lt;name&gt;</code> — Saved notes
• <code>/premium</code> — VIP status & expiry</blockquote>`)
		keyboard = getCommandsDirectoryKeyboard(botUsername)

	case "tab_admin_cmds":
		newText = fmt.Sprintf(`🛡️ <b>𝐀𝐝𝐦𝐢𝐧 𝐌𝐚𝐬𝐭𝐞𝐫 𝐃𝐢𝐫𝐞𝐜𝐭𝐨𝐫𝐲</b>

<blockquote expandable><b>🔨 Moderation:</b>
• <code>/ban</code>, <code>/tban</code>, <code>/unban</code>, <code>/kick</code>, <code>/mute</code>, <code>/tmute</code>, <code>/unmute</code>
• <code>/warn</code>, <code>/dwarn</code>, <code>/unwarn</code>, <code>/rmwarns</code>
• <code>/promote</code>, <code>/demote</code>

<b>🔐 Security & Protection:</b>
• <code>/lock &lt;type&gt;</code>, <code>/unlock</code>, <code>/locks</code>, <code>/locktypes</code>
• <code>/captcha</code>, <code>/captchamode</code>, <code>/captchatime</code>

<b>🧹 Tools & Messages:</b>
• <code>/purge</code>, <code>/del</code>, <code>/pin</code>, <code>/unpin</code>, <code>/unpinall</code>
• <code>/welcome</code>, <code>/setwelcome</code>, <code>/rmwelcome</code>
• <code>/goodbye</code>, <code>/setgoodbye</code>, <code>/rmgoodbye</code>
• <code>/setrules</code>, <code>/clearrules</code>, <code>/filter</code>, <code>/stop</code></blockquote>`)
		keyboard = getCommandsDirectoryKeyboard(botUsername)

	case "tab_admin_mod":
		newText = fmt.Sprintf(`🔨 <b>𝐌𝐨𝐝𝐞𝐫𝐚𝐭𝐢𝐨𝐧 & 𝐏𝐮𝐧𝐢𝐬𝐡𝐦𝐞𝐧𝐭𝐬</b>
<i>(Reply to a user to execute)</i>

<blockquote expandable>• <code>/ban</code> / <code>/unban</code> — Permanent ban / unban
• <code>/tban &lt;time&gt;</code> — Temp-ban (e.g. <code>/tban 2h</code>)
• <code>/kick</code> — Kick user from group
• <code>/mute</code> / <code>/unmute</code> — Permanent mute / unmute
• <code>/tmute &lt;time&gt;</code> — Temp-mute (e.g. <code>/tmute 30m</code>)
• <code>/warn</code> / <code>/dwarn</code> — Strike (3 = auto-ban)
• <code>/unwarn</code> / <code>/rmwarns</code> — Remove / reset warnings
• <code>/promote</code> / <code>/demote</code> — Promote / demote admin</blockquote>`)
		keyboard = getCommandsDirectoryKeyboard(botUsername)

	case "tab_admin_locks":
		newText = fmt.Sprintf(`🛡️ <b>𝐒𝐞𝐜𝐮𝐫𝐢𝐭𝐲 𝐋𝐨𝐜𝐤𝐬 & 𝐂𝐚𝐩𝐭𝐜𝐡𝐚</b>

<blockquote expandable><b>🔐 Content Locks:</b>
• <code>/lock &lt;type&gt;</code> — <code>links</code>, <code>forwards</code>, <code>stickers</code>, <code>media</code>, <code>bots</code>, <code>all</code>
• <code>/unlock &lt;type&gt;</code> — Unlock specified type
• <code>/locks</code> — View active group locks
• <code>/locktypes</code> — List all lock types

<b>🤖 Smart Captcha:</b>
• <code>/captcha &lt;on/off&gt;</code> — Toggle join verification
• <code>/captchamode &lt;button|math&gt;</code> — Set mode
• <code>/captchatime &lt;sec&gt;</code> — Set timeout (30-600s)</blockquote>`)
		keyboard = getCommandsDirectoryKeyboard(botUsername)

	case "tab_admin_tools":
		newText = fmt.Sprintf(`🧹 <b>𝐂𝐡𝐚𝐭 𝐓𝐨𝐨𝐥𝐬, 𝐂𝐥𝐞𝐚𝐧𝐮𝐩 & 𝐆𝐫𝐞𝐞𝐭𝐢𝐧𝐠𝐬</b>

<blockquote expandable><b>🧹 Tools & Cleanup:</b>
• <code>/purge</code> / <code>/del</code> — Mass / single delete
• <code>/pin</code> / <code>/unpin</code> / <code>/unpinall</code> — Message pinning

<b>🌸 Greetings & Rules:</b>
• <code>/welcome &lt;on/off&gt;</code>, <code>/setwelcome</code>, <code>/rmwelcome</code>
• <code>/goodbye &lt;on/off&gt;</code>, <code>/setgoodbye</code>, <code>/rmgoodbye</code>
• <code>/setrules</code> / <code>/clearrules</code> — Manage rules
• <code>/filter &lt;word&gt; &lt;reply&gt;</code> / <code>/stop</code> — Auto-replies</blockquote>`)
		keyboard = getCommandsDirectoryKeyboard(botUsername)

	case "tab_vip":
		newText = fmt.Sprintf(`👑 <b>𝐕𝐈𝐏 𝐏𝐫𝐞𝐦𝐢𝐮𝐦 𝐒𝐮𝐛𝐬𝐜𝐫𝐢𝐩𝐭𝐢𝐨𝐧</b>

<blockquote expandable><b>💎 Premium Features:</b>
• ⚡ <b>Zero-Latency Engine:</b> Instant filter execution
• 🛡️ <b>Anti-Raid Shield:</b> High-speed join flood defense
• 🎨 <b>Custom Greeting Graphics:</b> Banner cards
• 📊 <b>Unlimited Limits:</b> Unlimited filters & notes

<b>🔍 Check Subscription:</b>
• Use <code>/premium</code> to check status & expiry.

💬 <i>Contact @%s to activate VIP!</i></blockquote>`, html.EscapeString(config.OwnerUsername))
		keyboard = getCommandsDirectoryKeyboard(botUsername)

	case "tab_info":
		uptime := time.Since(botStartTime).Round(time.Second)
		newText = fmt.Sprintf(`ℹ️ <b>𝐁𝐨𝐭 𝐒𝐭𝐚𝐭𝐮𝐬 & 𝐈𝐧𝐟𝐨</b>

<blockquote expandable>🤖 <b>Bot:</b> @%s
⏱️ <b>Uptime:</b> %s
⚡ <b>Engine:</b> Go (Golang) + PostgreSQL
🛡️ <b>Security:</b> Anti-Raid Shield Active
✅ <b>Status:</b> All systems operational</blockquote>`,
			botUsername,
			uptime.String())
		keyboard = getSubmenuKeyboard(botUsername)

	default:
		return
	}

	isMedia := query.Message.Video != nil || query.Message.Photo != nil || query.Message.Animation != nil || query.Message.Document != nil

	if isMedia {
		editCaption := tgbotapi.NewEditMessageCaption(chatID, messageID, newText)
		editCaption.ParseMode = "HTML"
		editCaption.ReplyMarkup = &keyboard
		_, err := SafeSend(bot, editCaption)
		if err != nil && !strings.Contains(err.Error(), "message is not modified") {
			log.Printf("Failed to edit menu caption: %v", err)
			// Fallback: try editing text or sending message if caption editing fails
			editText := tgbotapi.NewEditMessageText(chatID, messageID, newText)
			editText.ParseMode = "HTML"
			editText.ReplyMarkup = &keyboard
			SafeSend(bot, editText)
		}
	} else {
		editText := tgbotapi.NewEditMessageText(chatID, messageID, newText)
		editText.ParseMode = "HTML"
		editText.ReplyMarkup = &keyboard
		_, err := SafeSend(bot, editText)
		if err != nil && !strings.Contains(err.Error(), "message is not modified") {
			log.Printf("Failed to edit menu text: %v", err)
		}
	}
}

// ----------------------------------------------------
// COMMAND ROUTER
// ----------------------------------------------------

func handleCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message, start time.Time) {
	command := strings.ToLower(message.Command())
	chatID := message.Chat.ID
	args := message.CommandArguments()

	var fromID int64
	var fromFirstName string
	var fromUserName string
	if message.From != nil {
		fromID = message.From.ID
		fromFirstName = message.From.FirstName
		fromUserName = message.From.UserName
	} else if message.SenderChat != nil {
		fromID = message.SenderChat.ID
		fromFirstName = message.SenderChat.Title
		fromUserName = message.SenderChat.UserName
	}

	var reply tgbotapi.MessageConfig
	sendReply := true

	switch command {
	// -------------------------
	// 1. GENERAL / SYSTEM
	// -------------------------
	case "start":
		startText := getHomeText(fromFirstName)
		keyboard := getStartKeyboard(bot.Self.UserName)

		videoFileIDMutex.RLock()
		cachedID := startVideoFileID
		videoFileIDMutex.RUnlock()

		if cachedID != "" {
			videoMsg := tgbotapi.NewVideo(chatID, tgbotapi.FileID(cachedID))
			videoMsg.Caption = startText
			videoMsg.ParseMode = "HTML"
			videoMsg.ReplyMarkup = keyboard

			_, err := SafeSend(bot, videoMsg)
			if err != nil {
				fallback := tgbotapi.NewMessage(chatID, startText)
				fallback.ParseMode = "HTML"
				fallback.ReplyMarkup = keyboard
				SafeSend(bot, fallback)
			}
		} else {
			videoMsg := tgbotapi.NewVideo(chatID, tgbotapi.FileID("BAACAgUAAxkDAANbap4sJKeey8ZMTroIvriT5RlPBg8AAncgAALRZ-FUEDTPfZlCwTk9BA"))
			videoMsg.Caption = startText
			videoMsg.ParseMode = "HTML"
			videoMsg.ReplyMarkup = keyboard

			sentMsg, err := SafeSend(bot, videoMsg)
			if err != nil {
				log.Printf("Failed to send start video: %v (falling back to text menu)", err)
				fallback := tgbotapi.NewMessage(chatID, startText)
				fallback.ParseMode = "HTML"
				fallback.ReplyMarkup = keyboard
				SafeSend(bot, fallback)
			} else if sentMsg.Video != nil {
				videoFileIDMutex.Lock()
				startVideoFileID = sentMsg.Video.FileID
				videoFileIDMutex.Unlock()
				log.Printf("✅ SUCCESS! Cached video file ID: %s", sentMsg.Video.FileID)
			}
		}

		sendReply = false

	case "help", "commands":
		helpText := fmt.Sprintf(`%s <b>Minimate Commands Directory</b>

<blockquote expandable>Select a category below to explore specific tools and usage:

• <b>👥 Member Commands:</b> General utilities, ID, rules, notes, filters
• <b>🔨 Moderation:</b> Ban, mute, kick, warns, unban
• <b>🔐 Locks & Captcha:</b> Anti-link, anti-forward, math verification
• <b>🧹 Tools & Greetings:</b> Purge, pin, welcome cards, group rules
• <b>👑 VIP & Owner:</b> Subscription manager and owner controls</blockquote>`, IconRobot)

		msg := tgbotapi.NewMessage(chatID, helpText)
		msg.ParseMode = "HTML"
		msg.ReplyMarkup = getCommandsDirectoryKeyboard(bot.Self.UserName)
		SafeSend(bot, msg)
		sendReply = false

	case "owner", "creator":
		ownerText := fmt.Sprintf(`👨‍💻 <b>𝐌𝐢𝐧𝐢𝐌𝐚𝐭𝐞 𝐎𝐰𝐧𝐞𝐫 & 𝐃𝐞𝐯𝐞𝐥𝐨𝐩𝐞𝐫</b>

<blockquote expandable>%s <b>Developer:</b> @%s
🌐 <b>GitHub:</b> github.com/%s
%s <b>Project:</b> MiniMate Bot</blockquote>`,
			IconCrown, html.EscapeString(config.OwnerUsername),
			html.EscapeString(config.OwnerUsername),
			IconBolt)

		msg := tgbotapi.NewMessage(chatID, ownerText)
		msg.ParseMode = "HTML"
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonURL("💬 Contact Owner", fmt.Sprintf("https://t.me/%s", config.OwnerUsername)),
			),
		)
		SafeSend(bot, msg)
		sendReply = false

	case "dashboard", "stats":
		HandleOwnerDashboard(bot, message)
		sendReply = false

	case "spam", "chats", "groups":
		HandleSpamCommand(bot, message)
		sendReply = false

	case "info":
		var usernameStr string
		if fromUserName != "" {
			usernameStr = "@" + html.EscapeString(fromUserName)
		} else {
			usernameStr = "<i>None</i>"
		}
		infoText := fmt.Sprintf("👤 <b>User Info:</b>\nID: <code>%d</code>\nUsername: %s\nFirst Name: %s",
			fromID, usernameStr, html.EscapeString(fromFirstName))
		reply = tgbotapi.NewMessage(chatID, infoText)
		reply.ParseMode = "HTML"

	case "id":
		targetID := fromID
		targetName := fromFirstName

		if message.ReplyToMessage != nil {
			if message.ReplyToMessage.From != nil {
				targetID = message.ReplyToMessage.From.ID
				targetName = message.ReplyToMessage.From.FirstName
			} else if message.ReplyToMessage.SenderChat != nil {
				targetID = message.ReplyToMessage.SenderChat.ID
				targetName = message.ReplyToMessage.SenderChat.Title
			}
		}

		idText := fmt.Sprintf("👤 <b>%s's ID:</b> <code>%d</code>\n💬 <b>Chat ID:</b> <code>%d</code>",
			html.EscapeString(targetName), targetID, chatID)
		reply = tgbotapi.NewMessage(chatID, idText)
		reply.ParseMode = "HTML"

	case "ping":
		apiStart := time.Now()
		msg := tgbotapi.NewMessage(chatID, "⚡ Pinging...")
		msg.ParseMode = "HTML"
		sentMsg, err := SafeSend(bot, msg)
		if err == nil {
			apiDuration := time.Since(apiStart).Milliseconds()
			internalLatency := time.Since(start).Milliseconds()
			editText := tgbotapi.NewEditMessageText(chatID, sentMsg.MessageID, fmt.Sprintf("⚡ <b>Pong!</b>\n\n• ⏱️ <b>API Roundtrip:</b> <code>%dms</code>\n• 🚀 <b>Internal Routing:</b> <code>%dms</code>", apiDuration, internalLatency))
			editText.ParseMode = "HTML"
			SafeSend(bot, editText)
		}
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
	// 3. VIP & SUBSCRIPTIONS
	// -------------------------
	case "setvip", "rmvip", "checkvip", "vipstatus", "premium":
		HandleVIPCommand(bot, message, command, args)
		sendReply = false

	// -------------------------
	// 4. WARNINGS
	// -------------------------
	case "warn", "dwarn", "unwarn", "warns", "warnlimit", "warnmode", "rmwarns":
		HandleWarnCommand(bot, message, command, args)
		sendReply = false

	// -------------------------
	// 5. GREETINGS & WELCOMES
	// -------------------------
	case "welcome", "setwelcome", "rmwelcome", "goodbye", "setgoodbye", "rmgoodbye", "welcomeclean", "cleanwelcome":
		HandleGreetingCommand(bot, message, command, args)
		sendReply = false

	// -------------------------
	// 6. NOTES & FILTERS
	// -------------------------
	case "get", "save", "clear", "notes", "filter", "stop", "filters":
		HandleFilterCommand(bot, message, command, args)
		sendReply = false

	// -------------------------
	// 7. SECURITY LOCKS & ANTI-SPAM
	// -------------------------
	case "lock", "unlock", "locks", "locktypes", "setflood", "floodmode", "antiflood":
		HandleLockCommand(bot, message, command, args)
		sendReply = false

	// -------------------------
	// 8. CAPTCHAS & VERIFICATION
	// -------------------------
	case "captcha", "captchamode", "captchatime", "captchakick":
		HandleCaptchaCommand(bot, message, command, args)
		sendReply = false

	// -------------------------
	// 9. RULES
	// -------------------------
	case "rules", "setrules", "clearrules", "privaterules":
		HandleRulesCommand(bot, message, command, args)
		sendReply = false

	// -------------------------
	// 10. MISC & CLEANUP
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
		reply = tgbotapi.NewMessage(chatID, fmt.Sprintf("Command /%s is not yet configured.", html.EscapeString(command)))

	default:
		sendReply = false
	}

	if sendReply {
		reply.ReplyToMessageID = message.MessageID
		_, err := SafeSend(bot, reply)
		if err != nil {
			log.Printf("Failed to dispatch /%s: %v", command, err)
		}
	}
}
