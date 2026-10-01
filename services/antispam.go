// Package services: Anti-Spam engine with multi-layer detection and configurable thresholds.
package services

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"minimate-bot/database"
	"minimate-bot/workers"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type userMessageRecord struct {
	Timestamp time.Time
	MessageID int
	Text      string
	IsSticker bool
	IsMedia   bool
	IsCommand bool
}

var (
	// chatID -> userID -> list of recent message records
	spamTracker   = make(map[int64]map[int64][]userMessageRecord)
	spamTrackerMu sync.Mutex
)

// CheckSpam inspects an incoming message against all active anti-spam rules.
// Returns true if the message was detected as spam and handled (caller should stop processing).
func CheckSpam(bot *tgbotapi.BotAPI, msg *tgbotapi.Message) bool {
	if msg == nil || msg.Chat == nil || msg.Chat.IsPrivate() {
		return false
	}

	chatID := msg.Chat.ID
	fromID := int64(0)
	if msg.From != nil {
		fromID = msg.From.ID
		if msg.From.IsBot {
			return false
		}
	} else if msg.SenderChat != nil {
		fromID = msg.SenderChat.ID
	}

	if fromID == 0 {
		return false
	}

	// Administrators and bot admins are immune to anti-spam checks
	if IsAdminOrRole(bot, chatID, fromID) {
		return false
	}

	settings := GetGroupSettings(chatID)
	if !settings.AntiSpamEnabled {
		return false
	}

	now := time.Now()
	text := msg.Text
	if text == "" {
		text = msg.Caption
	}

	isSticker := msg.Sticker != nil
	isMedia := len(msg.Photo) > 0 || msg.Video != nil || msg.Audio != nil ||
		msg.Voice != nil || msg.Document != nil || msg.VideoNote != nil || msg.Animation != nil
	isCommand := msg.IsCommand()

	spamTrackerMu.Lock()
	if spamTracker[chatID] == nil {
		spamTracker[chatID] = make(map[int64][]userMessageRecord)
	}

	// Clean records older than 60 seconds
	cutoff := now.Add(-60 * time.Second)
	var activeRecords []userMessageRecord
	for _, rec := range spamTracker[chatID][fromID] {
		if rec.Timestamp.After(cutoff) {
			activeRecords = append(activeRecords, rec)
		}
	}

	currentRecord := userMessageRecord{
		Timestamp: now,
		MessageID: msg.MessageID,
		Text:      text,
		IsSticker: isSticker,
		IsMedia:   isMedia,
		IsCommand: isCommand,
	}
	activeRecords = append(activeRecords, currentRecord)
	spamTracker[chatID][fromID] = activeRecords
	recordsSnapshot := make([]userMessageRecord, len(activeRecords))
	copy(recordsSnapshot, activeRecords)
	spamTrackerMu.Unlock()

	// 1. Message Flooding Check (e.g. 5 messages in 3 seconds)
	floodWindow := time.Duration(settings.FloodWindow) * time.Second
	if floodWindow <= 0 {
		floodWindow = 3 * time.Second
	}
	floodLimit := settings.FloodLimit
	if floodLimit <= 0 {
		floodLimit = 5
	}

	floodCount := 0
	var floodMsgIDs []int
	floodCutoff := now.Add(-floodWindow)
	for _, rec := range recordsSnapshot {
		if rec.Timestamp.After(floodCutoff) {
			floodCount++
			floodMsgIDs = append(floodMsgIDs, rec.MessageID)
		}
	}

	if floodCount >= floodLimit {
		handleSpamViolation(bot, chatID, fromID, floodMsgIDs, "Flood (Too Many Messages)", settings.FloodMuteDuration)
		return true
	}

	// 2. Repeated Identical Message Spam (e.g. same text 3 times)
	if text != "" {
		repeatLimit := settings.RepeatLimit
		if repeatLimit <= 0 {
			repeatLimit = 3
		}
		repeatCount := 0
		var repeatMsgIDs []int
		tenSecCutoff := now.Add(-15 * time.Second)
		cleanText := strings.TrimSpace(strings.ToLower(text))

		for _, rec := range recordsSnapshot {
			if rec.Timestamp.After(tenSecCutoff) && strings.TrimSpace(strings.ToLower(rec.Text)) == cleanText {
				repeatCount++
				repeatMsgIDs = append(repeatMsgIDs, rec.MessageID)
			}
		}

		if repeatCount >= repeatLimit {
			handleSpamViolation(bot, chatID, fromID, repeatMsgIDs, "Repeated Message Spam", settings.FloodMuteDuration)
			return true
		}
	}

	// 3. Excessive Emoji Spam
	if text != "" {
		emojiLimit := settings.EmojiLimit
		if emojiLimit <= 0 {
			emojiLimit = 15
		}
		emojiCount := countEmojis(text)
		if emojiCount > emojiLimit {
			handleSingleSpamViolation(bot, chatID, fromID, msg.MessageID, fmt.Sprintf("Excessive Emojis (%d/%d)", emojiCount, emojiLimit))
			return true
		}
	}

	// 4. Excessive Sticker Spam (e.g. 5 stickers in 10 seconds)
	if isSticker {
		stickerLimit := settings.StickerLimit
		if stickerLimit <= 0 {
			stickerLimit = 5
		}
		stickerCount := 0
		var stickerMsgIDs []int
		tenSecCutoff := now.Add(-10 * time.Second)
		for _, rec := range recordsSnapshot {
			if rec.Timestamp.After(tenSecCutoff) && rec.IsSticker {
				stickerCount++
				stickerMsgIDs = append(stickerMsgIDs, rec.MessageID)
			}
		}
		if stickerCount >= stickerLimit {
			handleSpamViolation(bot, chatID, fromID, stickerMsgIDs, "Sticker Flooding", settings.FloodMuteDuration)
			return true
		}
	}

	// 5. Excessive Mention Spam (e.g. >5 mentions in a single message)
	if len(msg.Entities) > 0 {
		mentionLimit := settings.MentionLimit
		if mentionLimit <= 0 {
			mentionLimit = 5
		}
		mentionCount := 0
		for _, entity := range msg.Entities {
			if entity.Type == "mention" || entity.Type == "text_mention" {
				mentionCount++
			}
		}
		if mentionCount > mentionLimit {
			handleSingleSpamViolation(bot, chatID, fromID, msg.MessageID, fmt.Sprintf("Mass Mention Spam (%d mentions)", mentionCount))
			return true
		}
	}

	// 6. Command Spam (e.g. >4 commands in 4 seconds)
	if isCommand {
		cmdCount := 0
		var cmdMsgIDs []int
		fourSecCutoff := now.Add(-4 * time.Second)
		for _, rec := range recordsSnapshot {
			if rec.Timestamp.After(fourSecCutoff) && rec.IsCommand {
				cmdCount++
				cmdMsgIDs = append(cmdMsgIDs, rec.MessageID)
			}
		}
		if cmdCount >= 4 {
			handleSpamViolation(bot, chatID, fromID, cmdMsgIDs, "Command Flooding", 60)
			return true
		}
	}

	return false
}

