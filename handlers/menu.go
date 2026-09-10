package handlers

import (
	"fmt"
	"html"
	"log"
	"time"

	"minimate-bot/config"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// HandleMenuCallback processes all inline menu and command directory tab button clicks
func HandleMenuCallback(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
	if query == nil || query.Message == nil {
		return
	}

	// 1. Immediately acknowledge the button click to remove the Telegram loading spinner
	bot.Request(tgbotapi.NewCallback(query.ID, ""))

	botUsername := bot.Self.UserName
	firstName := query.From.FirstName
	data := query.Data

	var newText string
	var markup tgbotapi.InlineKeyboardMarkup

	switch data {
	case "start_home":
		newText = GetHomeText(firstName)
		markup = GetStartKeyboard(botUsername)

	case "menu_commands", "help_menu":
		newText = `📚 <b>𝐌𝐢𝐧𝐢𝐌𝐚𝐭𝐞 𝐂𝐨𝐦𝐦𝐚𝐧𝐝 𝐄𝐱𝐩𝐥𝐨𝐫𝐞𝐫</b>

<blockquote expandable>Select a category below to explore all available features, moderation commands, and automation tools.</blockquote>`
		markup = getCommandsCategoryKeyboard()

	case "tab_admin":
		newText = `👮 <b>𝐀𝐝𝐦𝐢𝐧 &amp; 𝐌𝐨𝐝𝐞𝐫𝐚𝐭𝐢𝐨𝐧 𝐂𝐨𝐦𝐦𝐚𝐧𝐝𝐬</b>

<blockquote expandable><i>(Reply to a user to execute)</i>
• <code>/ban</code> — Ban a member permanently
• <code>/unban</code> — Unban a user
• <code>/tban &lt;time&gt;</code> — Temp-ban (e.g. <code>/tban 2h</code>)
• <code>/kick</code> — Kick user from group
• <code>/mute</code> — Mute user permanently
• <code>/unmute</code> — Unmute user
• <code>/tmute &lt;time&gt;</code> — Temp-mute (e.g. <code>/tmute 30m</code>)
• <code>/warn</code> — Issue a warning strike (3 strikes = auto-ban)
• <code>/dwarn</code> — Delete replied message + issue warn strike
• <code>/unwarn</code> — Remove 1 warning strike from user
• <code>/rmwarns</code> — Reset all warnings for a user
• <code>/warns</code> — Check your warning strike count
• <code>/promote &lt;title&gt;</code> — Promote user to Admin
• <code>/demote</code> — Demote admin to regular member</blockquote>`
		markup = getCategoryBackKeyboard()

	case "tab_locks":
		newText = `🛡️ <b>𝐒𝐞𝐜𝐮𝐫𝐢𝐭𝐲 &amp; 𝐋𝐨𝐜𝐤 𝐒𝐡𝐢𝐞𝐥𝐝𝐬 (48+ Types)</b>

<blockquote expandable>• <code>/lock &lt;type&gt;</code> — Lock any media, link, script or message type
• <code>/unlock &lt;type&gt;</code> — Unlock specified lock type
• <code>/locks</code> — View visual dashboard of all 48 shields
• <code>/locktypes</code> — Open full directory of 48+ lock categories
• <code>/captcha &lt;on/off&gt;</code> — Toggle new member captcha
• <code>/captchamode &lt;button|math&gt;</code> — Set captcha challenge
• <code>/captchatime &lt;sec&gt;</code> — Verification timeout (30-600s)

🌟 <b>Popular Lock Types:</b>
<code>all</code>, <code>media</code>, <code>url</code>, <code>invitelink</code>, <code>botlink</code>, <code>forward</code>, <code>sticker</code>, <code>gif</code>, <code>cjk</code>, <code>cyrillic</code>, <code>rtl</code>, <code>zalgo</code>, <code>album</code>, <code>poll</code>, <code>contact</code>, <code>anonchannel</code>, <code>bot</code></blockquote>`
		markup = getCategoryBackKeyboard()

	case "tab_tools":
		newText = `🧹 <b>𝐂𝐡𝐚𝐭 𝐓𝐨𝐨𝐥𝐬 &amp; 𝐔𝐭𝐢𝐥𝐢𝐭𝐢𝐞𝐬</b>

<blockquote expandable>• <code>/purge</code> — Mass delete replied to current message
• <code>/del</code> — Delete replied message immediately
• <code>/pin</code> — Pin replied message quietly
• <code>/pin loud</code> — Pin message with notification
• <code>/unpin</code> — Unpin replied message
• <code>/unpinall</code> — Unpin all pinned messages in chat
• <code>/ping</code> — Check bot latency & response speed
• <code>/id</code> — Get user ID and chat ID
• <code>/info</code> — View detailed account stats</blockquote>`
		markup = getCategoryBackKeyboard()

	case "tab_extra":
		newText = `📝 <b>𝐅𝐢𝐥𝐭𝐞𝐫𝐬, 𝐍𝐨𝐭𝐞𝐬 &amp; 𝐆𝐫𝐞𝐞𝐭𝐢𝐧𝐠𝐬</b>

<blockquote expandable>• <code>/filter &lt;keyword&gt; &lt;reply&gt;</code> — Add auto-reply
• <code>/stop &lt;keyword&gt;</code> — Delete an auto-reply filter
• <code>/filters</code> — List all active group filters
• <code>/save &lt;name&gt; &lt;text&gt;</code> — Save a group note
• <code>/get &lt;name&gt;</code> — Retrieve a saved note
• <code>/clear &lt;name&gt;</code> — Delete a saved note
• <code>/notes</code> — List all saved group notes
• <code>/rules</code> — Display group rules (or <code>/privaterules</code>)
• <code>/setrules &lt;text&gt;</code> — Update group rules
• <code>/clearrules</code> — Clear current group rules
• <code>/welcome &lt;on/off&gt;</code> — Toggle welcome greeting
• <code>/setwelcome &lt;text&gt;</code> — Custom welcome message
• <code>/goodbye &lt;on/off&gt;</code> — Toggle goodbye message
• <code>/setgoodbye &lt;text&gt;</code> — Custom goodbye message</blockquote>`
		markup = getCategoryBackKeyboard()

	case "tab_premium":
		newText = `💎 <b>𝐌𝐢𝐧𝐢𝐌𝐚𝐭𝐞 𝐕𝐈𝐏 &amp; 𝐏𝐫𝐞𝐦𝐢𝐮𝐦 𝐒𝐡𝐢𝐞𝐥𝐝</b>

<blockquote expandable>✨ <b>Exclusive VIP Features:</b>
• ⚡ <b>Ultra-High Priority Polling:</b> 0ms response latency
• 🛡️ <b>Advanced AI Anti-Raid:</b> Auto-ban spam bot waves
• 🎨 <b>Custom Emoji Packs:</b> PokeEmpire animated theme
• ♾️ <b>Unlimited Storage:</b> Unlimited filters & notes
• 👑 <b>Dedicated VIP Support:</b> 24/7 direct developer assistance</blockquote>

👉 Contact @` + html.EscapeString(config.OwnerUsername) + ` for group VIP activation!`
		markup = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonURL("👑 Contact Owner", fmt.Sprintf("https://t.me/%s", config.OwnerUsername)),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("⬅️ Back to Categories", "menu_commands"),
				tgbotapi.NewInlineKeyboardButtonData("🏠 Home", "start_home"),
			),
		)

	case "tab_about":
		uptime := time.Since(botStartTime).Round(time.Second)
		newText = fmt.Sprintf(`ℹ️ <b>𝐀𝐛𝐨𝐮𝐭 𝐌𝐢𝐧𝐢𝐌𝐚𝐭𝐞 𝐏𝐫𝐨</b>

<blockquote expandable>🤖 <b>Bot:</b> @%s
👑 <b>Developer:</b> @%s
⚡ <b>Engine:</b> Go 1.24 + PostgreSQL (Supabase)
⏱️ <b>Uptime:</b> %s
🛡️ <b>Framework:</b> Next-Gen Ultra-Fast Long Polling</blockquote>`,
			botUsername, html.EscapeString(config.OwnerUsername), uptime.String())
		markup = getCategoryBackKeyboard()

	default:
		return
	}

	// 2. Perform smooth in-place update (caption if media, text if regular message)
	EditMenu(bot, query, newText, &markup)
}

