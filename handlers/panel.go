package handlers

import (
	"fmt"
	"html"
	"strconv"
	"strings"
	"time"

	"minimate-bot/models"
	"minimate-bot/services"
	"minimate-bot/workers"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// HandlePanelCommand opens the interactive dashboard for group admins
func HandlePanelCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	chatID := message.Chat.ID
	fromID := int64(0)
	if message.From != nil {
		fromID = message.From.ID
	} else if message.SenderChat != nil {
		fromID = message.SenderChat.ID
	}

	if message.Chat.IsPrivate() {
		sendHTMLMessage(bot, chatID, `🛡️ <b>MiniMate Group Manager Dashboard</b>

To access the admin dashboard, send <code>/panel</code> inside your group or supergroup where MiniMate is an administrator.`)
		return
	}

	// Verify caller has administrator or bot-admin rights
	if !services.IsAdminOrRole(bot, chatID, fromID) {
		sendHTMLMessage(bot, chatID, "❌ <b>Access Denied:</b> Only administrators can access the group manager panel.")
		return
	}

	text, markup := renderPanelHome(bot, chatID)
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = markup
	SafeSend(bot, msg)
}

// HandlePanelCallback routes all dashboard inline button interactions
func HandlePanelCallback(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
	if query == nil || query.Message == nil {
		return
	}

	data := query.Data
	callerID := query.From.ID

	// Immediately acknowledge callback query
	bot.Request(tgbotapi.NewCallback(query.ID, ""))

	parts := strings.Split(data, ":")
	action := parts[0]

	// Extract target chatID
	var targetChatID int64
	if len(parts) > 1 {
		cid, err := strconv.ParseInt(parts[1], 10, 64)
		if err == nil {
			targetChatID = cid
		}
	}
	if targetChatID == 0 {
		targetChatID = query.Message.Chat.ID
	}

	// Strict authorization check: verify caller is admin of the target group
	if !services.IsAdminOrRole(bot, targetChatID, callerID) {
		bot.Request(tgbotapi.NewCallbackWithAlert(query.ID, "❌ You do not have permission to manage this group."))
		return
	}

	var newText string
	var markup tgbotapi.InlineKeyboardMarkup

	switch action {
	case "panel_home":
		newText, markup = renderPanelHome(bot, targetChatID)

	case "panel_sec":
		newText, markup = renderPanelSecurity(targetChatID)

	case "panel_mod":
		newText, markup = renderPanelModeration(bot, targetChatID)

	case "panel_members":
		newText, markup = renderPanelMembers(bot, targetChatID)

	case "panel_settings":
		newText, markup = renderPanelSettings(targetChatID)

	case "panel_analytics":
		newText, markup = renderPanelAnalytics(targetChatID)

	case "panel_logs":
		page := 1
		if len(parts) > 2 {
			if p, err := strconv.Atoi(parts[2]); err == nil && p > 0 {
				page = p
			}
		}
		newText, markup = renderPanelLogs(targetChatID, page)

	case "panel_filters":
		newText, markup = renderPanelFilters(targetChatID)

	case "panel_welcome":
		newText, markup = renderPanelWelcome(targetChatID)

	case "panel_verify":
		newText, markup = renderPanelVerification(targetChatID)

	case "panel_premium":
		newText, markup = renderPanelPremium(targetChatID)

	case "panel_auto":
		newText, markup = renderPanelAutomation(targetChatID)

	case "panel_toggle":
		if len(parts) > 2 {
			settingKey := parts[2]
			toggleGroupSetting(targetChatID, settingKey)
		}
		newText, markup = renderPanelSettings(targetChatID)

	case "panel_sec_toggle":
		if len(parts) > 2 {
			secKey := parts[2]
			toggleGroupSetting(targetChatID, secKey)
		}
		newText, markup = renderPanelSecurity(targetChatID)

	case "panel_lockdown_confirm":
		newText = `⚠️ <b>Confirm Emergency Lockdown</b>

<blockquote expandable>Activating lockdown will restrict <b>all regular members</b> from sending messages, stickers, media, and links.

Administrators will remain unaffected.</blockquote>

Are you sure you want to proceed?`
		markup = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🔴 Yes, Lock Group", fmt.Sprintf("panel_lockdown_do:%d:lock", targetChatID)),
				tgbotapi.NewInlineKeyboardButtonData("❌ Cancel", fmt.Sprintf("panel_sec:%d", targetChatID)),
			),
		)

	case "panel_lockdown_do":
		mode := "lock"
		if len(parts) > 2 {
			mode = parts[2]
		}
		if mode == "lock" {
			services.SetLockdown(bot, targetChatID, true, "Emergency Lockdown via Dashboard", callerID)
			bot.Request(tgbotapi.NewCallbackWithAlert(query.ID, "🔒 Emergency Lockdown Activated."))
		} else {
			services.SetLockdown(bot, targetChatID, false, "Unlocked via Dashboard", callerID)
			bot.Request(tgbotapi.NewCallbackWithAlert(query.ID, "🔓 Group Unlocked."))
		}
		newText, markup = renderPanelSecurity(targetChatID)

	case "raid_unlock":
		services.SetLockdown(bot, targetChatID, false, "Raid Lockdown Disabled by Admin", callerID)
		bot.Request(tgbotapi.NewCallbackWithAlert(query.ID, "🔓 Raid Lockdown successfully disabled."))
		newText, markup = renderPanelSecurity(targetChatID)

	case "raid_ack":
		bot.Request(tgbotapi.NewCallbackWithAlert(query.ID, "🛡️ Raid protection acknowledged. Lockdown remains active."))
		return

	default:
		return
	}

	edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, newText)
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &markup
	SafeSend(bot, edit)
}

