package handlers

import (
	"context"
	"fmt"
	"html"
	"log"
	"runtime"
	"strconv"
	"strings"
	"time"

	"minimate-bot/config"
	"minimate-bot/database"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// isAdmin checks if a given user has administrator privileges in the chat
func isAdmin(bot *tgbotapi.BotAPI, chatID int64, userID int64) bool {
	if userID == 1087968824 {
		return true
	}
	if userID == 0 {
		return false
	}
	// Bot Owner is always treated as administrator
	if config.OwnerID != 0 && userID == config.OwnerID {
		return true
	}

	config := tgbotapi.GetChatMemberConfig{
		ChatConfigWithUser: tgbotapi.ChatConfigWithUser{
			ChatID: chatID,
			UserID: userID,
		},
	}
	member, err := bot.GetChatMember(config)
	if err != nil {
		return false
	}
	return member.Status == "administrator" || member.Status == "creator"
}

// IsVIPChat checks if a given group has an active VIP / Premium subscription
func IsVIPChat(chatID int64) bool {
	var isVIP bool
	var expiresAt *time.Time
	query := "SELECT is_vip, expires_at FROM chat_subscriptions WHERE chat_id = $1"
	err := database.Pool.QueryRow(context.Background(), query, chatID).Scan(&isVIP, &expiresAt)
	if err != nil || !isVIP {
		return false
	}
	if expiresAt != nil && time.Now().After(*expiresAt) {
		return false
	}
	return true
}

// RecordChatActivity registers or updates active groups where MiniMate is active
func RecordChatActivity(chat *tgbotapi.Chat) {
	if chat == nil || chat.Type == "private" {
		return
	}

	query := `
		INSERT INTO bot_chats (chat_id, title, chat_type, is_active, updated_at)
		VALUES ($1, $2, $3, true, NOW())
		ON CONFLICT (chat_id) DO UPDATE SET
			title = EXCLUDED.title,
			chat_type = EXCLUDED.chat_type,
			is_active = true,
			updated_at = NOW();
	`
	database.Pool.Exec(context.Background(), query, chat.ID, chat.Title, chat.Type)
}

// parseDuration parses strings like "10m", "2h", "1d" into a unix timestamp
func parseDuration(durationStr string) (int64, error) {
	if len(durationStr) < 2 {
		return 0, fmt.Errorf("invalid duration format")
	}

	unit := strings.ToLower(durationStr[len(durationStr)-1:])
	valStr := durationStr[:len(durationStr)-1]

	val, err := strconv.ParseInt(valStr, 10, 64)
	if err != nil || val <= 0 {
		return 0, fmt.Errorf("invalid duration value")
	}

	var d time.Duration
	switch unit {
	case "m":
		d = time.Duration(val) * time.Minute
	case "h":
		d = time.Duration(val) * time.Hour
	case "d":
		d = time.Duration(val) * 24 * time.Hour
	default:
		return 0, fmt.Errorf("unknown time unit: %s (use m, h, or d)", unit)
	}

	if d < 30*time.Second {
		return 0, fmt.Errorf("duration must be at least 30 seconds")
	}
	if d > 366*24*time.Hour {
		return 0, fmt.Errorf("duration cannot exceed 366 days")
	}

	return time.Now().Add(d).Unix(), nil
}

func sendHTMLMessage(bot *tgbotapi.BotAPI, chatID int64, text string) (tgbotapi.Message, error) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	return bot.Send(msg)
}

// -------------------------
// OWNER DASHBOARD & SPAM
// -------------------------