// EditMenu handles editing text messages or media captions smoothly
func EditMenu(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery, text string, markup *tgbotapi.InlineKeyboardMarkup) {
	msg := query.Message
	if msg == nil {
		return
	}

	chatID := msg.Chat.ID
	messageID := msg.MessageID

	// If the original message is a Video/Animation/Photo, edit its Caption
	if msg.Video != nil || msg.Animation != nil || len(msg.Photo) > 0 {
		editCaption := tgbotapi.NewEditMessageCaption(chatID, messageID, text)
		editCaption.ParseMode = "HTML"
		editCaption.ReplyMarkup = markup
		_, err := SafeSend(bot, editCaption)
		if err != nil {
			log.Printf("EditMessageCaption notice: %v", err)
		}
		return
	}

	// Otherwise, edit the Message Text
	editText := tgbotapi.NewEditMessageText(chatID, messageID, text)
	editText.ParseMode = "HTML"
	editText.ReplyMarkup = markup
	_, err := SafeSend(bot, editText)
	if err != nil {
		log.Printf("EditMessageText notice: %v", err)
	}
}

// getCommandsCategoryKeyboard generates the categorized module grid
func getCommandsCategoryKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("👮 Admin & Moderation", "tab_admin"),
			tgbotapi.NewInlineKeyboardButtonData("🛡️ Security & Locks", "tab_locks"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🧹 Chat Tools & Pin", "tab_tools"),
			tgbotapi.NewInlineKeyboardButtonData("📝 Filters & Notes", "tab_extra"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💎 Premium VIP", "tab_premium"),
			tgbotapi.NewInlineKeyboardButtonData("ℹ️ About Bot", "tab_about"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⬅️ Back to Home", "start_home"),
		),
	)
}

// getCategoryBackKeyboard generates navigation buttons for category sub-tabs
func getCategoryBackKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⬅️ Back to Categories", "menu_commands"),
			tgbotapi.NewInlineKeyboardButtonData("🏠 Home", "start_home"),
		),
	)
}