// ==================== RENDERING HELPERS ====================

func renderPanelHome(bot *tgbotapi.BotAPI, chatID int64) (string, tgbotapi.InlineKeyboardMarkup) {
	settings := services.GetGroupSettings(chatID)
	isVIP := IsVIPChat(chatID)

	// Fetch Chat Info
	chatTitle := "This Group"
	memberCount := 0
	chat, err := bot.GetChat(tgbotapi.ChatInfoConfig{
		ChatConfig: tgbotapi.ChatConfig{ChatID: chatID},
	})
	if err == nil {
		chatTitle = chat.Title
	}
	count, err := bot.GetChatMembersCount(tgbotapi.ChatMemberCountConfig{
		ChatConfig: tgbotapi.ChatConfig{ChatID: chatID},
	})
	if err == nil {
		memberCount = count
	}

	secStatus := "🟢 ACTIVE"
	if !settings.AntiSpamEnabled && !settings.AntiRaidEnabled {
		secStatus = "🟡 PARTIAL"
	}

	vipBadge := "Free Tier"
	if isVIP {
		vipBadge = "💎 VIP Activated"
	}

	text := fmt.Sprintf(`🛡️ <b>𝐆𝐑𝐎𝐔𝐏 𝐌𝐀𝐍𝐀𝐆𝐄𝐑 𝐃𝐀𝐒𝐇𝐁𝐎𝐀𝐑𝐃</b>

<blockquote expandable>🏢 <b>Group:</b> %s
👥 <b>Members:</b> <code>%d</code>
🛡️ <b>Security Status:</b> %s
💎 <b>Subscription:</b> %s</blockquote>

Select a management module below:`,
		html.EscapeString(chatTitle), memberCount, secStatus, vipBadge)

	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("👥 Members", fmt.Sprintf("panel_members:%d", chatID)),
			tgbotapi.NewInlineKeyboardButtonData("🛡️ Security", fmt.Sprintf("panel_sec:%d", chatID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔨 Moderation", fmt.Sprintf("panel_mod:%d", chatID)),
			tgbotapi.NewInlineKeyboardButtonData("⚙️ Settings", fmt.Sprintf("panel_settings:%d", chatID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🤖 Automation", fmt.Sprintf("panel_auto:%d", chatID)),
			tgbotapi.NewInlineKeyboardButtonData("📊 Analytics", fmt.Sprintf("panel_analytics:%d", chatID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📋 Logs", fmt.Sprintf("panel_logs:%d:1", chatID)),
			tgbotapi.NewInlineKeyboardButtonData("📝 Filters", fmt.Sprintf("panel_filters:%d", chatID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("👋 Welcome", fmt.Sprintf("panel_welcome:%d", chatID)),
			tgbotapi.NewInlineKeyboardButtonData("🔐 Verification", fmt.Sprintf("panel_verify:%d", chatID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("💎 Premium VIP", fmt.Sprintf("panel_premium:%d", chatID)),
			tgbotapi.NewInlineKeyboardButtonData("🔄 Refresh", fmt.Sprintf("panel_home:%d", chatID)),
		),
	)

	return text, markup
}

func renderPanelSecurity(chatID int64) (string, tgbotapi.InlineKeyboardMarkup) {
	settings := services.GetGroupSettings(chatID)
	isLocked := services.IsLockdownActive(chatID)

	spamStatus := "🔴 OFF"
	if settings.AntiSpamEnabled {
		spamStatus = "🟢 ON"
	}

	raidStatus := "🔴 OFF"
	if settings.AntiRaidEnabled {
		raidStatus = "🟢 ON (" + settings.RaidSensitivity + ")"
	}

	lockStatus := "🟢 OPEN"
	if isLocked {
		lockStatus = "🔴 LOCKED (EMERGENCY)"
	}

	text := fmt.Sprintf(`🛡️ <b>𝐒𝐄𝐂𝐔𝐑𝐈𝐓𝐘 &amp; 𝐒𝐇𝐈𝐄𝐋𝐃 𝐂𝐄𝐍𝐓𝐄𝐑</b>

<blockquote expandable>• <b>Anti-Spam Engine:</b> %s
• <b>Anti-Raid Protection:</b> %s
• <b>Group Status:</b> %s
• <b>Flood Threshold:</b> <code>%d msgs / %ds</code>
• <b>Raid Sensitivity:</b> <code>%s (%d joins / %ds)</code></blockquote>

Toggle shields or trigger emergency actions below:`,
		spamStatus, raidStatus, lockStatus,
		settings.FloodLimit, settings.FloodWindow,
		settings.RaidSensitivity, settings.RaidJoinLimit, settings.RaidWindow)

	var lockdownBtn tgbotapi.InlineKeyboardButton
	if isLocked {
		lockdownBtn = tgbotapi.NewInlineKeyboardButtonData("🔓 Lift Lockdown", fmt.Sprintf("panel_lockdown_do:%d:unlock", chatID))
	} else {
		lockdownBtn = tgbotapi.NewInlineKeyboardButtonData("🚨 Emergency Lockdown", fmt.Sprintf("panel_lockdown_confirm:%d", chatID))
	}

	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🛡️ Toggle Anti-Spam", fmt.Sprintf("panel_sec_toggle:%d:antispam", chatID)),
			tgbotapi.NewInlineKeyboardButtonData("🚨 Toggle Anti-Raid", fmt.Sprintf("panel_sec_toggle:%d:antiraid", chatID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			lockdownBtn,
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⬅️ Back", fmt.Sprintf("panel_home:%d", chatID)),
			tgbotapi.NewInlineKeyboardButtonData("🏠 Home", fmt.Sprintf("panel_home:%d", chatID)),
			tgbotapi.NewInlineKeyboardButtonData("🔄 Refresh", fmt.Sprintf("panel_sec:%d", chatID)),
		),
	)

	return text, markup
}

func renderPanelModeration(bot *tgbotapi.BotAPI, chatID int64) (string, tgbotapi.InlineKeyboardMarkup) {
	settings := services.GetGroupSettings(chatID)

	// Fetch recent 5 logs
	logs, total, _ := services.GetModLogs(chatID, 0, 1, 5)

	var logLines strings.Builder
	if len(logs) == 0 {
		logLines.WriteString("<i>No recent moderation actions recorded.</i>\n")
	} else {
		for _, l := range logs {
			timeAgo := time.Since(l.CreatedAt).Round(time.Minute)
			logLines.WriteString(fmt.Sprintf("• <code>%s</code> on <code>%d</code> (%s ago)\n",
				html.EscapeString(l.Action), l.UserID, timeAgo.String()))
		}
	}

	text := fmt.Sprintf(`🔨 <b>𝐌𝐎𝐃𝐄𝐑𝐀𝐓𝐈𝐎𝐍 &amp; 𝐏𝐔𝐍𝐈𝐒𝐇𝐌𝐄𝐍𝐓 𝐂𝐄𝐍𝐓𝐄𝐑</b>

<blockquote expandable>⚠️ <b>Warning Thresholds:</b>
• 3 Strikes: <code>%s</code>
• 5 Strikes: <code>%s</code>
• 7 Strikes: <code>%s</code>
• 10 Strikes: <code>%s</code>
• Total Mod Actions: <code>%d</code></blockquote>

📋 <b>Recent Activity:</b>
%s`,
		settings.WarnThreshold3, settings.WarnThreshold5,
		settings.WarnThreshold7, settings.WarnThreshold10,
		total, logLines.String())

	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📋 View Full Mod Logs", fmt.Sprintf("panel_logs:%d:1", chatID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⬅️ Back", fmt.Sprintf("panel_home:%d", chatID)),
			tgbotapi.NewInlineKeyboardButtonData("🏠 Home", fmt.Sprintf("panel_home:%d", chatID)),
			tgbotapi.NewInlineKeyboardButtonData("🔄 Refresh", fmt.Sprintf("panel_mod:%d", chatID)),
		),
	)

	return text, markup
}

func renderPanelMembers(bot *tgbotapi.BotAPI, chatID int64) (string, tgbotapi.InlineKeyboardMarkup) {
	memberCount := 0
	count, err := bot.GetChatMembersCount(tgbotapi.ChatMemberCountConfig{
		ChatConfig: tgbotapi.ChatConfig{ChatID: chatID},
	})
	if err == nil {
		memberCount = count
	}

	text := fmt.Sprintf(`👥 <b>𝐌𝐄𝐌𝐁𝐄𝐑 𝐌𝐀𝐍𝐀𝐆𝐄𝐌𝐄𝐍𝐓</b>

<blockquote expandable>📊 <b>Community Size:</b> <code>%d members</code>

<b>Quick Actions:</b>
• <code>/info</code> — Inspect member account info & permissions
• <code>/warns</code> — Check member warning strikes
• <code>/mute @user &lt;time&gt;</code> — Temp-mute member
• <code>/ban @user &lt;time&gt;</code> — Temp-ban member
• <code>/promote @user [title]</code> — Promote to Jr/Sr Admin</blockquote>`,
		memberCount)

	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("👮 Admin Directory", fmt.Sprintf("tab_admin")),
			tgbotapi.NewInlineKeyboardButtonData("📋 Moderation Logs", fmt.Sprintf("panel_logs:%d:1", chatID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⬅️ Back", fmt.Sprintf("panel_home:%d", chatID)),
			tgbotapi.NewInlineKeyboardButtonData("🏠 Home", fmt.Sprintf("panel_home:%d", chatID)),
			tgbotapi.NewInlineKeyboardButtonData("🔄 Refresh", fmt.Sprintf("panel_members:%d", chatID)),
		),
	)

	return text, markup
}

func renderPanelSettings(chatID int64) (string, tgbotapi.InlineKeyboardMarkup) {
	settings := services.GetGroupSettings(chatID)

	badge := func(b bool) string {
		if b {
			return "🟢 ON"
		}
		return "🔴 OFF"
	}

	text := fmt.Sprintf(`⚙️ <b>𝐆𝐑𝐎𝐔𝐏 𝐂𝐎𝐍𝐅𝐈𝐆𝐔𝐑𝐀𝐓𝐈𝐎𝐍 𝐒𝐄𝐓𝐓𝐈𝐍𝐆𝐒</b>

<blockquote expandable>• Anti-Spam Engine: %s
• Anti-Raid Defense: %s
• Delete Join Messages: %s
• Delete Leave Messages: %s
• Delete Command Messages: %s</blockquote>

Tap a setting to toggle its state:`,
		badge(settings.AntiSpamEnabled), badge(settings.AntiRaidEnabled),
		badge(settings.DeleteJoinMessages), badge(settings.DeleteLeaveMessages),
		badge(settings.DeleteCmdMessages))

	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🛡️ Anti-Spam", fmt.Sprintf("panel_toggle:%d:antispam", chatID)),
			tgbotapi.NewInlineKeyboardButtonData("🚨 Anti-Raid", fmt.Sprintf("panel_toggle:%d:antiraid", chatID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🧹 Del Joins", fmt.Sprintf("panel_toggle:%d:del_joins", chatID)),
			tgbotapi.NewInlineKeyboardButtonData("🧹 Del Leaves", fmt.Sprintf("panel_toggle:%d:del_leaves", chatID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🧹 Del Commands", fmt.Sprintf("panel_toggle:%d:del_cmds", chatID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⬅️ Back", fmt.Sprintf("panel_home:%d", chatID)),
			tgbotapi.NewInlineKeyboardButtonData("🏠 Home", fmt.Sprintf("panel_home:%d", chatID)),
			tgbotapi.NewInlineKeyboardButtonData("🔄 Refresh", fmt.Sprintf("panel_settings:%d", chatID)),
		),
	)

	return text, markup
}

func renderPanelAnalytics(chatID int64) (string, tgbotapi.InlineKeyboardMarkup) {
	stats7d, _ := workers.GetAnalytics(chatID, 7)
	stats30d, _ := workers.GetAnalytics(chatID, 30)

	getVal := func(m map[string]int64, key string) int64 {
		if m == nil {
			return 0
		}
		return m[key]
	}

	text := fmt.Sprintf(`📊 <b>𝐆𝐑𝐎𝐔𝐏 𝐀𝐍𝐀𝐋𝐘𝐓𝐈𝐂𝐒 &amp; 𝐈𝐍𝐒𝐈𝐆𝐇𝐓𝐒</b>

<blockquote expandable>📈 <b>Last 7 Days:</b>
• 💬 <b>Messages:</b> <code>%d</code>
• 👥 <b>New Joins:</b> <code>%d</code>
• 👋 <b>Leaves:</b> <code>%d</code>
• 🛡️ <b>Spam Blocked:</b> <code>%d</code>
• 🔨 <b>Moderations:</b> <code>%d</code>

📊 <b>Last 30 Days:</b>
• 💬 <b>Messages:</b> <code>%d</code>
• 👥 <b>New Joins:</b> <code>%d</code>
• 🛡️ <b>Spam Blocked:</b> <code>%d</code></blockquote>`,
		getVal(stats7d, workers.MetricMessage),
		getVal(stats7d, workers.MetricJoin),
		getVal(stats7d, workers.MetricLeave),
		getVal(stats7d, workers.MetricSpamBlocked),
		getVal(stats7d, workers.MetricBan)+getVal(stats7d, workers.MetricMute)+getVal(stats7d, workers.MetricWarn),
		getVal(stats30d, workers.MetricMessage),
		getVal(stats30d, workers.MetricJoin),
		getVal(stats30d, workers.MetricSpamBlocked))

	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⬅️ Back", fmt.Sprintf("panel_home:%d", chatID)),
			tgbotapi.NewInlineKeyboardButtonData("🏠 Home", fmt.Sprintf("panel_home:%d", chatID)),
			tgbotapi.NewInlineKeyboardButtonData("🔄 Refresh", fmt.Sprintf("panel_analytics:%d", chatID)),
		),
	)

	return text, markup
}

func renderPanelLogs(chatID int64, page int) (string, tgbotapi.InlineKeyboardMarkup) {
	pageSize := 6
	logs, total, err := services.GetModLogs(chatID, 0, page, pageSize)
	if err != nil {
		logs = []models.ModLog{}
		total = 0
	}

	totalPages := (total + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("📋 <b>𝐌𝐎𝐃𝐄𝐑𝐀𝐓𝐈𝐎𝐍 𝐀𝐔𝐃𝐈𝐓 𝐋𝐎𝐆𝐒</b> <i>(Page %d/%d)</i>\n\n", page, totalPages))

	if len(logs) == 0 {
		b.WriteString("<blockquote expandable><i>No moderation records found in history.</i></blockquote>")
	} else {
		b.WriteString("<blockquote expandable>")
		for _, l := range logs {
			tStr := l.CreatedAt.Format("02 Jan 15:04")
			b.WriteString(fmt.Sprintf("• <b>%s</b> | User: <code>%d</code> | Mod: <code>%d</code>\n  <i>%s</i> (%s)\n",
				strings.ToUpper(l.Action), l.UserID, l.ModID, html.EscapeString(l.Reason), tStr))
		}
		b.WriteString("</blockquote>")
	}

	var navRow []tgbotapi.InlineKeyboardButton
	if page > 1 {
		navRow = append(navRow, tgbotapi.NewInlineKeyboardButtonData("⬅️ Prev", fmt.Sprintf("panel_logs:%d:%d", chatID, page-1)))
	}
	if page < totalPages {
		navRow = append(navRow, tgbotapi.NewInlineKeyboardButtonData("Next ➡️", fmt.Sprintf("panel_logs:%d:%d", chatID, page+1)))
	}

	rows := [][]tgbotapi.InlineKeyboardButton{}
	if len(navRow) > 0 {
		rows = append(rows, navRow)
	}
	rows = append(rows, []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("⬅️ Back", fmt.Sprintf("panel_mod:%d", chatID)),
		tgbotapi.NewInlineKeyboardButtonData("🏠 Home", fmt.Sprintf("panel_home:%d", chatID)),
		tgbotapi.NewInlineKeyboardButtonData("🔄 Refresh", fmt.Sprintf("panel_logs:%d:%d", chatID, page)),
	})

	return b.String(), tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func renderPanelFilters(chatID int64) (string, tgbotapi.InlineKeyboardMarkup) {
	filters := GetChatFilterKeywords(chatID)

	var b strings.Builder
	b.WriteString(fmt.Sprintf("📝 <b>𝐀𝐔𝐓𝐎-𝐑𝐄𝐏𝐋𝐘 𝐅𝐈𝐋𝐓𝐄𝐑𝐒 &amp; 𝐍𝐎𝐓𝐄𝐒</b> (<code>%d active</code>)\n\n<blockquote expandable>", len(filters)))

	if len(filters) == 0 {
		b.WriteString("<i>No auto-reply filters saved for this chat.</i>\n")
	} else {
		for i, keyword := range filters {
			b.WriteString(fmt.Sprintf("%d. <code>%s</code>\n", i+1, html.EscapeString(keyword)))
			if i >= 14 {
				b.WriteString(fmt.Sprintf("<i>...and %d more</i>\n", len(filters)-15))
				break
			}
		}
	}
	b.WriteString("</blockquote>\n💡 <i>Add new: <code>/filter &lt;keyword&gt; &lt;reply&gt;</code></i>")

	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⬅️ Back", fmt.Sprintf("panel_home:%d", chatID)),
			tgbotapi.NewInlineKeyboardButtonData("🏠 Home", fmt.Sprintf("panel_home:%d", chatID)),
			tgbotapi.NewInlineKeyboardButtonData("🔄 Refresh", fmt.Sprintf("panel_filters:%d", chatID)),
		),
	)

	return b.String(), markup
}

func renderPanelWelcome(chatID int64) (string, tgbotapi.InlineKeyboardMarkup) {
	g := GetGreetingConfig(chatID)

	wStatus := "🔴 Disabled"
	if g.WelcomeEnabled {
		wStatus = "🟢 Enabled"
	}
	gbStatus := "🔴 Disabled"
	if g.GoodbyeEnabled {
		gbStatus = "🟢 Enabled"
	}

	text := fmt.Sprintf(`👋 <b>𝐖𝐄𝐋𝐂𝐎𝐌𝐄 &amp; 𝐆𝐎𝐎𝐃𝐁𝐘𝐄 𝐒𝐔𝐈𝐓𝐄</b>

<blockquote expandable>• <b>Welcome Message:</b> %s
• <b>Goodbye Message:</b> %s

<b>Supported Template Variables:</b>
<code>{user}</code> — Clickable user mention
<code>{username}</code> — @username
<code>{group}</code> — Group Title
<code>{member_count}</code> — Total Members
<code>{id}</code> — User ID</blockquote>

💡 <i>Set message: <code>/setwelcome &lt;text&gt;</code></i>`,
		wStatus, gbStatus)

	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⬅️ Back", fmt.Sprintf("panel_home:%d", chatID)),
			tgbotapi.NewInlineKeyboardButtonData("🏠 Home", fmt.Sprintf("panel_home:%d", chatID)),
			tgbotapi.NewInlineKeyboardButtonData("🔄 Refresh", fmt.Sprintf("panel_welcome:%d", chatID)),
		),
	)

	return text, markup
}