// HandleOwnerDashboard renders live performance metrics and controls
func HandleOwnerDashboard(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	fromID := int64(0)
	if message.From != nil {
		fromID = message.From.ID
	}
	if config.OwnerID != 0 && fromID != config.OwnerID {
		sendHTMLMessage(bot, message.Chat.ID, "❌ This command is strictly restricted to the <b>Bot Owner</b>.")
		return
	}

	ctx := context.Background()
	var totalChats, totalVIP, totalFilters, totalWarns int

	database.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM bot_chats WHERE is_active = true").Scan(&totalChats)
	database.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM chat_subscriptions WHERE is_vip = true").Scan(&totalVIP)
	database.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM filters").Scan(&totalFilters)
	database.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM user_warns").Scan(&totalWarns)

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	ramAllocMB := float64(m.Alloc) / 1024 / 1024
	uptime := time.Since(botStartTime).Round(time.Second)

	text := fmt.Sprintf(`👑 <b>𝐌𝐢𝐧𝐢𝐌𝐚𝐭𝐞 𝐎𝐰𝐧𝐞𝐫 𝐂𝐨𝐧𝐭𝐫𝐨𝐥 𝐃𝐚𝐬𝐡𝐛𝐨𝐚𝐫𝐝</b>

<blockquote expandable>📊 <b>Live System Statistics:</b>
• <b>Active Groups / Channels:</b> <code>%d</code>
• <b>VIP Subscribed Chats:</b> <code>%d</code>
• <b>Saved Custom Filters:</b> <code>%d</code>
• <b>Active Warning Strikes:</b> <code>%d</code>

⚙️ <b>Engine Performance:</b>
• <b>RAM Usage:</b> <code>%.2f MB</code>
• <b>Goroutines:</b> <code>%d</code>
• <b>Bot Uptime:</b> <code>%s</code>
• <b>Go Version:</b> <code>%s</code></blockquote>

💡 <i>Use <code>/spam</code> to view all active groups or <code>/setvip</code> to manage subscriptions.</i>`,
		totalChats, totalVIP, totalFilters, totalWarns,
		ramAllocMB, runtime.NumGoroutine(), uptime.String(), runtime.Version())

	sendHTMLMessage(bot, message.Chat.ID, text)
}

// HandleSpamCommand lists all registered groups where the bot is active
func HandleSpamCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	fromID := int64(0)
	if message.From != nil {
		fromID = message.From.ID
	}
	if config.OwnerID != 0 && fromID != config.OwnerID {
		sendHTMLMessage(bot, message.Chat.ID, "❌ This command is strictly restricted to the <b>Bot Owner</b>.")
		return
	}

	ctx := context.Background()
	rows, err := database.Pool.Query(ctx, "SELECT chat_id, title, chat_type, is_active FROM bot_chats ORDER BY updated_at DESC LIMIT 50")
	if err != nil {
		sendHTMLMessage(bot, message.Chat.ID, "❌ Database error fetching active groups.")
		return
	}
	defer rows.Close()

	var builder strings.Builder
	builder.WriteString("📡 <b>Active Groups & Channels Directory:</b>\n\n")

	count := 0
	for rows.Next() {
		var chatID int64
		var title, chatType string
		var isActive bool

		if err := rows.Scan(&chatID, &title, &chatType, &isActive); err == nil {
			count++
			vipBadge := "⚪ Free"
			if IsVIPChat(chatID) {
				vipBadge = "👑 VIP"
			}
			builder.WriteString(fmt.Sprintf("%d. <b>%s</b>\n   • <b>ID:</b> <code>%d</code> | <b>Type:</b> <code>%s</code> | %s\n",
				count, html.EscapeString(title), chatID, html.EscapeString(chatType), vipBadge))
		}
	}

	if count == 0 {
		builder.WriteString("<i>No groups registered yet. Add MiniMate to a group to track it here!</i>")
	} else {
		builder.WriteString(fmt.Sprintf("\n📊 <i>Total Active Tracked Groups: %d</i>", count))
	}

	sendHTMLMessage(bot, message.Chat.ID, builder.String())
}

// -------------------------
// BANNING & KICKING
// -------------------------

