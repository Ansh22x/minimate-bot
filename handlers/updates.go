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
			tgbotapi.NewInlineKeyboardButtonData("🛡️ Admin Commands", "tab_admin_cmds"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔐 Security & Locks", "tab_admin_locks"),
			tgbotapi.NewInlineKeyboardButtonData("👑 VIP & Owner", "tab_admin_vip"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("👨‍💻 Owner Profile", "tab_owner"),
			tgbotapi.NewInlineKeyboardButtonData("ℹ️ Bot Info", "tab_info"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("➕ Add to Group", addURL),
			tgbotapi.NewInlineKeyboardButtonURL("💬 Contact Owner", ownerURL),
		),
	)
}

func getCommandsDirectoryKeyboard(botUsername string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("👥 Member Commands", "tab_member_cmds"),
			tgbotapi.NewInlineKeyboardButtonData("🔨 Moderation", "tab_admin_mod"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔐 Locks & Captcha", "tab_admin_locks"),
			tgbotapi.NewInlineKeyboardButtonData("🧹 Tools & Greetings", "tab_admin_tools"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("👑 VIP & Owner", "tab_admin_vip"),
			tgbotapi.NewInlineKeyboardButtonData("🔙 « Main Menu", "tab_home"),
		),
	)
}

func getSubmenuKeyboard(botUsername string) tgbotapi.InlineKeyboardMarkup {
	addURL := fmt.Sprintf("https://t.me/%s?startgroup=true", botUsername)

	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("👥 Member Cmds", "tab_member_cmds"),
			tgbotapi.NewInlineKeyboardButtonData("🛡️ Admin Cmds", "tab_admin_cmds"),
			tgbotapi.NewInlineKeyboardButtonData("🔐 Locks", "tab_admin_locks"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 « Main Menu", "tab_home"),
			tgbotapi.NewInlineKeyboardButtonURL("➕ Add to Group", addURL),
		),
	)
}

func getHomeText(firstName string) string {
	return fmt.Sprintf(`╭━━━━━━━━━━━━━━━━━━━━━━╮
   %s <b>𝐌𝐢𝐧𝐢𝐌𝐚𝐭𝐞 𝐏𝐫𝐨</b> %s
╰━━━━━━━━━━━━━━━━━━━━━━╯

👋 Hey, <b>%s</b>!

<blockquote expandable>%s <b>I’m MiniMate Pro</b> — your high-performance, next-generation Telegram Group Management Assistant.

%s <b>Fast • Reliable • Zero-Latency</b>
%s Next-Gen Anti-Raid & Security Locks
%s Premium Animated UI & Smart Math Captcha
📢 Automate • Moderate • Organize</blockquote>

%s <b>Interactive Keyboard Navigation:</b>
• Browse all <b>Member Commands</b>, <b>Admin Moderation</b> & <b>Security Locks</b> directly using the buttons below!

━━━━━━━━━━━━━━━━━━━━━━
👇 <i>Click any category tab to view complete commands directory:</i>`,
		IconFlower, IconFlower,
		html.EscapeString(firstName),
		IconRobot,
		IconBolt,
		IconShield,
		IconCrown,
		IconSparkles)
}

// ----------------------------------------------------
// INLINE MENU CALLBACK HANDLER
// ----------------------------------------------------