func renderPanelVerification(chatID int64) (string, tgbotapi.InlineKeyboardMarkup) {
	c := getCaptchaSettings(chatID)

	cStatus := "🔴 Disabled"
	if c.Enabled {
		cStatus = fmt.Sprintf("🟢 Enabled (%s, %ds)", c.Mode, c.TimeoutSeconds)
	}

	text := fmt.Sprintf(`🔐 <b>𝐇𝐔𝐌𝐀𝐍 𝐕𝐄𝐑𝐈𝐅𝐈𝐂𝐀𝐓𝐈𝐎𝐍 &amp; 𝐂𝐀𝐏𝐓𝐂𝐇𝐀</b>

<blockquote expandable>• <b>Verification Status:</b> %s
• <b>Challenge Mode:</b> <code>%s</code>
• <b>Verification Timeout:</b> <code>%ds</code>

Unverified users are restricted until passing the human verification challenge.</blockquote>

💡 <i>Toggle: <code>/captcha &lt;on/off&gt;</code></i>`,
		cStatus, c.Mode, c.TimeoutSeconds)

	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⬅️ Back", fmt.Sprintf("panel_home:%d", chatID)),
			tgbotapi.NewInlineKeyboardButtonData("🏠 Home", fmt.Sprintf("panel_home:%d", chatID)),
			tgbotapi.NewInlineKeyboardButtonData("🔄 Refresh", fmt.Sprintf("panel_verify:%d", chatID)),
		),
	)

	return text, markup
}

