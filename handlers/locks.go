package handlers

import (
	"context"
	"fmt"
	"html"
	"log"
	"strings"
	"sync"
	"time"

	"minimate-bot/database"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// LockSettings holds the active locks for a chat
type LockSettings struct {
	LockLinks    bool
	LockForwards bool
	LockStickers bool
	LockBots     bool
	LockMedia    bool
	LockInvites  bool
}

var (
	locksCache = make(map[int64]LockSettings)
	locksMutex sync.RWMutex
)

// loadLocks retrieves a chat's security locks and caches them in memory
func loadLocks(chatID int64) LockSettings {
	locksMutex.Lock()
	defer locksMutex.Unlock()

	if settings, exists := locksCache[chatID]; exists {
		return settings
	}

	var s LockSettings
	query := `
		SELECT lock_links, lock_forwards, lock_stickers, lock_bots, lock_media, lock_invites
		FROM chat_locks WHERE chat_id = $1
	`
	err := database.Pool.QueryRow(context.Background(), query, chatID).
		Scan(&s.LockLinks, &s.LockForwards, &s.LockStickers, &s.LockBots, &s.LockMedia, &s.LockInvites)

	if err != nil {
		s = LockSettings{}
	}

	locksCache[chatID] = s
	return s
}

func getLocks(chatID int64) LockSettings {
	locksMutex.RLock()
	s, exists := locksCache[chatID]
	locksMutex.RUnlock()

	if !exists {
		return loadLocks(chatID)
	}
	return s
}

// CheckMessageLocks intercepts incoming messages and deletes restricted content
func CheckMessageLocks(bot *tgbotapi.BotAPI, message *tgbotapi.Message) bool {
	if message == nil || message.Chat == nil {
		return false
	}

	chatID := message.Chat.ID
	fromID := int64(0)
	if message.From != nil {
		fromID = message.From.ID
	} else if message.SenderChat != nil {
		fromID = message.SenderChat.ID
	}

	// Administrators always bypass locks
	if isAdmin(bot, chatID, fromID) {
		return false
	}

	locks := getLocks(chatID)
	deleteReason := ""

	// 1. Check Links / Invites
	if locks.LockLinks || locks.LockInvites {
		hasLink := false
		text := strings.ToLower(message.Text + " " + message.Caption)
		if strings.Contains(text, "http://") || strings.Contains(text, "https://") || strings.Contains(text, "t.me/") || strings.Contains(text, "telegram.me/") {
			hasLink = true
		}
		if !hasLink && message.Entities != nil {
			for _, ent := range message.Entities {
				if ent.Type == "url" || ent.Type == "text_link" {
					hasLink = true
					break
				}
			}
		}
		if hasLink {
			deleteReason = "🔗 Links & Invitations are locked in this group."
		}
	}

	// 2. Check Forwards
	if deleteReason == "" && locks.LockForwards {
		if message.ForwardDate != 0 || message.ForwardFrom != nil || message.ForwardFromChat != nil || message.ForwardSenderName != "" {
			deleteReason = "⏩ Forwarded messages are locked in this group."
		}
	}

	// 3. Check Stickers & Animations
	if deleteReason == "" && locks.LockStickers {
		if message.Sticker != nil || message.Animation != nil {
			deleteReason = "🎭 Stickers & GIFs are locked in this group."
		}
	}

	// 4. Check Media (Photos, Videos, Audio, Documents, Voice)
	if deleteReason == "" && locks.LockMedia {
		if len(message.Photo) > 0 || message.Video != nil || message.Audio != nil || message.Voice != nil || message.Document != nil || message.VideoNote != nil {
			deleteReason = "📁 Media files are locked in this group."
		}
	}

	// If restricted content detected, delete and send self-destruct notice
	if deleteReason != "" {
		bot.Request(tgbotapi.NewDeleteMessage(chatID, message.MessageID))
		go func(userName string, reason string) {
			noticeText := fmt.Sprintf("🛡️ <b>Notice:</b> Message from <b>%s</b> was removed.\n<blockquote>%s</blockquote>",
				html.EscapeString(userName), reason)
			msg := tgbotapi.NewMessage(chatID, noticeText)
			msg.ParseMode = "HTML"
			sent, err := bot.Send(msg)
			if err == nil {
				time.Sleep(5 * time.Second)
				bot.Request(tgbotapi.NewDeleteMessage(chatID, sent.MessageID))
			}
		}(message.From.FirstName, deleteReason)
		return true
	}

	return false
}

// HandleLockCommand processes /lock, /unlock, /locks, /locktypes
func HandleLockCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message, cmd string, args string) {
	chatID := message.Chat.ID
	fromID := int64(0)
	if message.From != nil {
		fromID = message.From.ID
	} else if message.SenderChat != nil {
		fromID = message.SenderChat.ID
	}

	if !isAdmin(bot, chatID, fromID) {
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Only group administrators can configure security locks."))
		return
	}

	switch cmd {
	case "locktypes":
		help := `🔐 <b>Available Security Lock Types:</b>

• <code>links</code> — Web links, URLs & invite links
• <code>forwards</code> — Forwarded messages from other channels/chats
• <code>stickers</code> — Stickers, GIFs & animations
• <code>media</code> — Photos, videos, voice notes, documents
• <code>bots</code> — Automatically bans newly added non-admin bots
• <code>all</code> — Locks all above categories

<b>Usage:</b>
<code>/lock links</code>
<code>/unlock forwards</code>`
		msg := tgbotapi.NewMessage(chatID, help)
		msg.ParseMode = "HTML"
		bot.Send(msg)

	case "locks":
		s := getLocks(chatID)
		statusFormat := func(b bool) string {
			if b {
				return "🔒 <b>LOCKED</b>"
			}
			return "🔓 <i>Unlocked</i>"
		}

		text := fmt.Sprintf(`🛡️ <b>Security Locks Status for %s:</b>

<blockquote expandable>• <b>Links & Invites:</b> %s
• <b>Forwards:</b> %s
• <b>Stickers & GIFs:</b> %s
• <b>Media (Photo/Video/Doc):</b> %s
• <b>Unauthorized Bots:</b> %s</blockquote>

💡 <i>Use <code>/lock &lt;type&gt;</code> or <code>/unlock &lt;type&gt;</code> to modify.</i>`,
			html.EscapeString(message.Chat.Title),
			statusFormat(s.LockLinks),
			statusFormat(s.LockForwards),
			statusFormat(s.LockStickers),
			statusFormat(s.LockMedia),
			statusFormat(s.LockBots),
		)

		msg := tgbotapi.NewMessage(chatID, text)
		msg.ParseMode = "HTML"
		bot.Send(msg)

	case "lock", "unlock":
		targetType := strings.ToLower(strings.TrimSpace(args))
		if targetType == "" {
			msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Usage: <code>/%s &lt;links|forwards|stickers|media|bots|all&gt;</code>\nUse <code>/locktypes</code> for full directory.", cmd))
			msg.ParseMode = "HTML"
			bot.Send(msg)
			return
		}

		isLocking := (cmd == "lock")
		s := getLocks(chatID)

		switch targetType {
		case "links", "invites", "url":
			s.LockLinks = isLocking
			s.LockInvites = isLocking
		case "forwards", "forward":
			s.LockForwards = isLocking
		case "stickers", "sticker", "gifs", "gif":
			s.LockStickers = isLocking
		case "media", "photos", "videos", "files":
			s.LockMedia = isLocking
		case "bots", "bot":
			s.LockBots = isLocking
		case "all":
			s.LockLinks = isLocking
			s.LockInvites = isLocking
			s.LockForwards = isLocking
			s.LockStickers = isLocking
			s.LockMedia = isLocking
			s.LockBots = isLocking
		default:
			msg := tgbotapi.NewMessage(chatID, "❌ Invalid lock type. Use <code>/locktypes</code> to see all options.")
			msg.ParseMode = "HTML"
			bot.Send(msg)
			return
		}

		query := `
			INSERT INTO chat_locks (chat_id, lock_links, lock_forwards, lock_stickers, lock_bots, lock_media, lock_invites)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (chat_id) DO UPDATE SET
				lock_links = EXCLUDED.lock_links,
				lock_forwards = EXCLUDED.lock_forwards,
				lock_stickers = EXCLUDED.lock_stickers,
				lock_bots = EXCLUDED.lock_bots,
				lock_media = EXCLUDED.lock_media,
				lock_invites = EXCLUDED.lock_invites;
		`
		_, err := database.Pool.Exec(context.Background(), query,
			chatID, s.LockLinks, s.LockForwards, s.LockStickers, s.LockBots, s.LockMedia, s.LockInvites)

		if err != nil {
			log.Printf("DB error saving locks: %v", err)
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Database error updating locks."))
			return
		}

		locksMutex.Lock()
		locksCache[chatID] = s
		locksMutex.Unlock()

		actionStr := "locked 🔒"
		if !isLocking {
			actionStr = "unlocked 🔓"
		}

		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ <b>%s</b> is now <b>%s</b>.", html.EscapeString(targetType), actionStr))
		msg.ParseMode = "HTML"
		bot.Send(msg)
	}
}