// HandleBan permanently bans a user
func HandleBan(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	if message.From == nil || !isAdmin(bot, message.Chat.ID, message.From.ID) {
		sendHTMLMessage(bot, message.Chat.ID, "❌ You must be an administrator to use this command.")
		return
	}
	if message.ReplyToMessage == nil || message.ReplyToMessage.From == nil {
		sendHTMLMessage(bot, message.Chat.ID, "❌ Reply to a valid user message to ban them.")
		return
	}

	target := message.ReplyToMessage.From
	if target.ID == bot.Self.ID {
		sendHTMLMessage(bot, message.Chat.ID, "❌ I cannot ban myself.")
		return
	}

	banConfig := tgbotapi.BanChatMemberConfig{
		ChatMemberConfig: tgbotapi.ChatMemberConfig{
			ChatID: message.Chat.ID,
			UserID: target.ID,
		},
	}

	_, err := bot.Request(banConfig)
	if err != nil {
		sendHTMLMessage(bot, message.Chat.ID, "❌ Failed to ban. Ensure I have admin permissions.")
		log.Printf("Ban failed: %v", err)
		return
	}

	sendHTMLMessage(bot, message.Chat.ID, fmt.Sprintf("🚫 <b>%s</b> has been banned.", html.EscapeString(target.FirstName)))
}

// HandleTBan temporarily bans a user (e.g., /tban 2d)
func HandleTBan(bot *tgbotapi.BotAPI, message *tgbotapi.Message, args string) {
	if message.From == nil || !isAdmin(bot, message.Chat.ID, message.From.ID) {
		sendHTMLMessage(bot, message.Chat.ID, "❌ You must be an administrator to use this command.")
		return
	}
	if message.ReplyToMessage == nil || message.ReplyToMessage.From == nil || strings.TrimSpace(args) == "" {
		sendHTMLMessage(bot, message.Chat.ID, "❌ Usage: Reply to a user with <code>/tban &lt;time&gt;</code> (e.g. <code>/tban 2h</code>, <code>/tban 1d</code>)")
		return
	}

	untilDate, err := parseDuration(strings.TrimSpace(args))
	if err != nil {
		sendHTMLMessage(bot, message.Chat.ID, fmt.Sprintf("❌ %s", html.EscapeString(err.Error())))
		return
	}

	target := message.ReplyToMessage.From
	if target.ID == bot.Self.ID {
		sendHTMLMessage(bot, message.Chat.ID, "❌ I cannot ban myself.")
		return
	}

	banConfig := tgbotapi.BanChatMemberConfig{
		ChatMemberConfig: tgbotapi.ChatMemberConfig{
			ChatID: message.Chat.ID,
			UserID: target.ID,
		},
		UntilDate: untilDate,
	}

	_, err = bot.Request(banConfig)
	if err != nil {
		sendHTMLMessage(bot, message.Chat.ID, "❌ Failed to temporarily ban user. Ensure I have admin rights.")
		return
	}

	sendHTMLMessage(bot, message.Chat.ID, fmt.Sprintf("⏳ <b>%s</b> has been temporarily banned for %s.", html.EscapeString(target.FirstName), html.EscapeString(args)))
}

// HandleUnban unbans a user
func HandleUnban(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	if message.From == nil || !isAdmin(bot, message.Chat.ID, message.From.ID) {
		sendHTMLMessage(bot, message.Chat.ID, "❌ You must be an administrator to use this command.")
		return
	}
	if message.ReplyToMessage == nil || message.ReplyToMessage.From == nil {
		sendHTMLMessage(bot, message.Chat.ID, "❌ Reply to a user's message to unban them.")
		return
	}

	target := message.ReplyToMessage.From
	unbanConfig := tgbotapi.UnbanChatMemberConfig{
		ChatMemberConfig: tgbotapi.ChatMemberConfig{
			ChatID: message.Chat.ID,
			UserID: target.ID,
		},
		OnlyIfBanned: true,
	}

	_, err := bot.Request(unbanConfig)
	if err != nil {
		sendHTMLMessage(bot, message.Chat.ID, "❌ Failed to unban user.")
		return
	}

	sendHTMLMessage(bot, message.Chat.ID, fmt.Sprintf("✅ <b>%s</b> has been unbanned.", html.EscapeString(target.FirstName)))
}