func renderPanelPremium(chatID int64) (string, tgbotapi.InlineKeyboardMarkup) {
	isVIP := IsVIPChat(chatID)

	statusText := "⚪ <b>Free Community Tier</b>"
	if isVIP {
		statusText = "💎 <b>VIP Active (Ultra-Priority Engine)</b>"
	}

	text := fmt.Sprintf(`💎 <b>𝐌𝐈𝐍𝐈𝐌𝐀𝐓𝐄 𝐕𝐈𝐏 &amp; 𝐏𝐑𝐄𝐌𝐈𝐔𝐌</b>

<blockquote expandable>• <b>Current Plan:</b> %s
• <b>Priority Polling:</b> 0ms Routing
• <b>Custom Animated Emojis:</b> Unlocked
• <b>Storage Limits:</b> Unlimited Filters &amp; Notes
• <b>Extended Logs:</b> 365 Days Retention</blockquote>

👉 <i>Contact @TheDarkKratos to activate group VIP.</i>`,
		statusText)

	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⬅️ Back", fmt.Sprintf("panel_home:%d", chatID)),
			tgbotapi.NewInlineKeyboardButtonData("🏠 Home", fmt.Sprintf("panel_home:%d", chatID)),
			tgbotapi.NewInlineKeyboardButtonData("🔄 Refresh", fmt.Sprintf("panel_premium:%d", chatID)),
		),
	)

	return text, markup
}

