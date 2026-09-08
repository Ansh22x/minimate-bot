package handlers

import (
	"fmt"
	"html"
	"log"
	"os"
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
	videoMutex       sync.RWMutex
)

// HandleUpdate processes each incoming update concurrently
func HandleUpdate(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	// 1. Handle Inline Callbacks (Captcha verification & Tabbed Menus)
	if update.CallbackQuery != nil {
		if strings.HasPrefix(update.CallbackQuery.Data, "captcha_verify:") {
			HandleCaptchaCallback(bot, update.CallbackQuery)
			return
		}
		HandleMenuCallback(bot, update.CallbackQuery)
		return
	}

	// Guard against updates with no message
	if update.Message == nil {
		return
	}

	// Automatically track active group in database asynchronously (0ms latency impact)
	go RecordChatActivity(update.Message.Chat)

	// 2. Handle Security Locks (Anti-Link, Anti-Forward, Media blocker)
	if CheckMessageLocks(bot, update.Message) {
		return
	}

	// 3. Handle new members joining (Welcomes & Captcha Challenge)
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

	// 4. Handle left members (Goodbyes)
	if update.Message.LeftChatMember != nil {
		HandleLeftMember(bot, update.Message)
		return
	}

	start := time.Now()

	// 5. Route Commands
	if update.Message.IsCommand() {
		handleCommand(bot, update.Message, start)
		return
	}

	// 6. Route Regular Messages (Filters & Notes trigger)
	handlePassiveFilters(bot, update.Message)
}

// ----------------------------------------------------
// INLINE KEYBOARDS & GREETINGS
// ----------------------------------------------------

// GetStartKeyboard generates the main start menu buttons
func GetStartKeyboard(botUsername string) tgbotapi.InlineKeyboardMarkup {
	addURL := fmt.Sprintf("https://t.me/%s?startgroup=true", botUsername)
	ownerURL := fmt.Sprintf("https://t.me/%s", config.OwnerUsername)

	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("➕ Add Me To Your Group", addURL),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📜 Commands Explorer", "menu_commands"),
			tgbotapi.NewInlineKeyboardButtonData("💎 Premium VIP", "tab_premium"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("👑 Support & Owner", ownerURL),
			tgbotapi.NewInlineKeyboardButtonData("ℹ️ About", "tab_about"),
		),
	)
}