// HandleKick kicks a user from the chat (bans then unbans)
func HandleKick(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	if message.From == nil || !isAdmin(bot, message.Chat.ID, message.From.ID) {
		sendHTMLMessage(bot, message.Chat.ID, "❌ You must be an administrator to use this command.")
		return
	}
	if message.ReplyToMessage == nil || message.ReplyToMessage.From == nil {
		sendHTMLMessage(bot, message.Chat.ID, "❌ Reply to a user's message to kick them.")
		return
	}

	target := message.ReplyToMessage.From
	if target.ID == bot.Self.ID {
		sendHTMLMessage(bot, message.Chat.ID, "❌ I cannot kick myself.")
		return
	}

	banConfig := tgbotapi.BanChatMemberConfig{
		ChatMemberConfig: tgbotapi.ChatMemberConfig{ChatID: message.Chat.ID, UserID: target.ID},
	}
	bot.Request(banConfig)

	unbanConfig := tgbotapi.UnbanChatMemberConfig{
		ChatMemberConfig: tgbotapi.ChatMemberConfig{ChatID: message.Chat.ID, UserID: target.ID},
		OnlyIfBanned:     true,
	}
	bot.Request(unbanConfig)

	sendHTMLMessage(bot, message.Chat.ID, fmt.Sprintf("👢 <b>%s</b> has been kicked from the group.", html.EscapeString(target.FirstName)))
}

// -------------------------
// MUTING & UNMUTING
// -------------------------

// HandleMute permanently restricts a user from sending messages
func HandleMute(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	if message.From == nil || !isAdmin(bot, message.Chat.ID, message.From.ID) {
		sendHTMLMessage(bot, message.Chat.ID, "❌ You must be an administrator to use this command.")
		return
	}
	if message.ReplyToMessage == nil || message.ReplyToMessage.From == nil {
		sendHTMLMessage(bot, message.Chat.ID, "❌ Reply to a user's message to mute them.")
		return
	}

	target := message.ReplyToMessage.From
	if target.ID == bot.Self.ID {
		sendHTMLMessage(bot, message.Chat.ID, "❌ I cannot mute myself.")
		return
	}

	perms := tgbotapi.ChatPermissions{
		CanSendMessages: false,
	}

	restrictConfig := tgbotapi.RestrictChatMemberConfig{
		ChatMemberConfig: tgbotapi.ChatMemberConfig{
			ChatID: message.Chat.ID,
			UserID: target.ID,
		},
		Permissions: &perms,
	}

	_, err := bot.Request(restrictConfig)
	if err != nil {
		sendHTMLMessage(bot, message.Chat.ID, "❌ Failed to mute user. Ensure I have admin rights.")
		return
	}

	sendHTMLMessage(bot, message.Chat.ID, fmt.Sprintf("🤐 <b>%s</b> has been muted.", html.EscapeString(target.FirstName)))
}