func renderPanelAutomation(chatID int64) (string, tgbotapi.InlineKeyboardMarkup) {
	text := `🤖 <b>𝐀𝐔𝐓𝐎𝐌𝐀𝐓𝐈𝐎𝐍 &amp; 𝐒𝐂𝐇𝐄𝐃𝐔𝐋𝐄𝐃 𝐄𝐍𝐆𝐈𝐍𝐄</b>

<blockquote expandable><b>Supported Rule Formats:</b>
<code>WHEN</code> → <code>CONDITION</code> → <code>ACTION</code>

• Auto-Lockdown on Raid
• Auto-Mute on Warning Thresholds
• Auto-Cleanup Join &amp; Leave Messages
• Persistent Background Task Workers</blockquote>

💡 <i>Manage via <code>/automation</code> and <code>/schedule</code></i>`

	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⬅️ Back", fmt.Sprintf("panel_home:%d", chatID)),
			tgbotapi.NewInlineKeyboardButtonData("🏠 Home", fmt.Sprintf("panel_home:%d", chatID)),
			tgbotapi.NewInlineKeyboardButtonData("🔄 Refresh", fmt.Sprintf("panel_auto:%d", chatID)),
		),
	)

	return text, markup
}

// toggleGroupSetting flips boolean settings in group_settings
func toggleGroupSetting(chatID int64, key string) {
	settings := services.GetGroupSettings(chatID)
	switch key {
	case "antispam":
		settings.AntiSpamEnabled = !settings.AntiSpamEnabled
	case "antiraid":
		settings.AntiRaidEnabled = !settings.AntiRaidEnabled
	case "del_joins":
		settings.DeleteJoinMessages = !settings.DeleteJoinMessages
	case "del_leaves":
		settings.DeleteLeaveMessages = !settings.DeleteLeaveMessages
	case "del_cmds":
		settings.DeleteCmdMessages = !settings.DeleteCmdMessages
	}
	services.SaveGroupSettings(chatID, settings)
}

