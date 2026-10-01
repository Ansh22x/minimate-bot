// Package services: moderation service — single source of truth for all mod actions.
package services

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"minimate-bot/database"
	"minimate-bot/models"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// ModerationResult represents the outcome of a moderation action.
type ModerationResult struct {
	Success    bool
	ModLogID   int64
	Message    string
	UndoToken  string // callback data for undo button
}

// ==================== BAN ====================

// BanUser permanently or temporarily bans a user.
// duration = 0 means permanent ban.
func BanUser(bot *tgbotapi.BotAPI, chatID, userID, modID int64, reason string, duration time.Duration) ModerationResult {
	var untilDate int64
	action := models.ActionBan
	expiresAt := (*time.Time)(nil)

	if duration > 0 {
		t := time.Now().Add(duration)
		expiresAt = &t
		untilDate = t.Unix()
		action = models.ActionTempBan
	}

	banConfig := tgbotapi.BanChatMemberConfig{
		ChatMemberConfig: tgbotapi.ChatMemberConfig{
			ChatID: chatID,
			UserID: userID,
		},
		UntilDate:      untilDate,
		RevokeMessages: false,
	}
	if _, err := bot.Request(banConfig); err != nil {
		return ModerationResult{false, 0, fmt.Sprintf("❌ Ban failed: %s", cleanAPIError(err.Error())), ""}
	}

	logID := logAction(chatID, userID, modID, action, reason, expiresAt, nil)
	if duration > 0 && logID > 0 {
		schedTempAction(chatID, userID, models.TempActionBan, logID, *expiresAt)
	}

	msg := formatModMessage("🔨", "Banned", chatID, userID, modID, reason, duration)
	return ModerationResult{true, logID, msg, fmt.Sprintf("undo_ban:%d:%d:%d", chatID, userID, logID)}
}

// UnbanUser removes a ban from a user.
func UnbanUser(bot *tgbotapi.BotAPI, chatID, userID, modID int64, reason string) ModerationResult {
	unbanConfig := tgbotapi.UnbanChatMemberConfig{
		ChatMemberConfig: tgbotapi.ChatMemberConfig{
			ChatID: chatID,
			UserID: userID,
		},
		OnlyIfBanned: true,
	}
	if _, err := bot.Request(unbanConfig); err != nil {
		return ModerationResult{false, 0, fmt.Sprintf("❌ Unban failed: %s", cleanAPIError(err.Error())), ""}
	}
	logID := logAction(chatID, userID, modID, models.ActionUnban, reason, nil, nil)
	return ModerationResult{true, logID, fmt.Sprintf("✅ <b>Unbanned</b> user <code>%d</code>.", userID), ""}
}

// ==================== KICK ====================

// KickUser kicks a user (ban then immediately unban).
func KickUser(bot *tgbotapi.BotAPI, chatID, userID, modID int64, reason string) ModerationResult {
	// Ban first
	banConfig := tgbotapi.BanChatMemberConfig{
		ChatMemberConfig: tgbotapi.ChatMemberConfig{ChatID: chatID, UserID: userID},
	}
	if _, err := bot.Request(banConfig); err != nil {
		return ModerationResult{false, 0, fmt.Sprintf("❌ Kick failed: %s", cleanAPIError(err.Error())), ""}
	}
	// Then unban to allow rejoin
	bot.Request(tgbotapi.UnbanChatMemberConfig{
		ChatMemberConfig: tgbotapi.ChatMemberConfig{ChatID: chatID, UserID: userID},
		OnlyIfBanned:     true,
	})

	logID := logAction(chatID, userID, modID, models.ActionKick, reason, nil, nil)
	msg := formatModMessage("👢", "Kicked", chatID, userID, modID, reason, 0)
	return ModerationResult{true, logID, msg, ""}
}

// ==================== MUTE ====================