// HandleTMute temporarily restricts a user
func HandleTMute(bot *tgbotapi.BotAPI, message *tgbotapi.Message, args string) {
	if message.From == nil || !isAdmin(bot, message.Chat.ID, message.From.ID) {
		sendHTMLMessage(bot, message.Chat.ID, "❌ You must be an administrator to use this command.")
		return
	}
	if message.ReplyToMessage == nil || message.ReplyToMessage.From == nil || strings.TrimSpace(args) == "" {
		sendHTMLMessage(bot, message.Chat.ID, "❌ Usage: Reply with <code>/tmute &lt;time&gt;</code> (e.g. <code>/tmute 30m</code>, <code>/tmute 2h</code>)")
		return
	}

	untilDate, err := parseDuration(strings.TrimSpace(args))
	if err != nil {
		sendHTMLMessage(bot, message.Chat.ID, fmt.Sprintf("❌ %s", html.EscapeString(err.Error())))
		return
	}

	target := message.ReplyToMessage.From
	if target.ID == bot.Self.ID {
		sendHTMLMessage(bot, message.Chat.ID, "❌ I cannot mute myself.")
		return
	}

	perms := tgbotapi.ChatPermissions{
		CanSendMessages: false,
	}

	restrictConfig := tgbotapi.RestrictChatMemberConfig{
		ChatMemberConfig: tgbotapi.ChatMemberConfig{
			ChatID: message.Chat.ID,
			UserID: target.ID,
		},
		Permissions: &perms,
		UntilDate:   untilDate,
	}

	_, err = bot.Request(restrictConfig)
	if err != nil {
		sendHTMLMessage(bot, message.Chat.ID, "❌ Failed to temporarily mute user.")
		return
	}

	sendHTMLMessage(bot, message.Chat.ID, fmt.Sprintf("🤐 <b>%s</b> has been muted for %s.", html.EscapeString(target.FirstName), html.EscapeString(args)))
}

// HandleUnmute restores a user's permissions
func HandleUnmute(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	if message.From == nil || !isAdmin(bot, message.Chat.ID, message.From.ID) {
		sendHTMLMessage(bot, message.Chat.ID, "❌ You must be an administrator to use this command.")
		return
	}
	if message.ReplyToMessage == nil || message.ReplyToMessage.From == nil {
		sendHTMLMessage(bot, message.Chat.ID, "❌ Reply to a user's message to unmute them.")
		return
	}

	target := message.ReplyToMessage.From
	perms := tgbotapi.ChatPermissions{
		CanSendMessages:       true,
		CanSendMediaMessages:  true,
		CanSendPolls:          true,
		CanSendOtherMessages:  true,
		CanAddWebPagePreviews: true,
	}

	restrictConfig := tgbotapi.RestrictChatMemberConfig{
		ChatMemberConfig: tgbotapi.ChatMemberConfig{
			ChatID: message.Chat.ID,
			UserID: target.ID,
		},
		Permissions: &perms,
	}

	_, err := bot.Request(restrictConfig)
	if err != nil {
		sendHTMLMessage(bot, message.Chat.ID, "❌ Failed to unmute user.")
		return
	}

	sendHTMLMessage(bot, message.Chat.ID, fmt.Sprintf("🔊 <b>%s</b> has been unmuted.", html.EscapeString(target.FirstName)))
}

// -------------------------
// PROMOTIONS & ADMIN ROLES
// -------------------------

// HandlePromote promotes a user with standard admin privileges
func HandlePromote(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	if message.From == nil || !isAdmin(bot, message.Chat.ID, message.From.ID) {
		sendHTMLMessage(bot, message.Chat.ID, "❌ You must be an administrator to use this command.")
		return
	}
	if message.ReplyToMessage == nil || message.ReplyToMessage.From == nil {
		sendHTMLMessage(bot, message.Chat.ID, "❌ Reply to a user's message to promote them.")
		return
	}

	target := message.ReplyToMessage.From
	promoteConfig := tgbotapi.PromoteChatMemberConfig{
		ChatMemberConfig: tgbotapi.ChatMemberConfig{
			ChatID: message.Chat.ID,
			UserID: target.ID,
		},
		CanDeleteMessages:  true,
		CanInviteUsers:     true,
		CanRestrictMembers: true,
		CanPinMessages:     true,
	}

	_, err := bot.Request(promoteConfig)
	if err != nil {
		sendHTMLMessage(bot, message.Chat.ID, "❌ Failed to promote user.")
		return
	}

	sendHTMLMessage(bot, message.Chat.ID, fmt.Sprintf("⭐ <b>%s</b> has been promoted to Admin!", html.EscapeString(target.FirstName)))
}

