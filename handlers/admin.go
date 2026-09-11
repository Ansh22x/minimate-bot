package handlers

import (
	"context"
	"fmt"
	"html"
	"log"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"minimate-bot/config"
	"minimate-bot/database"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var (
	botAdminsCache  = make(map[int64]bool)
	botAdminsMutex  sync.RWMutex
	botAdminsLoaded = false
)

// isOwner checks if a user is the primary Bot Owner
func isOwner(userID int64) bool {
	if userID == 0 {
		return false
	}
	if config.OwnerID != 0 && userID == config.OwnerID {
		return true
	}
	return userID == 1087968824
}

// isBotAdmin checks if a user is the Owner or a designated Bot Admin (Sudo)
func isBotAdmin(userID int64) bool {
	if isOwner(userID) {
		return true
	}
	if userID == 0 {
		return false
	}

	botAdminsMutex.RLock()
	if botAdminsLoaded {
		isAdminUser := botAdminsCache[userID]
		botAdminsMutex.RUnlock()
		return isAdminUser
	}
	botAdminsMutex.RUnlock()

	// Load from database if not loaded yet
	botAdminsMutex.Lock()
	defer botAdminsMutex.Unlock()
	rows, err := database.Pool.Query(context.Background(), "SELECT user_id FROM bot_admins")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var uid int64
			if err := rows.Scan(&uid); err == nil {
				botAdminsCache[uid] = true
			}
		}
		botAdminsLoaded = true
	}
	return botAdminsCache[userID]
}

