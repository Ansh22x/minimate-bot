package handlers

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// isAdmin checks if a given user has administrator privileges in the chat
func isAdmin(bot *tgbotapi.BotAPI, chatID int64, userID int64) bool {
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

// parseDuration parses strings like "10m", "2h", "1d" into a unix timestamp
func parseDuration(durationStr string) (int64, error) {
	if len(durationStr) < 2 {
		return 0, fmt.Errorf("invalid duration format")
	}

	unit := durationStr[len(durationStr)-1:]
	valStr := durationStr[:len(durationStr)-1]

	val, err := strconv.ParseInt(valStr, 10, 64)
	if err != nil {
		return 0, err
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
		return 0, fmt.Errorf("unknown time unit: %s", unit)
	}

	return time.Now().Add(d).Unix(), nil
}

// -------------------------
// BANNING & KICKING
// -------------------------

// HandleBan permanently bans a user
func HandleBan(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	if !isAdmin(bot, message.Chat.ID, message.From.ID) {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ You must be an administrator to use this command."))
		return
	}
	if message.ReplyToMessage == nil {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ Reply to a user's message to ban them."))
		return
	}

	target := message.ReplyToMessage.From
	banConfig := tgbotapi.BanChatMemberConfig{
		ChatMemberConfig: tgbotapi.ChatMemberConfig{
			ChatID: message.Chat.ID,
			UserID: target.ID,
		},
	}

	_, err := bot.Request(banConfig)
	if err != nil {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ Failed to ban. Ensure I have admin permissions."))
		log.Printf("Ban failed: %v", err)
		return
	}

	bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("🚫 <b>%s</b> has been banned.", target.FirstName)))
}

// HandleTBan temporarily bans a user (e.g., /tban 2d)
func HandleTBan(bot *tgbotapi.BotAPI, message *tgbotapi.Message, args string) {
	if !isAdmin(bot, message.Chat.ID, message.From.ID) {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ You must be an administrator to use this command."))
		return
	}
	if message.ReplyToMessage == nil || args == "" {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ Usage: Reply to a message with <code>/tban &lt;time&gt;</code> (e.g. <code>/tban 2h</code>, <code>/tban 1d</code>)"))
		return
	}

	untilDate, err := parseDuration(strings.TrimSpace(args))
	if err != nil {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ Invalid time format. Use m (minutes), h (hours), or d (days)."))
		return
	}

	target := message.ReplyToMessage.From
	banConfig := tgbotapi.BanChatMemberConfig{
		ChatMemberConfig: tgbotapi.ChatMemberConfig{
			ChatID: message.Chat.ID,
			UserID: target.ID,
		},
		UntilDate: untilDate,
	}

	_, err = bot.Request(banConfig)
	if err != nil {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ Failed to temporary ban user."))
		return
	}

	bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("⏳ <b>%s</b> has been temporarily banned for %s.", target.FirstName, args)))
}

// HandleUnban unbans a user
func HandleUnban(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	if !isAdmin(bot, message.Chat.ID, message.From.ID) {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ You must be an administrator to use this command."))
		return
	}
	if message.ReplyToMessage == nil {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ Reply to a user's message to unban them."))
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
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ Failed to unban user."))
		return
	}

	bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("✅ <b>%s</b> has been unbanned.", target.FirstName)))
}

// HandleKick kicks a user from the chat (bans then unbans)
func HandleKick(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	if !isAdmin(bot, message.Chat.ID, message.From.ID) {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ You must be an administrator to use this command."))
		return
	}
	if message.ReplyToMessage == nil {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ Reply to a user's message to kick them."))
		return
	}

	target := message.ReplyToMessage.From
	banConfig := tgbotapi.BanChatMemberConfig{
		ChatMemberConfig: tgbotapi.ChatMemberConfig{ChatID: message.Chat.ID, UserID: target.ID},
	}
	bot.Request(banConfig)

	unbanConfig := tgbotapi.UnbanChatMemberConfig{
		ChatMemberConfig: tgbotapi.ChatMemberConfig{ChatID: message.Chat.ID, UserID: target.ID},
		OnlyIfBanned:     true,
	}
	bot.Request(unbanConfig)

	bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("👢 <b>%s</b> has been kicked from the group.", target.FirstName)))
}

// -------------------------
// MUTING & UNMUTING
// -------------------------

// HandleMute permanently restricts a user from sending messages
func HandleMute(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	if !isAdmin(bot, message.Chat.ID, message.From.ID) {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ You must be an administrator to use this command."))
		return
	}
	if message.ReplyToMessage == nil {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ Reply to a user's message to mute them."))
		return
	}

	target := message.ReplyToMessage.From
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
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ Failed to mute user."))
		return
	}

	bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("🤐 <b>%s</b> has been muted.", target.FirstName)))
}

// HandleTMute temporarily restricts a user
func HandleTMute(bot *tgbotapi.BotAPI, message *tgbotapi.Message, args string) {
	if !isAdmin(bot, message.Chat.ID, message.From.ID) {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ You must be an administrator to use this command."))
		return
	}
	if message.ReplyToMessage == nil || args == "" {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ Usage: Reply with <code>/tmute &lt;time&gt;</code> (e.g. <code>/tmute 30m</code>, <code>/tmute 2h</code>)"))
		return
	}

	untilDate, err := parseDuration(strings.TrimSpace(args))
	if err != nil {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ Invalid time format. Use m (minutes), h (hours), or d (days)."))
		return
	}

	target := message.ReplyToMessage.From
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
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ Failed to temporary mute user."))
		return
	}

	bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("🤐 <b>%s</b> has been muted for %s.", target.FirstName, args)))
}