// HandleUndoCallback handles reverse moderation action from [↩ Undo] button
func HandleUndoCallback(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
	if query == nil {
		return
	}
	callerID := query.From.ID
	data := query.Data

	parts := strings.Split(data, ":")
	if len(parts) < 4 {
		return
	}

	chatID, _ := strconv.ParseInt(parts[1], 10, 64)
	logID, _ := strconv.ParseInt(parts[3], 10, 64)

	if !services.IsAdminOrRole(bot, chatID, callerID) {
		bot.Request(tgbotapi.NewCallbackWithAlert(query.ID, "❌ Only administrators can undo moderation actions."))
		return
	}

	res := services.UndoAction(bot, logID, callerID)
	if res.Success {
		bot.Request(tgbotapi.NewCallbackWithAlert(query.ID, "✅ Action successfully undone."))
		if query.Message != nil {
			editText := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID,
				query.Message.Text+"\n\n<i>[↩️ Action was undone by administrator]</i>")
			editText.ParseMode = "HTML"
			SafeSend(bot, editText)
		}
	} else {
		bot.Request(tgbotapi.NewCallbackWithAlert(query.ID, res.Message))
	}
}

// HandleLogsCommand displays paginated moderation history for the chat or a specific user
func HandleLogsCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message, args string) {
	chatID := message.Chat.ID
	fromID := int64(0)
	if message.From != nil {
		fromID = message.From.ID
	}
	if !services.IsAdminOrRole(bot, chatID, fromID) {
		sendHTMLMessage(bot, chatID, "❌ Only administrators can view moderation logs.")
		return
	}

	text, markup := renderPanelLogs(chatID, 1)
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = markup
	SafeSend(bot, msg)
}