// isAdmin checks if a given user has administrator privileges in the chat
func isAdmin(bot *tgbotapi.BotAPI, chatID int64, userID int64) bool {
	if isBotAdmin(userID) {
		return true
	}
	if userID == 0 {
		return false
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
	return SafeSend(bot, msg)
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

// HandlePromote promotes a user with tiered privileges (Jr. Admin / Sr. Admin)
func HandlePromote(bot *tgbotapi.BotAPI, message *tgbotapi.Message, cmd string, args string) {
	if message.From == nil || !isAdmin(bot, message.Chat.ID, message.From.ID) {
		sendHTMLMessage(bot, message.Chat.ID, "❌ You must be an administrator to use this command.")
		return
	}
	if message.ReplyToMessage == nil || message.ReplyToMessage.From == nil {
		sendHTMLMessage(bot, message.Chat.ID, "❌ Reply to a user's message with <code>/promote</code> to promote them.")
		return
	}

	target := message.ReplyToMessage.From
	if target.ID == bot.Self.ID {
		sendHTMLMessage(bot, message.Chat.ID, "🤖 I am already the bot administrator.")
		return
	}

	chatID := message.Chat.ID
	cleanArgs := strings.TrimSpace(args)
	argsLower := strings.ToLower(cleanArgs)
	cmdLower := strings.ToLower(cmd)

	// Fetch target's current chat member status
	memberConfig := tgbotapi.GetChatMemberConfig{
		ChatConfigWithUser: tgbotapi.ChatConfigWithUser{
			ChatID: chatID,
			UserID: target.ID,
		},
	}
	currentMember, _ := bot.GetChatMember(memberConfig)

	// Determine requested promotion level
	isExplicitSenior := cmdLower == "fullpromote" || cmdLower == "spromote" || cmdLower == "snrpromote" ||
		strings.HasPrefix(argsLower, "sr") || strings.HasPrefix(argsLower, "snr") || strings.HasPrefix(argsLower, "senior") || strings.HasPrefix(argsLower, "full")
	isExplicitJunior := cmdLower == "jrpromote" || cmdLower == "jpromote" ||
		strings.HasPrefix(argsLower, "jr") || strings.HasPrefix(argsLower, "junior") || strings.HasPrefix(argsLower, "low")

	level := "junior"
	customTitle := ""

	if isExplicitSenior {
		level = "senior"
		fields := strings.Fields(cleanArgs)
		if len(fields) > 1 {
			customTitle = strings.Join(fields[1:], " ")
		}
	} else if isExplicitJunior {
		level = "junior"
		fields := strings.Fields(cleanArgs)
		if len(fields) > 1 {
			customTitle = strings.Join(fields[1:], " ")
		}
	} else {
		// Custom title provided without explicit level keyword (e.g. "/promote Moderator")
		if cleanArgs != "" {
			customTitle = cleanArgs
		}

		// Smart Tiering:
		// If user is already an admin, promote to Senior Admin (Full Rights)!
		// If user is a regular member, promote to Junior Admin!
		if currentMember.Status == "administrator" {
			level = "senior"
		} else {
			level = "junior"
		}
	}

	if customTitle == "" {
		if level == "senior" {
			customTitle = "Sr. Admin"
		} else {
			customTitle = "Jr. Admin"
		}
	}
	if len(customTitle) > 16 {
		customTitle = customTitle[:16]
	}

	var err error
	if message.Chat.IsChannel() {
		promoteParams := tgbotapi.Params{
			"chat_id":             strconv.FormatInt(chatID, 10),
			"user_id":             strconv.FormatInt(target.ID, 10),
			"can_manage_chat":     "true",
			"can_post_messages":   "true",
			"can_edit_messages":   "true",
			"can_delete_messages": "true",
			"can_invite_users":    "true",
			"can_change_info":     strconv.FormatBool(level == "senior"),
			"can_promote_members": "false",
		}
		_, err = bot.MakeRequest("promoteChatMember", promoteParams)
	} else {
		promoteParams := tgbotapi.Params{
			"chat_id":                strconv.FormatInt(chatID, 10),
			"user_id":                strconv.FormatInt(target.ID, 10),
			"can_manage_chat":        "true",
			"can_change_info":        strconv.FormatBool(level == "senior"),
			"can_delete_messages":    "true",
			"can_invite_users":       "true",
			"can_restrict_members":   strconv.FormatBool(level == "senior"),
			"can_pin_messages":       "true",
			"can_promote_members":    "false",
			"can_manage_video_chats": "true",
		}
		_, err = bot.MakeRequest("promoteChatMember", promoteParams)
	}

	if err != nil {
		log.Printf("[Promote] Error promoting user %d: %v", target.ID, err)
		errStr := err.Error()
		if strings.Contains(strings.ToLower(errStr), "right") || strings.Contains(strings.ToLower(errStr), "admin") {
			sendHTMLMessage(bot, chatID, "❌ <b>Cannot promote:</b> Make sure the bot has <b>Add new admins</b> permission.")
		} else {
			sendHTMLMessage(bot, chatID, fmt.Sprintf("❌ Failed to promote user: <i>%s</i>", html.EscapeString(errStr)))
		}
		return
	}

	// Apply custom title
	titleParams := tgbotapi.Params{
		"chat_id":      strconv.FormatInt(chatID, 10),
		"user_id":      strconv.FormatInt(target.ID, 10),
		"custom_title": customTitle,
	}
	_, _ = bot.MakeRequest("setChatAdministratorCustomTitle", titleParams)

	var text string
	if level == "senior" {
		text = fmt.Sprintf(`🛡️ <b>Senior Administrator Promoted!</b>

👤 <b>User:</b> <b>%s</b>
🏷️ <b>Title:</b> <code>%s</code>
⚡ <b>Tier:</b> <b>Sr. Admin (Full Moderation Rights)</b>

<blockquote expandable>✅ <b>Assigned Permissions:</b>
• 🚫 Ban, Mute &amp; Kick Members
• 🗑️ Delete Messages
• ⚙️ Change Group Info &amp; Settings
• 🔗 Invite Users via Link
• 📌 Pin &amp; Unpin Messages
• 🎙️ Manage Video Chats</blockquote>`,
			html.EscapeString(target.FirstName), html.EscapeString(customTitle))
	} else {
		text = fmt.Sprintf(`🎖️ <b>Junior Administrator Promoted!</b>

👤 <b>User:</b> <b>%s</b>
🏷️ <b>Title:</b> <code>%s</code>
🔰 <b>Tier:</b> <b>Jr. Admin (Junior Moderator)</b>

<blockquote expandable>✅ <b>Assigned Permissions:</b>
• 🗑️ Delete Messages
• 🔗 Invite Users via Link
• 📌 Pin &amp; Unpin Messages
• 🎙️ Manage Video Chats

🔒 <b>Restricted (Disabled):</b>
• ❌ Ban / Mute / Kick Members
• ❌ Change Group Info
• ❌ Add New Admins</blockquote>

💡 <i>Reply with <code>/promote</code> again to upgrade this user to <b>Sr. Admin</b> with full permissions!</i>`,
			html.EscapeString(target.FirstName), html.EscapeString(customTitle))
	}

	sendHTMLMessage(bot, chatID, text)
}

// HandleDemote strips all admin privileges from a user
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

	// Prepare exact parameters per chat type to avoid Telegram BOT_CHANNELS_NA
	var demoteParams tgbotapi.Params
	if message.Chat.IsChannel() {
		demoteParams = tgbotapi.Params{
			"chat_id":             strconv.FormatInt(message.Chat.ID, 10),
			"user_id":             strconv.FormatInt(target.ID, 10),
			"can_manage_chat":     "false",
			"can_change_info":     "false",
			"can_post_messages":   "false",
			"can_edit_messages":   "false",
			"can_delete_messages": "false",
			"can_invite_users":    "false",
			"can_promote_members": "false",
		}
	} else {
		demoteParams = tgbotapi.Params{
			"chat_id":                strconv.FormatInt(message.Chat.ID, 10),
			"user_id":                strconv.FormatInt(target.ID, 10),
			"is_anonymous":           "false",
			"can_manage_chat":        "false",
			"can_change_info":        "false",
			"can_delete_messages":    "false",
			"can_manage_video_chats": "false",
			"can_invite_users":       "false",
			"can_restrict_members":   "false",
			"can_pin_messages":       "false",
			"can_promote_members":    "false",
		}
	}

	_, err := bot.MakeRequest("promoteChatMember", demoteParams)
	if err != nil {
		errStr := err.Error()
		log.Printf("[Demote] Initial attempt failed for user %d: %v. Retrying with minimal params...", target.ID, err)

		// Fallback retry with universal minimal parameters if chat type was misidentified or strict mode
		fallbackParams := tgbotapi.Params{
			"chat_id":             strconv.FormatInt(message.Chat.ID, 10),
			"user_id":             strconv.FormatInt(target.ID, 10),
			"can_change_info":     "false",
			"can_delete_messages": "false",
			"can_invite_users":    "false",
			"can_promote_members": "false",
		}
		_, fallbackErr := bot.MakeRequest("promoteChatMember", fallbackParams)
		if fallbackErr == nil {
			sendHTMLMessage(bot, message.Chat.ID, fmt.Sprintf("🔽 <b>%s</b> has been demoted.", html.EscapeString(target.FirstName)))
			return
		}

		errLower := strings.ToLower(errStr)
		if strings.Contains(errLower, "not promoted by bot") || strings.Contains(errLower, "creator") {
			sendHTMLMessage(bot, message.Chat.ID, "❌ <b>Cannot demote:</b> This user was appointed by the Group Creator or another admin.\n\n<i>Note: Telegram only allows bots to demote admins that were promoted by the bot itself.</i>")
		} else if strings.Contains(errLower, "right") || strings.Contains(errLower, "admin") || strings.Contains(errLower, "privilege") {
			sendHTMLMessage(bot, message.Chat.ID, "❌ <b>Cannot demote:</b> The bot lacks the required administrator rights (e.g. <i>Add New Admins</i>).")
		} else if strings.Contains(errLower, "bot_channels_na") {
			sendHTMLMessage(bot, message.Chat.ID, "❌ <b>Cannot demote:</b> This user cannot be demoted with current chat permissions.")
		} else {
			sendHTMLMessage(bot, message.Chat.ID, fmt.Sprintf("❌ Failed to demote user: <i>%s</i>", html.EscapeString(errStr)))
		}
		return
	}

	sendHTMLMessage(bot, message.Chat.ID, fmt.Sprintf("🔽 <b>%s</b> has been demoted.", html.EscapeString(target.FirstName)))
}

// HandleAdminList lists all group admins categorized into distinct tiered rows
func HandleAdminList(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	config := tgbotapi.ChatAdministratorsConfig{
		ChatConfig: tgbotapi.ChatConfig{ChatID: message.Chat.ID},
	}

	admins, err := bot.GetChatAdministrators(config)
	if err != nil {
		sendHTMLMessage(bot, message.Chat.ID, "❌ Failed to fetch admin list.")
		return
	}

	var creators []tgbotapi.ChatMember
	var seniors []tgbotapi.ChatMember
	var juniors []tgbotapi.ChatMember
	var bots []tgbotapi.ChatMember

	for _, admin := range admins {
		if admin.User.IsBot {
			bots = append(bots, admin)
		} else if admin.Status == "creator" {
			creators = append(creators, admin)
		} else if admin.CanRestrictMembers || admin.CanChangeInfo {
			seniors = append(seniors, admin)
		} else {
			juniors = append(juniors, admin)
		}
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("🛡️ <b>Staff &amp; Administrators Directory:</b>\n💬 <b>Group:</b> <b>%s</b>\n\n", html.EscapeString(message.Chat.Title)))

	formatAdmin := func(m tgbotapi.ChatMember) string {
		name := html.EscapeString(m.User.FirstName)
		userTag := ""
		if m.User.UserName != "" {
			userTag = fmt.Sprintf(" (@%s)", html.EscapeString(m.User.UserName))
		}
		title := m.CustomTitle
		if title == "" {
			if m.Status == "creator" {
				title = "Owner"
			} else if m.CanRestrictMembers || m.CanChangeInfo {
				title = "Sr. Admin"
			} else {
				title = "Jr. Admin"
			}
		}
		return fmt.Sprintf("• <b>%s</b>%s — <code>%s</code>\n", name, userTag, html.EscapeString(title))
	}

	// 1. Group Creator / Owner
	builder.WriteString("👑 <b>Group Creator / Owner:</b>\n")
	if len(creators) > 0 {
		for _, c := range creators {
			builder.WriteString(formatAdmin(c))
		}
	} else {
		builder.WriteString("• <i>Hidden or Anonymous</i>\n")
	}
	builder.WriteString("\n")

	// 2. Senior Administrators (Full Rights)
	builder.WriteString("🛡️ <b>Senior Administrators (Full Rights):</b>\n")
	if len(seniors) > 0 {
		for _, s := range seniors {
			builder.WriteString(formatAdmin(s))
		}
	} else {
		builder.WriteString("• <i>None assigned</i>\n")
	}
	builder.WriteString("\n")

	// 3. Junior Administrators (Moderators)
	builder.WriteString("🎖️ <b>Junior Administrators (Moderators):</b>\n")
	if len(juniors) > 0 {
		for _, j := range juniors {
			builder.WriteString(formatAdmin(j))
		}
	} else {
		builder.WriteString("• <i>None assigned</i>\n")
	}
	builder.WriteString("\n")

	// 4. Bot Administrators
	if len(bots) > 0 {
		builder.WriteString("🤖 <b>Bot Administrators:</b>\n")
		for _, b := range bots {
			builder.WriteString(formatAdmin(b))
		}
		builder.WriteString("\n")
	}

	// Summary statistics footer
	builder.WriteString(fmt.Sprintf("<blockquote expandable>📊 <b>Total Staff:</b> <code>%d</code> (👑 %d Owner, 🛡️ %d Senior, 🎖️ %d Junior, 🤖 %d Bots)</blockquote>",
		len(admins), len(creators), len(seniors), len(juniors), len(bots)))

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

// HandleTitle sets a custom admin title for an administrator
func HandleTitle(bot *tgbotapi.BotAPI, message *tgbotapi.Message, args string) {
	if message.From == nil || !isAdmin(bot, message.Chat.ID, message.From.ID) {
		sendHTMLMessage(bot, message.Chat.ID, "❌ You must be an administrator to use this command.")
		return
	}
	if message.ReplyToMessage == nil || message.ReplyToMessage.From == nil {
		sendHTMLMessage(bot, message.Chat.ID, "❌ Reply to an administrator with <code>/title &lt;custom_title&gt;</code> to set their title.")
		return
	}

	title := strings.TrimSpace(args)
	if title == "" {
		sendHTMLMessage(bot, message.Chat.ID, "❌ Please specify a title.\nExample: <code>/title Moderator</code>")
		return
	}
	if len(title) > 16 {
		sendHTMLMessage(bot, message.Chat.ID, "❌ Custom title cannot exceed 16 characters.")
		return
	}

	target := message.ReplyToMessage.From
	titleParams := tgbotapi.Params{
		"chat_id":      strconv.FormatInt(message.Chat.ID, 10),
		"user_id":      strconv.FormatInt(target.ID, 10),
		"custom_title": title,
	}
	_, err := bot.MakeRequest("setChatAdministratorCustomTitle", titleParams)
	if err != nil {
		errStr := err.Error()
		log.Printf("[Title] Failed to set title for %d: %v", target.ID, err)
		sendHTMLMessage(bot, message.Chat.ID, fmt.Sprintf("❌ Failed to set title: <i>%s</i>\n\n<i>Note: Bots can only edit custom titles for administrators that were promoted by the bot itself.</i>", html.EscapeString(errStr)))
		return
	}

	sendHTMLMessage(bot, message.Chat.ID, fmt.Sprintf("✅ Custom title for <b>%s</b> has been set to <code>%s</code>.", html.EscapeString(target.FirstName), html.EscapeString(title)))
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

// -------------------------
// BOT ADMIN (SUDO) MANAGEMENT
// -------------------------

// HandleAddBotAdmin adds a new global bot administrator (Owner only)
func HandleAddBotAdmin(bot *tgbotapi.BotAPI, message *tgbotapi.Message, args string) {
	fromID := int64(0)
	if message.From != nil {
		fromID = message.From.ID
	}
	if !isOwner(fromID) {
		sendHTMLMessage(bot, message.Chat.ID, "❌ Only the <b>Bot Owner</b> can add bot administrators.")
		return
	}

	var targetID int64
	var targetUsername string
	var targetFirstName string

	if message.ReplyToMessage != nil && message.ReplyToMessage.From != nil {
		targetID = message.ReplyToMessage.From.ID
		targetUsername = message.ReplyToMessage.From.UserName
		targetFirstName = message.ReplyToMessage.From.FirstName
	} else if strings.TrimSpace(args) != "" {
		id, err := strconv.ParseInt(strings.TrimSpace(args), 10, 64)
		if err != nil {
			sendHTMLMessage(bot, message.Chat.ID, "❌ Invalid User ID. Usage: <code>/addbotadmin &lt;user_id&gt;</code> or reply to a user.")
			return
		}
		targetID = id
		targetFirstName = fmt.Sprintf("User %d", id)
	} else {
		sendHTMLMessage(bot, message.Chat.ID, "❌ Usage: Reply to a user with <code>/addbotadmin</code> or provide a User ID: <code>/addbotadmin &lt;user_id&gt;</code>")
		return
	}

	if targetID == bot.Self.ID || targetID == fromID {
		sendHTMLMessage(bot, message.Chat.ID, "❌ Cannot add bot or owner as bot admin.")
		return
	}

	query := `
		INSERT INTO bot_admins (user_id, user_name, first_name, added_by)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id) DO UPDATE SET
			user_name = EXCLUDED.user_name,
			first_name = EXCLUDED.first_name;
	`
	_, err := database.Pool.Exec(context.Background(), query, targetID, targetUsername, targetFirstName, fromID)
	if err != nil {
		sendHTMLMessage(bot, message.Chat.ID, "❌ Database error adding bot administrator.")
		return
	}

	botAdminsMutex.Lock()
	botAdminsCache[targetID] = true
	botAdminsMutex.Unlock()

	sendHTMLMessage(bot, message.Chat.ID, fmt.Sprintf("👑 <b>%s</b> (<code>%d</code>) is now a <b>Bot Administrator (Sudo)</b>!\nThey can now manage VIP subscriptions, broadcast announcements, and control bot departures.", html.EscapeString(targetFirstName), targetID))
}

// HandleRmBotAdmin removes a global bot administrator (Owner only)
func HandleRmBotAdmin(bot *tgbotapi.BotAPI, message *tgbotapi.Message, args string) {
	fromID := int64(0)
	if message.From != nil {
		fromID = message.From.ID
	}
	if !isOwner(fromID) {
		sendHTMLMessage(bot, message.Chat.ID, "❌ Only the <b>Bot Owner</b> can remove bot administrators.")
		return
	}

	var targetID int64
	if message.ReplyToMessage != nil && message.ReplyToMessage.From != nil {
		targetID = message.ReplyToMessage.From.ID
	} else if strings.TrimSpace(args) != "" {
		id, err := strconv.ParseInt(strings.TrimSpace(args), 10, 64)
		if err != nil {
			sendHTMLMessage(bot, message.Chat.ID, "❌ Invalid User ID. Usage: <code>/rmbotadmin &lt;user_id&gt;</code> or reply to a user.")
			return
		}
		targetID = id
	} else {
		sendHTMLMessage(bot, message.Chat.ID, "❌ Usage: Reply to a user with <code>/rmbotadmin</code> or provide a User ID: <code>/rmbotadmin &lt;user_id&gt;</code>")
		return
	}

	_, err := database.Pool.Exec(context.Background(), "DELETE FROM bot_admins WHERE user_id = $1", targetID)
	if err != nil {
		sendHTMLMessage(bot, message.Chat.ID, "❌ Database error removing bot administrator.")
		return
	}

	botAdminsMutex.Lock()
	delete(botAdminsCache, targetID)
	botAdminsMutex.Unlock()

	sendHTMLMessage(bot, message.Chat.ID, fmt.Sprintf("🗑️ Bot Administrator rights revoked for user <code>%d</code>.", targetID))
}

// HandleListBotAdmins lists all active bot administrators
func HandleListBotAdmins(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	fromID := int64(0)
	if message.From != nil {
		fromID = message.From.ID
	}
	if !isBotAdmin(fromID) {
		sendHTMLMessage(bot, message.Chat.ID, "❌ This command is restricted to <b>Bot Administrators</b>.")
		return
	}

	rows, err := database.Pool.Query(context.Background(), "SELECT user_id, user_name, first_name, created_at FROM bot_admins ORDER BY created_at ASC")
	if err != nil {
		sendHTMLMessage(bot, message.Chat.ID, "❌ Database error retrieving bot administrators.")
		return
	}
	defer rows.Close()

	var builder strings.Builder
	builder.WriteString("👑 <b>MiniMate Bot Administrators (Sudos):</b>\n\n")
	builder.WriteString(fmt.Sprintf("• <b>Primary Owner:</b> <code>%d</code> (@%s)\n\n", config.OwnerID, html.EscapeString(config.OwnerUsername)))

	count := 0
	for rows.Next() {
		var uid int64
		var uname, fname string
		var created time.Time
		if err := rows.Scan(&uid, &uname, &fname, &created); err == nil {
			count++
			userTag := fmt.Sprintf("@%s", uname)
			if uname == "" {
				userTag = fname
			}
			builder.WriteString(fmt.Sprintf("%d. <b>%s</b> (<code>%d</code>)\n   • Added: <code>%s</code>\n",
				count, html.EscapeString(userTag), uid, created.Format("2006-01-02 15:04")))
		}
	}

	if count == 0 {
		builder.WriteString("<i>No additional bot administrators appointed. Use <code>/addbotadmin</code> to add sudos.</i>")
	}

	sendHTMLMessage(bot, message.Chat.ID, builder.String())
}

// -------------------------
// ENHANCED VIP SUBSCRIPTIONS
// -------------------------

// HandleSetVIP grants VIP status to a chat
func HandleSetVIP(bot *tgbotapi.BotAPI, message *tgbotapi.Message, args string) {
	fromID := int64(0)
	if message.From != nil {
		fromID = message.From.ID
	}
	if !isBotAdmin(fromID) {
		sendHTMLMessage(bot, message.Chat.ID, "❌ This command is restricted to <b>Bot Administrators</b>.")
		return
	}

	parts := strings.Fields(args)
	if len(parts) == 0 {
		sendHTMLMessage(bot, message.Chat.ID, `💎 <b>Usage:</b> <code>/setvip &lt;duration&gt; [chat_id]</code>

<b>Examples:</b>
• <code>/setvip 30d</code> <i>(In current group)</i>
• <code>/setvip 1y -1001234567890</code> <i>(From DM for specific group)</i>
• <code>/setvip lifetime</code> <i>(Permanent VIP)</i>
• <code>/setvip 7d</code> <i>(1-week trial)</i>`)
		return
	}

	durStr := strings.ToLower(parts[0])
	targetChatID := message.Chat.ID

	if len(parts) >= 2 {
		id, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			sendHTMLMessage(bot, message.Chat.ID, "❌ Invalid Chat ID. Must be a valid numeric ID (e.g. <code>-1001234567890</code>).")
			return
		}
		targetChatID = id
	}

	var expiresAt *time.Time
	var durationLabel string

	if durStr == "lifetime" || durStr == "permanent" || durStr == "inf" || durStr == "forever" {
		expiresAt = nil
		durationLabel = "Lifetime (Permanent)"
	} else {
		unixExp, err := parseDuration(durStr)
		if err != nil {
			sendHTMLMessage(bot, message.Chat.ID, fmt.Sprintf("❌ Invalid duration: <i>%s</i> (Use e.g. <code>30d</code>, <code>12h</code>, <code>1y</code>, or <code>lifetime</code>)", err.Error()))
			return
		}
		t := time.Unix(unixExp, 0)
		expiresAt = &t
		durationLabel = fmt.Sprintf("Expires on %s", t.Format("02 Jan 2006 15:04 UTC"))
	}

	query := `
		INSERT INTO chat_subscriptions (chat_id, is_vip, expires_at, plan_name)
		VALUES ($1, true, $2, 'VIP')
		ON CONFLICT (chat_id) DO UPDATE SET
			is_vip = true,
			expires_at = EXCLUDED.expires_at,
			plan_name = 'VIP';
	`
	_, err := database.Pool.Exec(context.Background(), query, targetChatID, expiresAt)
	if err != nil {
		sendHTMLMessage(bot, message.Chat.ID, "❌ Database error updating VIP subscription.")
		return
	}

	text := fmt.Sprintf(`💎 <b>VIP Subscription Activated!</b>

<blockquote expandable>👑 <b>Target Chat:</b> <code>%d</code>
⏳ <b>Duration:</b> <code>%s</code>
🛡️ <b>Status:</b> Active VIP Shield Enabled
✨ <b>Perks:</b> 0ms Latency • Anti-Raid • Animated Custom Emojis • Unlimited Storage</blockquote>`,
		targetChatID, html.EscapeString(durationLabel))

	sendHTMLMessage(bot, message.Chat.ID, text)

	// If activated remotely from DM, also notify the group if possible
	if targetChatID != message.Chat.ID {
		remoteNotice := fmt.Sprintf(`💎 <b>Congratulations! This chat has been upgraded to MiniMate VIP!</b>

<blockquote expandable>⏳ <b>Duration:</b> <code>%s</code>
✨ All premium security shields and unlimited storage are now active!</blockquote>`, html.EscapeString(durationLabel))
		sendHTMLMessage(bot, targetChatID, remoteNotice)
	}
}

// HandleRmVIP revokes VIP status from a chat
func HandleRmVIP(bot *tgbotapi.BotAPI, message *tgbotapi.Message, args string) {
	fromID := int64(0)
	if message.From != nil {
		fromID = message.From.ID
	}
	if !isBotAdmin(fromID) {
		sendHTMLMessage(bot, message.Chat.ID, "❌ This command is restricted to <b>Bot Administrators</b>.")
		return
	}

	targetChatID := message.Chat.ID
	if strings.TrimSpace(args) != "" {
		id, err := strconv.ParseInt(strings.TrimSpace(args), 10, 64)
		if err != nil {
			sendHTMLMessage(bot, message.Chat.ID, "❌ Invalid Chat ID. Usage: <code>/rmvip [chat_id]</code>")
			return
		}
		targetChatID = id
	}

	query := `
		UPDATE chat_subscriptions
		SET is_vip = false, expires_at = NOW()
		WHERE chat_id = $1;
	`
	_, err := database.Pool.Exec(context.Background(), query, targetChatID)
	if err != nil {
		sendHTMLMessage(bot, message.Chat.ID, "❌ Database error revoking VIP subscription.")
		return
	}

	sendHTMLMessage(bot, message.Chat.ID, fmt.Sprintf("🗑️ VIP subscription revoked for chat <code>%d</code>.", targetChatID))
}

// HandleVIPStatus displays the VIP status card for a group
func HandleVIPStatus(bot *tgbotapi.BotAPI, message *tgbotapi.Message, args string) {
	targetChatID := message.Chat.ID
	if strings.TrimSpace(args) != "" && isBotAdmin(message.From.ID) {
		if id, err := strconv.ParseInt(strings.TrimSpace(args), 10, 64); err == nil {
			targetChatID = id
		}
	}

	var isVIP bool
	var expiresAt *time.Time
	var planName string

	err := database.Pool.QueryRow(context.Background(),
		"SELECT is_vip, expires_at, plan_name FROM chat_subscriptions WHERE chat_id = $1", targetChatID).
		Scan(&isVIP, &expiresAt, &planName)

	if err != nil || !isVIP || (expiresAt != nil && time.Now().After(*expiresAt)) {
		statusText := fmt.Sprintf(`🌸 <b>MiniMate Group Status: Standard (Free)</b>

<blockquote expandable>📊 <b>Chat ID:</b> <code>%d</code>
🛡️ <b>Plan:</b> Free Community Edition
⚡ <b>Speed:</b> Standard Polling
🎨 <b>Theme:</b> Unicode High-Definition Emojis</blockquote>

👑 <i>Upgrade to VIP for 0ms priority polling, PokeEmpire animated themes, and Anti-Raid shields! Use <code>/premium</code> to contact @%s.</i>`,
			targetChatID, html.EscapeString(config.OwnerUsername))
		sendHTMLMessage(bot, message.Chat.ID, statusText)
		return
	}

	var expiryText string
	if expiresAt == nil {
		expiryText = "♾️ Lifetime (Never expires)"
	} else {
		remaining := time.Until(*expiresAt).Round(time.Hour)
		days := int(remaining.Hours()) / 24
		hours := int(remaining.Hours()) % 24
		expiryText = fmt.Sprintf("%s (%d days, %d hours remaining)", expiresAt.Format("02 Jan 2006"), days, hours)
	}

	text := fmt.Sprintf(`💎 <b>MiniMate VIP Status: Active</b>

<blockquote expandable>👑 <b>Subscription:</b> %s
📊 <b>Chat ID:</b> <code>%d</code>
⏳ <b>Expiry:</b> %s
⚡ <b>Speed:</b> Ultra-Fast High-Priority Channel
🛡️ <b>Anti-Raid:</b> Active &amp; Guarded
🎨 <b>Theme:</b> PokeEmpire Animated Custom Emojis</blockquote>`,
		html.EscapeString(planName), targetChatID, html.EscapeString(expiryText))

	sendHTMLMessage(bot, message.Chat.ID, text)
}

// HandleVIPList lists all active VIP subscriptions
func HandleVIPList(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	fromID := int64(0)
	if message.From != nil {
		fromID = message.From.ID
	}
	if !isBotAdmin(fromID) {
		sendHTMLMessage(bot, message.Chat.ID, "❌ This command is restricted to <b>Bot Administrators</b>.")
		return
	}

	rows, err := database.Pool.Query(context.Background(), `
		SELECT s.chat_id, COALESCE(c.title, 'Private/Unknown'), s.expires_at 
		FROM chat_subscriptions s
		LEFT JOIN bot_chats c ON s.chat_id = c.chat_id
		WHERE s.is_vip = true AND (s.expires_at IS NULL OR s.expires_at > NOW())
		ORDER BY s.expires_at ASC NULLS LAST
	`)
	if err != nil {
		sendHTMLMessage(bot, message.Chat.ID, "❌ Database error retrieving VIP list.")
		return
	}
	defer rows.Close()

	var builder strings.Builder
	builder.WriteString("💎 <b>Active VIP Subscribed Chats:</b>\n\n")

	count := 0
	for rows.Next() {
		var cid int64
		var title string
		var expiresAt *time.Time
		if err := rows.Scan(&cid, &title, &expiresAt); err == nil {
			count++
			exp := "Lifetime"
			if expiresAt != nil {
				exp = expiresAt.Format("02 Jan 2006")
			}
			builder.WriteString(fmt.Sprintf("%d. <b>%s</b>\n   • <b>ID:</b> <code>%d</code> | <b>Expires:</b> <code>%s</code>\n",
				count, html.EscapeString(title), cid, exp))
		}
	}

	if count == 0 {
		builder.WriteString("<i>No active VIP subscriptions. Use <code>/setvip</code> to activate a chat.</i>")
	} else {
		builder.WriteString(fmt.Sprintf("\n📊 <i>Total Active VIP Chats: %d</i>", count))
	}

	sendHTMLMessage(bot, message.Chat.ID, builder.String())
}

// -------------------------
// LEAVE COMMAND
// -------------------------

// HandleLeave forces the bot to leave a chat (current chat, or remote chat ID from DM)
func HandleLeave(bot *tgbotapi.BotAPI, message *tgbotapi.Message, args string) {
	fromID := int64(0)
	if message.From != nil {
		fromID = message.From.ID
	}
	if !isBotAdmin(fromID) {
		sendHTMLMessage(bot, message.Chat.ID, "❌ This command is restricted to <b>Bot Administrators</b>.")
		return
	}

	targetChatID := message.Chat.ID

	// If in DM or specifying a remote chat ID
	if strings.TrimSpace(args) != "" {
		id, err := strconv.ParseInt(strings.TrimSpace(args), 10, 64)
		if err != nil {
			sendHTMLMessage(bot, message.Chat.ID, "❌ Invalid Chat ID. Usage: <code>/leave [chat_id]</code>")
			return
		}
		targetChatID = id
	} else if message.Chat.IsPrivate() {
		sendHTMLMessage(bot, message.Chat.ID, "❌ In Direct Messages, provide the Chat ID to leave: <code>/leave &lt;chat_id&gt;</code>\nUse <code>/spam</code> to view active chat IDs.")
		return
	}

	// 1. Send farewell message to the target chat
	farewell := tgbotapi.NewMessage(targetChatID, "👋 <b>MiniMate is leaving this chat.</b> Thank you for using MiniMate!")
	farewell.ParseMode = "HTML"
	SafeSend(bot, farewell)

	// 2. Request Telegram to leave the chat
	leaveConfig := tgbotapi.LeaveChatConfig{
		ChatID: targetChatID,
	}
	_, err := bot.Request(leaveConfig)
	if err != nil {
		log.Printf("[Leave] Error leaving chat %d: %v", targetChatID, err)
		sendHTMLMessage(bot, message.Chat.ID, fmt.Sprintf("❌ Failed to leave chat <code>%d</code>: <i>%s</i>", targetChatID, html.EscapeString(err.Error())))
		return
	}

	// 3. Mark chat inactive in database
	database.Pool.Exec(context.Background(), "UPDATE bot_chats SET is_active = false, updated_at = NOW() WHERE chat_id = $1", targetChatID)

	if targetChatID != message.Chat.ID {
		sendHTMLMessage(bot, message.Chat.ID, fmt.Sprintf("✅ MiniMate successfully left remote chat <code>%d</code>.", targetChatID))
	}
}

// -------------------------
// UNIVERSAL MULTI-FORMAT BROADCAST
// -------------------------

// HandleBroadcast sends text, stickers, photos, videos, audio, voice, docs to all registered groups
func HandleBroadcast(bot *tgbotapi.BotAPI, message *tgbotapi.Message, args string) {
	fromID := int64(0)
	if message.From != nil {
		fromID = message.From.ID
	}
	if !isBotAdmin(fromID) {
		sendHTMLMessage(bot, message.Chat.ID, "❌ This command is restricted to <b>Bot Administrators</b>.")
		return
	}

	hasReply := message.ReplyToMessage != nil
	textBroadcast := strings.TrimSpace(args)

	if !hasReply && textBroadcast == "" {
		sendHTMLMessage(bot, message.Chat.ID, `📢 <b>Universal Multi-Format Broadcast</b>

<b>Usage:</b>
1. <b>Reply Broadcast:</b> Reply to <b>ANY</b> message (photo, video, sticker, GIF, document, voice note, formatted text with links) with <code>/broadcast</code>.
2. <b>Text Broadcast:</b> Send <code>/broadcast &lt;message&gt;</code>.`)
		return
	}

	// Fetch all active chats
	rows, err := database.Pool.Query(context.Background(), "SELECT chat_id FROM bot_chats WHERE is_active = true")
	if err != nil {
		sendHTMLMessage(bot, message.Chat.ID, "❌ Database error retrieving broadcast target chats.")
		return
	}
	defer rows.Close()

	var targets []int64
	for rows.Next() {
		var cid int64
		if err := rows.Scan(&cid); err == nil {
			targets = append(targets, cid)
		}
	}

	total := len(targets)
	if total == 0 {
		sendHTMLMessage(bot, message.Chat.ID, "ℹ️ No registered active chats found to broadcast to.")
		return
	}

	statusMsg, _ := sendHTMLMessage(bot, message.Chat.ID, fmt.Sprintf("⏳ <b>Starting broadcast to %d chats...</b>", total))

	startTime := time.Now()
	sourceChatID := message.Chat.ID
	replyMsgID := 0
	if hasReply {
		replyMsgID = message.ReplyToMessage.MessageID
	}

	go func(targetList []int64, replyID int, text string, statusMessageID int, callerChatID int64) {
		successCount := 0
		failCount := 0

		for _, cid := range targetList {
			var sendErr error

			if replyID != 0 {
				// Native copyMessage handles all media types: video, photo, sticker, voice, doc, links with 100% fidelity
				copyConfig := tgbotapi.NewCopyMessage(cid, sourceChatID, replyID)
				_, sendErr = bot.Request(copyConfig)
			} else {
				msg := tgbotapi.NewMessage(cid, text)
				msg.ParseMode = "HTML"
				_, sendErr = SafeSend(bot, msg)
			}

			if sendErr != nil {
				failCount++
				errStr := strings.ToLower(sendErr.Error())
				if strings.Contains(errStr, "blocked") || strings.Contains(errStr, "kicked") || strings.Contains(errStr, "chat not found") || strings.Contains(errStr, "deactivated") {
					database.Pool.Exec(context.Background(), "UPDATE bot_chats SET is_active = false, updated_at = NOW() WHERE chat_id = $1", cid)
				}
			} else {
				successCount++
			}

			// Gentle 35ms pause to respect Telegram Bot API rate limits (30 msgs/sec max)
			time.Sleep(35 * time.Millisecond)
		}

		duration := time.Since(startTime).Round(time.Millisecond)

		reportText := fmt.Sprintf(`📢 <b>Broadcast Completed!</b>

<blockquote expandable>📊 <b>Delivery Statistics:</b>
• 🎯 <b>Total Targeted:</b> <code>%d chats</code>
• ✅ <b>Successfully Delivered:</b> <code>%d chats</code>
• ❌ <b>Failed / Inactive:</b> <code>%d chats</code>
• ⏱️ <b>Time Elapsed:</b> <code>%s</code></blockquote>`,
			total, successCount, failCount, duration.String())

		if statusMessageID != 0 {
			edit := tgbotapi.NewEditMessageText(callerChatID, statusMessageID, reportText)
			edit.ParseMode = "HTML"
			SafeSend(bot, edit)
		} else {
			sendHTMLMessage(bot, callerChatID, reportText)
		}
	}(targets, replyMsgID, textBroadcast, statusMsg.MessageID, message.Chat.ID)
}