// HandleUnmute restores a user's permissions
func HandleUnmute(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	if !isAdmin(bot, message.Chat.ID, message.From.ID) {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ You must be an administrator to use this command."))
		return
	}
	if message.ReplyToMessage == nil {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ Reply to a user's message to unmute them."))
		return
	}

	target := message.ReplyToMessage.From
	perms := tgbotapi.ChatPermissions{
		CanSendMessages:       true,
		CanSendMediaMessages:  true, // Covers photos, videos, audio, etc. in v5
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
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ Failed to unmute user."))
		return
	}

	bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("🔊 <b>%s</b> has been unmuted.", target.FirstName)))
}

// -------------------------
// PROMOTIONS & ADMIN ROLES
// -------------------------

// HandlePromote promotes a user with standard admin privileges
func HandlePromote(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	if !isAdmin(bot, message.Chat.ID, message.From.ID) {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ You must be an administrator to use this command."))
		return
	}
	if message.ReplyToMessage == nil {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ Reply to a user's message to promote them."))
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
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ Failed to promote user."))
		return
	}

	bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("⭐ <b>%s</b> has been promoted to Admin!", target.FirstName)))
}

// HandleDemote strips admin privileges from a user
func HandleDemote(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	if !isAdmin(bot, message.Chat.ID, message.From.ID) {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ You must be an administrator to use this command."))
		return
	}
	if message.ReplyToMessage == nil {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ Reply to an admin to demote them."))
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
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ Failed to demote user."))
		return
	}

	bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("🔽 <b>%s</b> has been demoted.", target.FirstName)))
}

// HandleAdminList lists all group admins
func HandleAdminList(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	config := tgbotapi.ChatAdministratorsConfig{
		ChatConfig: tgbotapi.ChatConfig{ChatID: message.Chat.ID},
	}

	admins, err := bot.GetChatAdministrators(config)
	if err != nil {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ Failed to fetch admin list."))
		return
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("🛡️ <b>Admins in %s:</b>\n\n", message.Chat.Title))

	for _, admin := range admins {
		title := admin.CustomTitle
		if title == "" {
			if admin.Status == "creator" {
				title = "Creator"
			} else {
				title = "Admin"
			}
		}
		builder.WriteString(fmt.Sprintf("• %s (<code>%s</code>)\n", admin.User.FirstName, title))
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, builder.String())
	msg.ParseMode = "HTML"
	bot.Send(msg)
}

// HandleInviteLink exports the chat invite link using the built-in v5 method
func HandleInviteLink(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	if !isAdmin(bot, message.Chat.ID, message.From.ID) {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ You must be an administrator to use this command."))
		return
	}

	config := tgbotapi.ChatInviteLinkConfig{
		ChatConfig: tgbotapi.ChatConfig{ChatID: message.Chat.ID},
	}
	
	link, err := bot.GetInviteLink(config)
	if err != nil {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ Failed to export invite link."))
		return
	}

	bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("🔗 <b>Group Invite Link:</b>\n%s", link)))
}

// HandleTitle sets a custom admin title
func HandleTitle(bot *tgbotapi.BotAPI, message *tgbotapi.Message, args string) {
	// The v5 wrapper does not natively support custom titles yet, so we will return a placeholder
	bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ Custom admin titles are pending wrapper support."))
}

// -------------------------
// CLEANUP & PINS
// -------------------------

// HandleDel deletes a single replied message
func HandleDel(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	if !isAdmin(bot, message.Chat.ID, message.From.ID) {
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
	if !isAdmin(bot, message.Chat.ID, message.From.ID) {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ Only admins can purge messages."))
		return
	}
	if message.ReplyToMessage == nil {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ Reply to the message where you want the purge to start."))
		return
	}

	startID := message.ReplyToMessage.MessageID
	endID := message.MessageID

	go func(chatID int64, start, end int) {
		count := 0
		for id := start; id <= end; id++ {
			del := tgbotapi.NewDeleteMessage(chatID, id)
			_, err := bot.Request(del)
			if err == nil {
				count++
			}
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
	if !isAdmin(bot, message.Chat.ID, message.From.ID) {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ You must be an administrator to pin/unpin messages."))
		return
	}

	switch cmd {
	case "pin":
		if message.ReplyToMessage == nil {
			bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ Reply to a message to pin it."))
			return
		}
		pinConfig := tgbotapi.PinChatMessageConfig{
			ChatID:              message.Chat.ID,
			MessageID:           message.ReplyToMessage.MessageID,
			DisableNotification: false,
		}
		_, err := bot.Request(pinConfig)
		if err == nil {
			bot.Send(tgbotapi.NewMessage(message.Chat.ID, "📌 Message pinned!"))
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
			bot.Send(tgbotapi.NewMessage(message.Chat.ID, "📌 Message unpinned."))
		}

	case "unpinall":
		unpinAllConfig := tgbotapi.UnpinAllChatMessagesConfig{
			ChatID: message.Chat.ID,
		}
		_, err := bot.Request(unpinAllConfig)
		if err == nil {
			bot.Send(tgbotapi.NewMessage(message.Chat.ID, "📌 All messages have been unpinned."))
		}
	}
}