// GetHomeText generates the main welcome greeting
func GetHomeText(firstName string) string {
	return fmt.Sprintf(`╭━━━━━━━━━━━━━━━━━━━━━━╮
   🌸 <b>𝐌𝐢𝐧𝐢𝐌𝐚𝐭𝐞 𝐏𝐫𝐨</b> 🌸
╰━━━━━━━━━━━━━━━━━━━━━━╯

👋 Hey, <b>%s</b>!

<blockquote expandable>🤖 <b>Next-Gen Telegram Group Management</b>
⚡ Fast • Reliable • Zero-Latency
🛡️ Next-Gen Anti-Raid & Security Locks
✨ Premium Animated UI & Smart Math Captcha
📢 Automate • Moderate • Organize</blockquote>

📚 Tap <b>Commands Explorer</b> below to browse all features!`, html.EscapeString(firstName))
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
		startText := GetHomeText(fromFirstName)
		keyboard := GetStartKeyboard(bot.Self.UserName)

		// Attempt to send Intro video with instant caching
		videoMutex.RLock()
		cachedID := startVideoFileID
		videoMutex.RUnlock()

		var sentVideo bool
		if cachedID != "" {
			videoMsg := tgbotapi.NewVideo(chatID, tgbotapi.FileID(cachedID))
			videoMsg.Caption = startText
			videoMsg.ParseMode = "HTML"
			videoMsg.ReplyMarkup = keyboard
			res, err := SafeSend(bot, videoMsg)
			if err == nil && res.MessageID != 0 {
				sentVideo = true
			} else {
				log.Printf("Notice: Cached start video send returned: %v", err)
			}
		}

		if !sentVideo {
			if _, err := os.Stat("Intro.mp4"); err == nil {
				videoMsg := tgbotapi.NewVideo(chatID, tgbotapi.FilePath("Intro.mp4"))
				videoMsg.Caption = startText
				videoMsg.ParseMode = "HTML"
				videoMsg.ReplyMarkup = keyboard
				res, err := SafeSend(bot, videoMsg)
				if err == nil && res.MessageID != 0 {
					sentVideo = true
					if res.Video != nil && res.Video.FileID != "" {
						videoMutex.Lock()
						startVideoFileID = res.Video.FileID
						videoMutex.Unlock()
						log.Printf("✅ Intro video uploaded and cached with FileID: %s", res.Video.FileID)
					}
				} else {
					log.Printf("Notice: Failed to upload Intro.mp4 (%v), falling back to text menu", err)
				}
			}
		}

		if sentVideo {
			sendReply = false
			break
		}

		// Fallback clean text message
		msg := tgbotapi.NewMessage(chatID, startText)
		msg.ParseMode = "HTML"
		msg.ReplyMarkup = keyboard
		SafeSend(bot, msg)
		sendReply = false

	case "help", "commands":
		if message.Chat.IsPrivate() {
			msg := tgbotapi.NewMessage(chatID, `📚 <b>𝐌𝐢𝐧𝐢𝐌𝐚𝐭𝐞 𝐂𝐨𝐦𝐦𝐚𝐧𝐝 𝐄𝐱𝐩𝐥𝐨𝐫𝐞𝐫</b>

<blockquote expandable>Select a category below to explore all available features, moderation commands, and automation tools.</blockquote>`)
			msg.ParseMode = "HTML"
			msg.ReplyMarkup = getCommandsCategoryKeyboard()
			SafeSend(bot, msg)
			sendReply = false
		} else {
			helpText := fmt.Sprintf(`📋 <b>𝐌𝐢𝐧𝐢𝐌𝐚𝐭𝐞 𝐂𝐨𝐦𝐦𝐚𝐧𝐝𝐬 𝐃𝐢𝐫𝐞𝐜𝐭𝐨𝐫𝐲</b>

<blockquote expandable><b>👥 Member Commands:</b>
• <code>/start</code> — Start the bot
• <code>/ping</code> — Check bot speed & latency
• <code>/help</code> — Open commands directory
• <code>/info</code> — View your Telegram account info
• <code>/id</code> — Get your ID or replied user ID
• <code>/rules</code> — Read group rules (or <code>/privaterules</code>)
• <code>/warns</code> — Check your warning strike count
• <code>/filters</code> — List active group auto-reply filters
• <code>/notes</code> — List saved group notes
• <code>/get &lt;name&gt;</code> — Read a saved note
• <code>/premium</code> — Check group VIP subscription

<b>🔨 Admin Moderation:</b>
<i>(Reply to a user to execute)</i>
• <code>/ban</code> / <code>/unban</code> — Permanent ban / unban
• <code>/tban &lt;time&gt;</code> — Temp-ban (e.g. <code>/tban 2h</code>)
• <code>/kick</code> — Kick user from group
• <code>/mute</code> / <code>/unmute</code> — Mute / unmute user
• <code>/tmute &lt;time&gt;</code> — Temp-mute (e.g. <code>/tmute 30m</code>)
• <code>/warn</code> / <code>/dwarn</code> — Strike (3 strikes = auto-ban)
• <code>/unwarn</code> / <code>/rmwarns</code> — Remove / reset warnings
• <code>/promote</code> / <code>/demote</code> — Promote / demote admin

<b>🔐 Security &amp; Locks:</b>
• <code>/lock &lt;type&gt;</code> — <code>links</code>, <code>forwards</code>, <code>stickers</code>, <code>media</code>, <code>bots</code>, <code>all</code>
• <code>/unlock &lt;type&gt;</code> — Unlock specified category
• <code>/locks</code> — View active group lock status
• <code>/captcha &lt;on/off&gt;</code> — Toggle join verification
• <code>/captchamode &lt;button|math&gt;</code> — Set mode
• <code>/captchatime &lt;sec&gt;</code> — Set timeout (30-600s)

<b>🧹 Chat Tools &amp; Greetings:</b>
• <code>/purge</code> / <code>/del</code> — Mass / single delete
• <code>/pin</code> / <code>/unpin</code> / <code>/unpinall</code> — Message pinning
• <code>/welcome &lt;on/off&gt;</code>, <code>/setwelcome</code>, <code>/rmwelcome</code>
• <code>/goodbye &lt;on/off&gt;</code>, <code>/setgoodbye</code>, <code>/rmgoodbye</code>
• <code>/setrules &lt;text&gt;</code> / <code>/clearrules</code> — Rules manager
• <code>/filter &lt;word&gt; &lt;reply&gt;</code> / <code>/stop</code> — Auto-reply filter</blockquote>`)

			msg := tgbotapi.NewMessage(chatID, helpText)
			msg.ParseMode = "HTML"
			pmURL := fmt.Sprintf("https://t.me/%s?start=help", bot.Self.UserName)
			msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonURL("✨ Open Interactive Explorer", pmURL),
				),
			)
			SafeSend(bot, msg)
			sendReply = false
		}

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
	case "setvip":
		HandleSetVIP(bot, message, args)
		sendReply = false

	case "rmvip":
		HandleRmVIP(bot, message, args)
		sendReply = false

	case "viplist", "vips":
		HandleVIPList(bot, message)
		sendReply = false

	case "vip", "premium", "checkvip", "vipstatus":
		HandleVIPStatus(bot, message, args)
		sendReply = false

	// -------------------------
	// BOT ADMINS & SUDO PANEL
	// -------------------------
	case "addbotadmin", "addsudo":
		HandleAddBotAdmin(bot, message, args)
		sendReply = false

	case "rmbotadmin", "delsudo":
		HandleRmBotAdmin(bot, message, args)
		sendReply = false

	case "botadmins", "sudolist", "sudos":
		HandleListBotAdmins(bot, message)
		sendReply = false

	case "leave", "kickme":
		HandleLeave(bot, message, args)
		sendReply = false

	case "broadcast", "gcast", "post":
		HandleBroadcast(bot, message, args)
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