// HandleDemote strips admin privileges from a user
func HandleDemote(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	if message.From == nil || !isAdmin(bot, message.Chat.ID, message.From.ID) {
		sendHTMLMessage(bot, message.Chat.ID, "❌ You must be an administrator to use this command.")
		return
	}
	if message.ReplyToMessage == nil || message.ReplyToMessage.From == nil {
		sendHTMLMessage(bot, message.Chat.ID, "❌ Reply to an admin to demote them.")
		return
	}

	target := message.ReplyToMessage.From
	demoteConfig := tgbotapi.PromoteChatMemberConfig{
		ChatMemberConfig: tgbotapi.ChatMemberConfig{
			ChatID: message.Chat.ID,
			UserID: target.ID,
		},
		CanChangeInfo:      false,
		CanPostMessages:    false,
		CanEditMessages:    false,
		CanDeleteMessages:  false,
		CanInviteUsers:     false,
		CanRestrictMembers: false,
		CanPinMessages:     false,
		CanPromoteMembers:  false,
	}

	_, err := bot.Request(demoteConfig)
	if err != nil {
		sendHTMLMessage(bot, message.Chat.ID, "❌ Failed to demote user.")
		return
	}

	sendHTMLMessage(bot, message.Chat.ID, fmt.Sprintf("🔽 <b>%s</b> has been demoted.", html.EscapeString(target.FirstName)))
}

// HandleAdminList lists all group admins
func HandleAdminList(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	config := tgbotapi.ChatAdministratorsConfig{
		ChatConfig: tgbotapi.ChatConfig{ChatID: message.Chat.ID},
	}

	admins, err := bot.GetChatAdministrators(config)
	if err != nil {
		sendHTMLMessage(bot, message.Chat.ID, "❌ Failed to fetch admin list.")
		return
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("🛡️ <b>Admins in %s:</b>\n\n", html.EscapeString(message.Chat.Title)))

	for _, admin := range admins {
		title := admin.CustomTitle
		if title == "" {
			if admin.Status == "creator" {
				title = "Creator"
			} else {
				title = "Admin"
			}
		}
		builder.WriteString(fmt.Sprintf("• %s (<code>%s</code>)\n", html.EscapeString(admin.User.FirstName), html.EscapeString(title)))
	}

	sendHTMLMessage(bot, message.Chat.ID, builder.String())
}

// HandleInviteLink exports the chat invite link using the built-in v5 method
func HandleInviteLink(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	if message.From == nil || !isAdmin(bot, message.Chat.ID, message.From.ID) {
		sendHTMLMessage(bot, message.Chat.ID, "❌ You must be an administrator to use this command.")
		return
	}

	config := tgbotapi.ChatInviteLinkConfig{
		ChatConfig: tgbotapi.ChatConfig{ChatID: message.Chat.ID},
	}

	link, err := bot.GetInviteLink(config)
	if err != nil {
		sendHTMLMessage(bot, message.Chat.ID, "❌ Failed to export invite link.")
		return
	}

	sendHTMLMessage(bot, message.Chat.ID, fmt.Sprintf("🔗 <b>Group Invite Link:</b>\n%s", html.EscapeString(link)))
}

// HandleTitle sets a custom admin title
func HandleTitle(bot *tgbotapi.BotAPI, message *tgbotapi.Message, args string) {
	sendHTMLMessage(bot, message.Chat.ID, "❌ Custom admin titles are pending wrapper support.")
}

// -------------------------
// VIP SUBSCRIPTIONS
// -------------------------