// HandleLockdownCommand activates or lifts emergency group lockdown
func HandleLockdownCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message, cmd string, args string) {
	chatID := message.Chat.ID
	fromID := int64(0)
	if message.From != nil {
		fromID = message.From.ID
	}
	if !services.IsAdminOrRole(bot, chatID, fromID) {
		sendHTMLMessage(bot, chatID, "❌ Only administrators can toggle group lockdown.")
		return
	}

	cmdLower := strings.ToLower(cmd)
	if cmdLower == "unlockdown" || strings.ToLower(args) == "off" || strings.ToLower(args) == "unlock" {
		services.SetLockdown(bot, chatID, false, "Unlocked via command", fromID)
		sendHTMLMessage(bot, chatID, "🔓 <b>Group Lockdown Disabled:</b> Regular members can now chat normally.")
		return
	}

	// Show confirmation for emergency lockdown
	newText := `⚠️ <b>Confirm Emergency Lockdown</b>

<blockquote expandable>Activating lockdown will restrict <b>all regular members</b> from sending messages, stickers, media, and links.

Administrators will remain unaffected.</blockquote>

Are you sure you want to proceed?`
	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔴 Yes, Lock Group", fmt.Sprintf("panel_lockdown_do:%d:lock", chatID)),
			tgbotapi.NewInlineKeyboardButtonData("❌ Cancel", fmt.Sprintf("panel_sec:%d", chatID)),
		),
	)

	msg := tgbotapi.NewMessage(chatID, newText)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = markup
	SafeSend(bot, msg)
}