func handleMenuCallback(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
	callbackResponse := tgbotapi.NewCallback(query.ID, "")
	bot.Request(callbackResponse)

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
		newText = `📋 <b>𝐂𝐨𝐦𝐦𝐚𝐧𝐝𝐬 𝐃𝐢𝐫𝐞𝐜𝐭𝐨𝐫𝐲 𝐂𝐚𝐭𝐞𝐠𝐨𝐫𝐢𝐞𝐬</b>

<blockquote expandable>Select a category below to explore specific tools and usage:

• <b>👥 Member Commands:</b> General utilities, ID, rules, notes, filters
• <b>🔨 Moderation:</b> Ban, mute, kick, warns, unban
• <b>🔐 Locks & Captcha:</b> Anti-link, anti-forward, math verification
• <b>🧹 Tools & Greetings:</b> Purge, pin, welcome cards, group rules
• <b>👑 VIP & Owner:</b> Subscription manager and owner controls</blockquote>`
		keyboard = getCommandsDirectoryKeyboard(botUsername)

	case "tab_member_cmds":
		newText = `👥 <b>𝐆𝐞𝐧𝐞𝐫𝐚𝐥 & 𝐌𝐞𝐦𝐛𝐞𝐫 𝐂𝐨𝐦𝐦𝐚𝐧𝐝𝐬</b>

<blockquote expandable>• <code>/start</code> — Open main bot menu
• <code>/ping</code> — Check bot latency & status
• <code>/help</code> — Open commands directory
• <code>/info</code> — View your Telegram ID & account details
• <code>/id</code> — Get your ID or replied user's ID
• <code>/rules</code> — Read current group rules
• <code>/privaterules</code> — Receive chat rules directly in PM
• <code>/warns</code> — Check your warning strike count
• <code>/filters</code> — List all active group auto-reply filters
• <code>/notes</code> — List saved group notes
• <code>/get &lt;name&gt;</code> — Fetch and read a saved note
• <code>/premium</code> — Check group VIP status & expiry</blockquote>`
		keyboard = getCommandsDirectoryKeyboard(botUsername)

	case "tab_admin_cmds":
		newText = `🛡️ <b>𝐀𝐝𝐦𝐢𝐧 𝐌𝐚𝐬𝐭𝐞𝐫 𝐃𝐢𝐫𝐞𝐜𝐭𝐨𝐫𝐲</b>

<blockquote expandable><b>🔨 Moderation:</b>
• <code>/ban</code>, <code>/tban &lt;time&gt;</code>, <code>/unban</code>, <code>/kick</code>
• <code>/mute</code>, <code>/tmute &lt;time&gt;</code>, <code>/unmute</code>
• <code>/warn</code>, <code>/dwarn</code>, <code>/unwarn</code>, <code>/rmwarns</code>

<b>🔐 Security & Protection:</b>
• <code>/lock &lt;type&gt;</code>, <code>/unlock &lt;type&gt;</code>, <code>/locks</code>
• <code>/captcha &lt;on/off&gt;</code>, <code>/captchamode &lt;button|math&gt;</code>

<b>🧹 Tools & Messages:</b>
• <code>/purge</code>, <code>/del</code>, <code>/pin</code>, <code>/unpin</code>, <code>/unpinall</code>
• <code>/setrules</code>, <code>/clearrules</code>, <code>/welcome</code>, <code>/setwelcome</code></blockquote>`
		keyboard = getCommandsDirectoryKeyboard(botUsername)

	case "tab_admin_mod":
		newText = `🔨 <b>𝐌𝐨𝐝𝐞𝐫𝐚𝐭𝐢𝐨𝐧 & 𝐏𝐮𝐧𝐢𝐬𝐡𝐦𝐞𝐧𝐭𝐬</b>
<i>(Reply to a user's message to execute)</i>

<blockquote expandable>• <code>/ban</code> — Permanently bans replied user
• <code>/tban &lt;time&gt;</code> — Temporary ban (e.g. <code>/tban 2h</code>, <code>/tban 1d</code>)
• <code>/unban</code> — Unbans replied user
• <code>/kick</code> — Kicks user out of the group
• <code>/mute</code> — Permanently mutes replied user
• <code>/tmute &lt;time&gt;</code> — Temporary mute (e.g. <code>/tmute 30m</code>, <code>/tmute 2h</code>)
• <code>/unmute</code> — Restores all chat permissions
• <code>/warn [reason]</code> — Issues a warning strike (auto-ban at 3)
• <code>/dwarn [reason]</code> — Deletes message and issues a warning strike
• <code>/unwarn</code> — Removes 1 warning strike
• <code>/rmwarns</code> — Resets all strikes for replied user</blockquote>`
		keyboard = getCommandsDirectoryKeyboard(botUsername)

	case "tab_admin_locks":
		newText = fmt.Sprintf(`%s <b>𝐒𝐞𝐜𝐮𝐫𝐢𝐭𝐲 𝐋𝐨𝐜𝐤𝐬 & 𝐂𝐚𝐩𝐭𝐜𝐡𝐚</b>

<blockquote expandable><b>🔐 Content Locks:</b>
• <code>/lock links</code> — Deletes URLs, invites and web links
• <code>/lock forwards</code> — Blocks forwarded messages
• <code>/lock stickers</code> — Blocks stickers and GIF animations
• <code>/lock media</code> — Blocks photos, videos, files and voice notes
• <code>/lock bots</code> — Automatically bans newly added userbots
• <code>/lock all</code> — Enables all security locks at once
• <code>/unlock &lt;type&gt;</code> — Unlocks specified category
• <code>/locks</code> — View active group lock status

<b>🤖 Automated Captcha:</b>
• <code>/captcha &lt;on/off&gt;</code> — Enable/disable join verification
• <code>/captchamode &lt;button|math&gt;</code> — Set one-tap button or math puzzle
• <code>/captchatime &lt;seconds&gt;</code> — Set timeout before auto-kick (30-600s)</blockquote>`, IconShield)
		keyboard = getCommandsDirectoryKeyboard(botUsername)

	case "tab_admin_tools":
		newText = `🧹 <b>𝐂𝐡𝐚𝐭 𝐓𝐨𝐨𝐥𝐬, 𝐂𝐥𝐞𝐚𝐧𝐮𝐩 & 𝐆𝐫𝐞𝐞𝐭𝐢𝐧𝐠𝐬</b>

<blockquote expandable><b>🧹 Cleanup & Pinning:</b>
• <code>/purge</code> — Reply to a message to delete all messages down to command
• <code>/del</code> — Deletes replied message
• <code>/pin</code> — Pins replied message
• <code>/unpin</code> — Unpins replied message
• <code>/unpinall</code> — Unpins all pinned messages in group

<b>🌸 Welcome & Goodbye Cards:</b>
• <code>/welcome &lt;on/off&gt;</code> — Toggle join greetings
• <code>/setwelcome &lt;text&gt;</code> — Set custom card (tags: <code>{first}</code>, <code>{username}</code>, <code>{chatname}</code>)
• <code>/rmwelcome</code> — Remove and disable welcome card
• <code>/goodbye &lt;on/off&gt;</code> — Toggle leave messages
• <code>/setgoodbye &lt;text&gt;</code> — Set custom goodbye message

<b>📜 Rules & Filters:</b>
• <code>/setrules &lt;text&gt;</code> — Save group rules
• <code>/clearrules</code> — Remove group rules
• <code>/filter &lt;word&gt; &lt;reply&gt;</code> — Add instant 0ms auto-reply filter
• <code>/stop &lt;word&gt;</code> — Remove an auto-reply filter</blockquote>`
		keyboard = getCommandsDirectoryKeyboard(botUsername)

	case "tab_admin_vip":
		newText = fmt.Sprintf(`%s <b>𝐕𝐈𝐏 𝐒𝐮𝐛𝐬𝐜𝐫𝐢𝐩𝐭𝐢𝐨𝐧𝐬 & 𝐎𝐰𝐧𝐞𝐫 𝐓𝐨𝐨𝐥𝐬</b>

<blockquote expandable><b>👑 VIP & Subscription Management:</b>
• <code>/premium</code> (or <code>/checkvip</code>) — Check group VIP status & expiry
• <code>/setvip &lt;chat_id&gt; &lt;days&gt;</code> — Activate VIP Pro for a group (0 for lifetime)
• <code>/rmvip &lt;chat_id&gt;</code> — Revoke VIP status

<b>👨‍💻 Bot Owner Exclusive Commands:</b>
• <code>/dashboard</code> (or <code>/stats</code>) — Live bot performance & database metrics
• <code>/spam</code> (or <code>/chats</code>) — Complete directory of all active groups</blockquote>`, IconCrown)
		keyboard = getCommandsDirectoryKeyboard(botUsername)

	case "tab_owner":
		newText = fmt.Sprintf(`👨‍💻 <b>𝐌𝐢𝐧𝐢𝐌𝐚𝐭𝐞 𝐎𝐰𝐧𝐞𝐫 & 𝐃𝐞𝐯𝐞𝐥𝐨𝐩𝐞𝐫 𝐏𝐫𝐨𝐟𝐢𝐥𝐞</b>

<blockquote expandable>%s <b>Developer:</b> @%s
🌐 <b>Project:</b> MiniMate Bot
%s <b>Tech Stack:</b> Go (Golang) + Supabase PostgreSQL
%s <b>Support & Business:</b> Contact directly for custom bot integrations, VIP activations & partnerships.</blockquote>

👇 <i>Click below to direct message or explore developer channels:</i>`,
			IconCrown, html.EscapeString(config.OwnerUsername),
			IconBolt,
			IconShield)
		keyboard = getSubmenuKeyboard(botUsername)

	case "tab_info":
		uptime := time.Since(botStartTime).Round(time.Second)
		newText = fmt.Sprintf(`ℹ️ <b>𝐁𝐨𝐭 𝐒𝐭𝐚𝐭𝐮𝐬 & 𝐈𝐧𝐟𝐨</b>

<blockquote expandable>%s <b>Bot:</b> @%s
⏱️ <b>Uptime:</b> %s
%s <b>Engine:</b> Go (Golang) + Supabase PostgreSQL
%s <b>Security:</b> Active Anti-Raid Shield
%s <b>Status:</b> All systems operational</blockquote>`,
			IconRobot, botUsername,
			uptime.String(),
			IconBolt,
			IconShield,
			IconCheck)
		keyboard = getSubmenuKeyboard(botUsername)

	default:
		return
	}

	if query.Message.Video != nil || query.Message.Photo != nil {
		editCaption := tgbotapi.NewEditMessageCaption(chatID, messageID, newText)
		editCaption.ParseMode = "HTML"
		editCaption.ReplyMarkup = &keyboard
		_, err := bot.Send(editCaption)
		if err != nil {
			log.Printf("Failed to edit menu caption: %v", err)
		}
	} else {
		editText := tgbotapi.NewEditMessageText(chatID, messageID, newText)
		editText.ParseMode = "HTML"
		editText.ReplyMarkup = &keyboard
		_, err := bot.Send(editText)
		if err != nil {
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

			_, err := bot.Send(videoMsg)
			if err != nil {
				fallback := tgbotapi.NewMessage(chatID, startText)
				fallback.ParseMode = "HTML"
				fallback.ReplyMarkup = keyboard
				bot.Send(fallback)
			}
		} else {
			videoMsg := tgbotapi.NewVideo(chatID, tgbotapi.FileID("BAACAgUAAxkDAAMmapu-vHWqXcAkgRXpttuUgKLfR_AAAncgAALRZ-FUEDTPfZlCwTk9BA"))
			videoMsg.Caption = startText
			videoMsg.ParseMode = "HTML"
			videoMsg.ReplyMarkup = keyboard

			sentMsg, err := bot.Send(videoMsg)
			if err != nil {
				log.Printf("Failed to send start video: %v (falling back to text menu)", err)
				fallback := tgbotapi.NewMessage(chatID, startText)
				fallback.ParseMode = "HTML"
				fallback.ReplyMarkup = keyboard
				bot.Send(fallback)
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
		bot.Send(msg)
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
		bot.Send(msg)
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
		msg := tgbotapi.NewMessage(chatID, "Pinging...")
		sentMsg, err := bot.Send(msg)
		if err == nil {
			apiDuration := time.Since(apiStart).Milliseconds()
			internalLatency := time.Since(start).Milliseconds()
			bot.Send(tgbotapi.NewEditMessageText(chatID, sentMsg.MessageID, fmt.Sprintf("Pong!\n\n• API Roundtrip: %dms\n• Internal Routing: %dms", apiDuration, internalLatency)))
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
		_, err := bot.Send(reply)
		if err != nil {
			log.Printf("Failed to dispatch /%s: %v", command, err)
		}
	}
}