// HandleVIPCommand manages chat VIP and Premium status
func HandleVIPCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message, cmd string, args string) {
	chatID := message.Chat.ID
	fromID := int64(0)
	if message.From != nil {
		fromID = message.From.ID
	} else if message.SenderChat != nil {
		fromID = message.SenderChat.ID
	}

	switch cmd {
	case "checkvip", "vipstatus", "premium":
		isVIP := IsVIPChat(chatID)
		if isVIP {
			var expiresAt *time.Time
			var planName string
			database.Pool.QueryRow(context.Background(),
				"SELECT expires_at, plan_name FROM chat_subscriptions WHERE chat_id = $1", chatID).
				Scan(&expiresAt, &planName)

			expiryStr := "Lifetime"
			if expiresAt != nil {
				expiryStr = expiresAt.Format("02 Jan 2006 15:04 MST")
			}

			text := fmt.Sprintf(`👑 <b>𝐌𝐢𝐧𝐢𝐌𝐚𝐭𝐞 𝐏𝐫𝐞𝐦𝐢𝐮𝐦 𝐀𝐜𝐭𝐢𝐯𝐚𝐭𝐞𝐝!</b>

<blockquote expandable>• <b>Plan:</b> %s
• <b>Status:</b> 🟢 Active
• <b>Expires:</b> %s
• <b>Unlocked:</b> Unlimited Filters, Security Shield, Math Captcha, Priority Routing</blockquote>`,
				html.EscapeString(planName), expiryStr)
			sendHTMLMessage(bot, chatID, text)
		} else {
			text := `👑 <b>𝐌𝐢𝐧𝐢𝐌𝐚𝐭𝐞 𝐅𝐫𝐞𝐞 𝐓𝐢𝐞𝐫</b>

<blockquote>This chat is currently on the Standard plan. Contact bot owner to activate VIP access for advanced anti-raid locks & unlimited storage.</blockquote>`
			sendHTMLMessage(bot, chatID, text)
		}

	case "setvip":
		if !isAdmin(bot, chatID, fromID) {
			sendHTMLMessage(bot, chatID, "❌ Only administrators can manage VIP subscriptions.")
			return
		}

		parts := strings.Fields(args)
		if len(parts) < 2 {
			sendHTMLMessage(bot, chatID, "❌ Usage: <code>/setvip &lt;chat_id&gt; &lt;days&gt;</code> (use days=0 for lifetime)")
			return
		}

		targetChatID, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			sendHTMLMessage(bot, chatID, "❌ Invalid Chat ID.")
			return
		}

		days, err := strconv.Atoi(parts[1])
		if err != nil || days < 0 {
			sendHTMLMessage(bot, chatID, "❌ Invalid number of days.")
			return
		}

		var expiresAt *time.Time
		if days > 0 {
			exp := time.Now().Add(time.Duration(days) * 24 * time.Hour)
			expiresAt = &exp
		}

		query := `
			INSERT INTO chat_subscriptions (chat_id, is_vip, expires_at, plan_name)
			VALUES ($1, true, $2, 'VIP PRO')
			ON CONFLICT (chat_id) DO UPDATE SET
				is_vip = true,
				expires_at = EXCLUDED.expires_at,
				plan_name = 'VIP PRO';
		`
		_, err = database.Pool.Exec(context.Background(), query, targetChatID, expiresAt)
		if err != nil {
			sendHTMLMessage(bot, chatID, "❌ Database error updating VIP subscription.")
			return
		}

		sendHTMLMessage(bot, chatID, fmt.Sprintf("✅ <b>Chat %d</b> has been upgraded to <b>VIP PRO</b> for %d days!", targetChatID, days))

	case "rmvip":
		if !isAdmin(bot, chatID, fromID) {
			sendHTMLMessage(bot, chatID, "❌ Only administrators can remove VIP subscriptions.")
			return
		}

		targetChatID, err := strconv.ParseInt(strings.TrimSpace(args), 10, 64)
		if err != nil {
			sendHTMLMessage(bot, chatID, "❌ Usage: <code>/rmvip &lt;chat_id&gt;</code>")
			return
		}

		query := "UPDATE chat_subscriptions SET is_vip = false WHERE chat_id = $1"
		database.Pool.Exec(context.Background(), query, targetChatID)

		sendHTMLMessage(bot, chatID, fmt.Sprintf("🗑️ VIP status revoked for Chat <code>%d</code>.", targetChatID))
	}
}