// handleSpamViolation deletes offending messages, temporarily mutes the user, and logs the incident.
func handleSpamViolation(bot *tgbotapi.BotAPI, chatID, userID int64, msgIDs []int, reason string, muteSecs int) {
	// 1. Delete spam messages
	for _, id := range msgIDs {
		bot.Request(tgbotapi.DeleteMessageConfig{
			ChatID:    chatID,
			MessageID: id,
		})
	}

	// 2. Mute duration
	if muteSecs <= 0 {
		muteSecs = 300 // 5 minutes default
	}
	dur := time.Duration(muteSecs) * time.Second

	// 3. Mute the user
	MuteUser(bot, chatID, userID, bot.Self.ID, fmt.Sprintf("Anti-Spam: %s", reason), dur)

	// 4. Record incident in DB
	database.Pool.Exec(context.Background(), `
		INSERT INTO antispam_incidents (chat_id, user_id, spam_type, detail, action, created_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
	`, chatID, userID, reason, fmt.Sprintf("Deleted %d messages", len(msgIDs)), fmt.Sprintf("Muted for %ds", muteSecs))

	// 5. Increment analytics
	workers.IncrMetric(chatID, workers.MetricSpamBlocked)

	// 6. Send notification notice
	notice := tgbotapi.NewMessage(chatID, fmt.Sprintf("🛡️ <b>Anti-Spam Shield</b>\n\nUser <code>%d</code> was muted for <b>%s</b> due to <b>%s</b>.",
		userID, formatDuration(dur), htmlEscape(reason)))
	notice.ParseMode = "HTML"
	sentMsg, err := bot.Send(notice)
	if err == nil {
		// Auto-delete notification after 8 seconds to avoid chat clutter
		go func(cid int64, mid int) {
			time.Sleep(8 * time.Second)
			bot.Request(tgbotapi.DeleteMessageConfig{ChatID: cid, MessageID: mid})
		}(chatID, sentMsg.MessageID)
	}
}

// handleSingleSpamViolation deletes a single violating message and issues a warning.
func handleSingleSpamViolation(bot *tgbotapi.BotAPI, chatID, userID int64, msgID int, reason string) {
	bot.Request(tgbotapi.DeleteMessageConfig{ChatID: chatID, MessageID: msgID})
	WarnUser(bot, chatID, userID, bot.Self.ID, fmt.Sprintf("Anti-Spam: %s", reason))

	database.Pool.Exec(context.Background(), `
		INSERT INTO antispam_incidents (chat_id, user_id, spam_type, detail, action, created_at)
		VALUES ($1, $2, $3, $4, 'warn', NOW())
	`, chatID, userID, reason, "Single message violation")

	workers.IncrMetric(chatID, workers.MetricSpamBlocked)
}

// countEmojis counts standard and presentation emojis in a string
func countEmojis(s string) int {
	count := 0
	for _, r := range s {
		if isEmojiRune(r) {
			count++
		}
	}
	return count
}

func isEmojiRune(r rune) bool {
	// Common Emoji Unicode Blocks
	if r >= 0x1F600 && r <= 0x1F64F { // Emoticons
		return true
	}
	if r >= 0x1F300 && r <= 0x1F5FF { // Misc Symbols and Pictographs
		return true
	}
	if r >= 0x1F680 && r <= 0x1F6FF { // Transport and Map
		return true
	}
	if r >= 0x1F700 && r <= 0x1F77F { // Alchemical Symbols
		return true
	}
	if r >= 0x1F780 && r <= 0x1F7FF { // Geometric Shapes Extended
		return true
	}
	if r >= 0x1F800 && r <= 0x1F8FF { // Supplemental Arrows-C
		return true
	}
	if r >= 0x1F900 && r <= 0x1F9FF { // Supplemental Symbols and Pictographs
		return true
	}
	if r >= 0x1FA00 && r <= 0x1FA6F { // Chess Symbols
		return true
	}
	if r >= 0x1FA70 && r <= 0x1FAFF { // Symbols and Pictographs Extended-A
		return true
	}
	if r >= 0x2600 && r <= 0x26FF { // Misc symbols
		return true
	}
	if r >= 0x2700 && r <= 0x27BF { // Dingbats
		return true
	}
	return false
}