// MuteUser restricts a user from sending messages.
// duration = 0 means permanent mute.
func MuteUser(bot *tgbotapi.BotAPI, chatID, userID, modID int64, reason string, duration time.Duration) ModerationResult {
	var untilDate int64
	action := models.ActionMute
	expiresAt := (*time.Time)(nil)

	if duration > 0 {
		t := time.Now().Add(duration)
		expiresAt = &t
		untilDate = t.Unix()
		action = models.ActionTempMute
	}

	restrictConfig := tgbotapi.RestrictChatMemberConfig{
		ChatMemberConfig: tgbotapi.ChatMemberConfig{ChatID: chatID, UserID: userID},
		UntilDate:        untilDate,
		Permissions:      &tgbotapi.ChatPermissions{},
	}
	if _, err := bot.Request(restrictConfig); err != nil {
		return ModerationResult{false, 0, fmt.Sprintf("❌ Mute failed: %s", cleanAPIError(err.Error())), ""}
	}

	logID := logAction(chatID, userID, modID, action, reason, expiresAt, nil)
	if duration > 0 && logID > 0 {
		schedTempAction(chatID, userID, models.TempActionMute, logID, *expiresAt)
	}

	msg := formatModMessage("🔇", "Muted", chatID, userID, modID, reason, duration)
	return ModerationResult{true, logID, msg, fmt.Sprintf("undo_mute:%d:%d:%d", chatID, userID, logID)}
}

// UnmuteUser restores full send permissions to a user.
func UnmuteUser(bot *tgbotapi.BotAPI, chatID, userID, modID int64, reason string) ModerationResult {
	restrictConfig := tgbotapi.RestrictChatMemberConfig{
		ChatMemberConfig: tgbotapi.ChatMemberConfig{ChatID: chatID, UserID: userID},
		UntilDate:        0,
		Permissions: &tgbotapi.ChatPermissions{
			CanSendMessages:       true,
			CanSendMediaMessages:  true,
			CanSendPolls:          true,
			CanSendOtherMessages:  true,
			CanAddWebPagePreviews: true,
			CanChangeInfo:         false,
			CanInviteUsers:        true,
			CanPinMessages:        false,
		},
	}
	if _, err := bot.Request(restrictConfig); err != nil {
		return ModerationResult{false, 0, fmt.Sprintf("❌ Unmute failed: %s", cleanAPIError(err.Error())), ""}
	}
	logID := logAction(chatID, userID, modID, models.ActionUnmute, reason, nil, nil)
	return ModerationResult{true, logID, fmt.Sprintf("🔊 <b>Unmuted</b> user <code>%d</code>.", userID), ""}
}

// ==================== WARN ====================

// WarnUser adds a warning to a user and checks thresholds for automatic punishment.
// Returns the updated warn count and any auto-action taken.
func WarnUser(bot *tgbotapi.BotAPI, chatID, userID, modID int64, reason string) (warnCount int, autoAction string, modResult ModerationResult) {
	settings := GetGroupSettings(chatID)

	// Insert warn entry
	var warnID int64
	err := database.Pool.QueryRow(context.Background(), `
		INSERT INTO warn_entries (chat_id, user_id, mod_id, reason, active, created_at)
		VALUES ($1, $2, $3, $4, true, NOW())
		RETURNING id
	`, chatID, userID, modID, reason).Scan(&warnID)
	if err != nil {
		return 0, "", ModerationResult{false, 0, "❌ Database error issuing warning.", ""}
	}

	// Count active warns
	var count int
	database.Pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM warn_entries
		WHERE chat_id = $1 AND user_id = $2 AND active = true
	`, chatID, userID).Scan(&count)

	// Update warn_count in group_members
	database.Pool.Exec(context.Background(), `
		INSERT INTO group_members (chat_id, user_id, warn_count)
		VALUES ($1, $2, 1)
		ON CONFLICT (chat_id, user_id) DO UPDATE SET warn_count = group_members.warn_count + 1
	`, chatID, userID)

	logID := logAction(chatID, userID, modID, models.ActionWarn, reason, nil, map[string]interface{}{
		"warn_id":    warnID,
		"warn_count": count,
	})

	// Check thresholds
	var threshold string
	switch {
	case count >= 10:
		threshold = settings.WarnThreshold10
	case count >= 7:
		threshold = settings.WarnThreshold7
	case count >= 5:
		threshold = settings.WarnThreshold5
	case count >= 3:
		threshold = settings.WarnThreshold3
	}

	if threshold != "" {
		parts := strings.SplitN(threshold, ":", 2)
		act := parts[0]
		var dur time.Duration
		if len(parts) == 2 {
			secs, _ := strconv.Atoi(parts[1])
			dur = time.Duration(secs) * time.Second
		}

		switch act {
		case "mute":
			result := MuteUser(bot, chatID, userID, bot.Self.ID, fmt.Sprintf("Auto: reached %d warnings", count), dur)
			return count, fmt.Sprintf("mute:%v", dur), result
		case "tempban":
			result := BanUser(bot, chatID, userID, bot.Self.ID, fmt.Sprintf("Auto: reached %d warnings", count), dur)
			return count, fmt.Sprintf("tempban:%v", dur), result
		case "ban":
			result := BanUser(bot, chatID, userID, bot.Self.ID, fmt.Sprintf("Auto: reached %d warnings", count), 0)
			return count, "ban", result
		}
	}

	_ = logID
	msg := fmt.Sprintf("⚠️ <b>Warning issued</b>\n\n<b>Warnings:</b> %d/%d\n<b>Reason:</b> %s",
		count, settings.WarnLimit, htmlEscape(reason))
	return count, "", ModerationResult{true, logID, msg, fmt.Sprintf("undo_warn:%d:%d:%d", chatID, userID, warnID)}
}

// RemoveWarn removes one active warning from a user.
func RemoveWarn(chatID, userID, modID int64) (remaining int, err error) {
	// Deactivate the most recent active warn
	var warnID int64
	err = database.Pool.QueryRow(context.Background(), `
		UPDATE warn_entries SET active = false
		WHERE id = (
			SELECT id FROM warn_entries
			WHERE chat_id = $1 AND user_id = $2 AND active = true
			ORDER BY created_at DESC LIMIT 1
		)
		RETURNING id
	`, chatID, userID).Scan(&warnID)
	if err != nil {
		return 0, fmt.Errorf("no active warning found")
	}

	database.Pool.Exec(context.Background(), `
		UPDATE group_members SET warn_count = GREATEST(0, warn_count - 1)
		WHERE chat_id = $1 AND user_id = $2
	`, chatID, userID)

	database.Pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM warn_entries WHERE chat_id = $1 AND user_id = $2 AND active = true
	`, chatID, userID).Scan(&remaining)

	logAction(chatID, userID, modID, models.ActionUnwarn, "Removed one warning", nil, map[string]interface{}{
		"warn_id": warnID,
	})
	return remaining, nil
}