// -------------------------
// CLEANUP & PINS
// -------------------------

// HandleDel deletes a single replied message
func HandleDel(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	if message.From == nil || !isAdmin(bot, message.Chat.ID, message.From.ID) {
		return
	}
	if message.ReplyToMessage == nil {
		return
	}

	bot.Request(tgbotapi.NewDeleteMessage(message.Chat.ID, message.ReplyToMessage.MessageID))
	bot.Request(tgbotapi.NewDeleteMessage(message.Chat.ID, message.MessageID))
}

// HandlePurge deletes messages between the replied message and the command
func HandlePurge(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	if message.From == nil || !isAdmin(bot, message.Chat.ID, message.From.ID) {
		sendHTMLMessage(bot, message.Chat.ID, "❌ Only admins can purge messages.")
		return
	}
	if message.ReplyToMessage == nil {
		sendHTMLMessage(bot, message.Chat.ID, "❌ Reply to the message where you want the purge to start.")
		return
	}

	startID := message.ReplyToMessage.MessageID
	endID := message.MessageID

	if startID > endID {
		startID, endID = endID, startID
	}

	go func(chatID int64, start, end int) {
		count := 0
		for id := start; id <= end; id++ {
			del := tgbotapi.NewDeleteMessage(chatID, id)
			_, err := bot.Request(del)
			if err == nil {
				count++
			}
			time.Sleep(30 * time.Millisecond)
		}

		confirm := tgbotapi.NewMessage(chatID, fmt.Sprintf("🧹 Purged <b>%d</b> messages.", count))
		confirm.ParseMode = "HTML"
		sent, err := bot.Send(confirm)
		if err == nil {
			time.Sleep(3 * time.Second)
			bot.Request(tgbotapi.NewDeleteMessage(chatID, sent.MessageID))
		}
	}(message.Chat.ID, startID, endID)
}

// HandlePinCommand handles pin, unpin, unpinall
func HandlePinCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message, cmd string) {
	if message.From == nil || !isAdmin(bot, message.Chat.ID, message.From.ID) {
		sendHTMLMessage(bot, message.Chat.ID, "❌ You must be an administrator to pin/unpin messages.")
		return
	}

	switch cmd {
	case "pin":
		if message.ReplyToMessage == nil {
			sendHTMLMessage(bot, message.Chat.ID, "❌ Reply to a message to pin it.")
			return
		}
		pinConfig := tgbotapi.PinChatMessageConfig{
			ChatID:              message.Chat.ID,
			MessageID:           message.ReplyToMessage.MessageID,
			DisableNotification: false,
		}
		_, err := bot.Request(pinConfig)
		if err == nil {
			sendHTMLMessage(bot, message.Chat.ID, "📌 Message pinned!")
		}

	case "unpin":
		unpinConfig := tgbotapi.UnpinChatMessageConfig{
			ChatID: message.Chat.ID,
		}
		if message.ReplyToMessage != nil {
			unpinConfig.MessageID = message.ReplyToMessage.MessageID
		}
		_, err := bot.Request(unpinConfig)
		if err == nil {
			sendHTMLMessage(bot, message.Chat.ID, "📌 Message unpinned.")
		}

	case "unpinall":
		unpinAllConfig := tgbotapi.UnpinAllChatMessagesConfig{
			ChatID: message.Chat.ID,
		}
		_, err := bot.Request(unpinAllConfig)
		if err == nil {
			sendHTMLMessage(bot, message.Chat.ID, "📌 All messages have been unpinned.")
		}
	}
}