// ResetWarns clears all warnings for a user.
func ResetWarns(chatID, userID, modID int64) error {
	_, err := database.Pool.Exec(context.Background(), `
		UPDATE warn_entries SET active = false
		WHERE chat_id = $1 AND user_id = $2 AND active = true
	`, chatID, userID)
	if err != nil {
		return err
	}
	database.Pool.Exec(context.Background(),
		"UPDATE group_members SET warn_count = 0 WHERE chat_id = $1 AND user_id = $2", chatID, userID)
	logAction(chatID, userID, modID, models.ActionResetWarns, "Reset all warnings", nil, nil)
	return nil
}

// GetWarnCount returns the current active warning count for a user.
func GetWarnCount(chatID, userID int64) int {
	var count int
	database.Pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM warn_entries WHERE chat_id = $1 AND user_id = $2 AND active = true
	`, chatID, userID).Scan(&count)
	return count
}

// ==================== UNDO ====================

// UndoAction reverses a recent moderation action.
func UndoAction(bot *tgbotapi.BotAPI, logID int64, undoneBy int64) ModerationResult {
	var chatID, userID int64
	var action string
	err := database.Pool.QueryRow(context.Background(), `
		SELECT chat_id, user_id, action FROM moderation_logs WHERE id = $1 AND undone = false
	`, logID).Scan(&chatID, &userID, &action)
	if err != nil {
		return ModerationResult{false, 0, "❌ Action not found or already undone.", ""}
	}

	var result ModerationResult
	switch action {
	case models.ActionBan, models.ActionTempBan:
		result = UnbanUser(bot, chatID, userID, undoneBy, "Undo ban")
	case models.ActionMute, models.ActionTempMute:
		result = UnmuteUser(bot, chatID, userID, undoneBy, "Undo mute")
	case models.ActionWarn:
		remaining, err2 := RemoveWarn(chatID, userID, undoneBy)
		if err2 != nil {
			return ModerationResult{false, 0, "❌ Warning already removed.", ""}
		}
		result = ModerationResult{true, 0, fmt.Sprintf("↩️ Warn undone. Remaining: %d", remaining), ""}
	default:
		return ModerationResult{false, 0, "❌ This action type cannot be undone.", ""}
	}

	if result.Success {
		database.Pool.Exec(context.Background(), `
			UPDATE moderation_logs SET undone = true, undone_by = $1, undone_at = NOW()
			WHERE id = $2
		`, undoneBy, logID)
		// Cancel any pending temp action
		database.Pool.Exec(context.Background(),
			"UPDATE temp_actions SET done = true WHERE log_id = $1", logID)
	}
	return result
}

// ==================== LOGS ====================

// GetModLogs returns paginated moderation logs for a user in a chat.
func GetModLogs(chatID, userID int64, page, pageSize int) ([]models.ModLog, int, error) {
	offset := (page - 1) * pageSize
	query := `
		SELECT id, chat_id, user_id, mod_id, action, COALESCE(reason,''), extra, created_at,
		       expires_at, undone
		FROM moderation_logs
		WHERE chat_id = $1
	`
	args := []interface{}{chatID}
	if userID > 0 {
		query += " AND user_id = $2"
		args = append(args, userID)
		query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT %d OFFSET %d", pageSize, offset)
	} else {
		query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT %d OFFSET %d", pageSize, offset)
	}

	rows, err := database.Pool.Query(context.Background(), query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []models.ModLog
	for rows.Next() {
		var ml models.ModLog
		var extraJSON []byte
		if err := rows.Scan(&ml.ID, &ml.ChatID, &ml.UserID, &ml.ModID, &ml.Action,
			&ml.Reason, &extraJSON, &ml.CreatedAt, &ml.ExpiresAt, &ml.Undone); err == nil {
			logs = append(logs, ml)
		}
	}

	// Total count
	countQuery := "SELECT COUNT(*) FROM moderation_logs WHERE chat_id = $1"
	countArgs := []interface{}{chatID}
	if userID > 0 {
		countQuery += " AND user_id = $2"
		countArgs = append(countArgs, userID)
	}
	var total int
	database.Pool.QueryRow(context.Background(), countQuery, countArgs...).Scan(&total)

	return logs, total, nil
}

// ==================== INTERNAL HELPERS ====================

// logAction records a moderation action to the database.
func logAction(chatID, userID, modID int64, action, reason string, expiresAt *time.Time, extra map[string]interface{}) int64 {
	var extraJSON []byte
	if extra != nil {
		import_json, _ := jsonMarshal(extra)
		extraJSON = import_json
	} else {
		extraJSON = []byte("{}")
	}

	var id int64
	err := database.Pool.QueryRow(context.Background(), `
		INSERT INTO moderation_logs (chat_id, user_id, mod_id, action, reason, extra, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`, chatID, userID, modID, action, reason, extraJSON, expiresAt).Scan(&id)
	if err != nil {
		log.Printf("⚠️ logAction error: %v", err)
	}
	return id
}

// schedTempAction creates a temp_action record for background worker processing.
func schedTempAction(chatID, userID int64, actionType string, logID int64, expiresAt time.Time) {
	_, err := database.Pool.Exec(context.Background(), `
		INSERT INTO temp_actions (chat_id, user_id, action_type, log_id, expires_at)
		VALUES ($1, $2, $3, $4, $5)
	`, chatID, userID, actionType, logID, expiresAt)
	if err != nil {
		log.Printf("⚠️ schedTempAction error: %v", err)
	}
}

// formatModMessage formats a moderation notification message.
func formatModMessage(emoji, actionName string, chatID, userID, modID int64, reason string, duration time.Duration) string {
	durationStr := "permanent"
	if duration > 0 {
		durationStr = formatDuration(duration)
	}

	msg := fmt.Sprintf(`%s <b>%s</b>

<b>User:</b> <code>%d</code>
<b>By:</b> <code>%d</code>
<b>Duration:</b> %s`,
		emoji, actionName, userID, modID, durationStr)

	if reason != "" {
		msg += fmt.Sprintf("\n<b>Reason:</b> %s", htmlEscape(reason))
	}
	return msg
}

// formatDuration returns a human-readable duration string.
func formatDuration(d time.Duration) string {
	if d == 0 {
		return "permanent"
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 24 {
		days := h / 24
		return fmt.Sprintf("%dd", days)
	}
	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

func cleanAPIError(e string) string {
	// Strip "Bad Request: " prefix from Telegram API errors
	e = strings.TrimPrefix(e, "Bad Request: ")
	e = strings.TrimPrefix(e, "Forbidden: ")
	return e
}

func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func jsonMarshal(v interface{}) ([]byte, error) {
	// Use standard library via import alias to avoid conflict
	return marshalJSON(v)
}
