package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

	"minimate-bot/database"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Pre-compiled regex patterns for script and link detection
var (
	reCJK      = regexp.MustCompile(`[\p{Han}\p{Hiragana}\p{Katakana}\p{Hangul}]`)
	reCyrillic = regexp.MustCompile(`[\p{Cyrillic}]`)
	reRTL      = regexp.MustCompile(`[\p{Arabic}\p{Hebrew}\p{Syriac}\p{Thaana}]`)
	reZalgo    = regexp.MustCompile(`[\x{0300}-\x{036F}\x{1AB0}-\x{1AFF}\x{1DC0}-\x{1DFF}\x{20D0}-\x{20FF}\x{FE20}-\x{FE2F}]{3,}`)
	reEmail    = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)
	rePhone    = regexp.MustCompile(`(?:\+?\d{1,3}[-.\s]?)?\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}`)
	reCashtag  = regexp.MustCompile(`\$[A-Za-z]{2,10}\b`)
	reBotLink  = regexp.MustCompile(`(?i)(?:t\.me|telegram\.me)/(?:[a-zA-Z0-9_]+bot|joinchat/|addbot)`)
)

// All supported 48 lock types grouped by category
var allLockKeys = []string{
	// Media & Files
	"photo", "video", "audio", "voice", "document", "videonote", "gif", "media",
	// Stickers
	"sticker", "stickeranimated", "stickerpremium",
	// Layout & Rich Media
	"album", "collage", "slideshow", "poll", "checklist", "emojigame", "contact", "location",
	// Links & Contacts
	"url", "invitelink", "botlink", "email", "phone", "cashtag",
	// Forwards & Reposting
	"forward", "forwarduser", "forwardbot", "forwardchannel", "forwardstory", "externalreply",
	// Text & Typography
	"text", "command", "spoiler", "emoji", "emojicustom", "emojionly", "button", "richmessage",
	// Scripts & Anti-Spam
	"cjk", "cyrillic", "rtl", "zalgo",
	// Senders & Bots
	"bot", "anonchannel", "comment",
	// Master toggle
	"all",
}

// Memory cache for chat locks: chat_id -> map[lock_name]bool
var (
	locksMapCache = make(map[int64]map[string]bool)
	locksMapMutex sync.RWMutex
)

// Normalize lock aliases to official key names
func normalizeLockKey(key string) string {
	k := strings.ToLower(strings.TrimSpace(key))
	switch k {
	case "links", "link", "urls", "url":
		return "url"
	case "invites", "invitelink", "invitelinks", "invite", "invlinks":
		return "invitelink"
	case "botlinks", "botlink", "boturl":
		return "botlink"
	case "forwards", "forward", "fwd":
		return "forward"
	case "forwarduser", "forwardusers", "fwduser":
		return "forwarduser"
	case "forwardbot", "forwardbots", "fwdbot":
		return "forwardbot"
	case "forwardchannel", "forwardchannels", "fwdchannel":
		return "forwardchannel"
	case "forwardstory", "forwardstories", "fwdstory":
		return "forwardstory"
	case "externalreply", "extreply", "extreplies":
		return "externalreply"
	case "stickers", "sticker":
		return "sticker"
	case "stickeranimated", "animatedsticker", "animsticker":
		return "stickeranimated"
	case "stickerpremium", "premiumsticker", "premimsticker":
		return "stickerpremium"
	case "gifs", "gif", "animation", "animations":
		return "gif"
	case "media", "allmedia":
		return "media"
	case "photos", "photo", "pic", "pics", "image", "images":
		return "photo"
	case "videos", "video", "vid", "vids":
		return "video"
	case "audio", "audios", "music", "song", "songs":
		return "audio"
	case "voice", "voices", "voicenote", "voicenotes", "vn":
		return "voice"
	case "videonote", "videonotes", "roundvideo", "roundvideos", "notevideo":
		return "videonote"
	case "doc", "docs", "document", "documents", "file", "files":
		return "document"
	case "location", "locations", "loc", "venue", "venues":
		return "location"
	case "contact", "contacts", "vcard":
		return "contact"
	case "poll", "polls", "quiz":
		return "poll"
	case "checklist", "checklists":
		return "checklist"
	case "game", "games", "emojigame", "dice":
		return "emojigame"
	case "album", "albums", "collage", "collages", "slideshow", "slideshows":
		return "album"
	case "bots", "bot", "guestbot", "guestbots":
		return "bot"
	case "anonchannel", "anon", "anonymous", "channel", "channels":
		return "anonchannel"
	case "comment", "comments", "discussion":
		return "comment"
	case "text", "messages", "msg", "msgs":
		return "text"
	case "command", "commands", "cmd", "cmds":
		return "command"
	case "spoiler", "spoilers":
		return "spoiler"
	case "emoji", "emojis":
		return "emoji"
	case "emojicustom", "customemoji", "customemojis":
		return "emojicustom"
	case "emojionly", "onlyemoji":
		return "emojionly"
	case "button", "buttons", "inline", "keyboard":
		return "button"
	case "richmessage", "rich", "formatting", "markdown":
		return "richmessage"
	case "cashtag", "cashtags":
		return "cashtag"
	case "email", "emails", "mail":
		return "email"
	case "phone", "phones", "phonenumber", "telephonenumber":
		return "phone"
	case "cjk", "chinese", "japanese", "korean", "asian":
		return "cjk"
	case "cyrillic", "russian":
		return "cyrillic"
	case "rtl", "arabic", "hebrew", "persian":
		return "rtl"
	case "zalgo", "glitch":
		return "zalgo"
	case "reaction", "reactions":
		return "reaction"
	case "outsidereaction", "outsidereactions":
		return "outsidereaction"
	case "all", "everything":
		return "all"
	default:
		return k
	}
}

// Legacy struct for backward compatibility
type LockSettings struct {
	LockLinks    bool
	LockForwards bool
	LockStickers bool
	LockBots     bool
	LockMedia    bool
	LockInvites  bool
}

func getLocks(chatID int64) LockSettings {
	m := getChatLocks(chatID)
	return LockSettings{
		LockLinks:    m["url"] || m["invitelink"] || m["all"],
		LockForwards: m["forward"] || m["all"],
		LockStickers: m["sticker"] || m["all"],
		LockBots:     m["bot"] || m["all"],
		LockMedia:    m["media"] || m["all"],
		LockInvites:  m["invitelink"] || m["all"],
	}
}

// loadChatLocks fetches a chat's lock settings from PostgreSQL
func loadChatLocks(chatID int64) map[string]bool {
	locksMapMutex.Lock()
	defer locksMapMutex.Unlock()

	if m, exists := locksMapCache[chatID]; exists {
		return m
	}

	locks := make(map[string]bool)
	var rawJSON []byte
	var lockLinks, lockForwards, lockStickers, lockBots, lockMedia, lockInvites bool

	query := `
		SELECT COALESCE(locks, '{}'::jsonb), lock_links, lock_forwards, lock_stickers, lock_bots, lock_media, lock_invites
		FROM chat_locks WHERE chat_id = $1
	`
	err := database.Pool.QueryRow(context.Background(), query, chatID).
		Scan(&rawJSON, &lockLinks, &lockForwards, &lockStickers, &lockBots, &lockMedia, &lockInvites)

	if err == nil && len(rawJSON) > 0 {
		_ = json.Unmarshal(rawJSON, &locks)
	}

	// Backfill legacy column states if not set in JSON
	if lockLinks && !locks["url"] {
		locks["url"] = true
	}
	if lockForwards && !locks["forward"] {
		locks["forward"] = true
	}
	if lockStickers && !locks["sticker"] {
		locks["sticker"] = true
	}
	if lockBots && !locks["bot"] {
		locks["bot"] = true
	}
	if lockMedia && !locks["media"] {
		locks["media"] = true
	}
	if lockInvites && !locks["invitelink"] {
		locks["invitelink"] = true
	}

	locksMapCache[chatID] = locks
	return locks
}

// getChatLocks retrieves the cached locks map for a chat
func getChatLocks(chatID int64) map[string]bool {
	locksMapMutex.RLock()
	m, exists := locksMapCache[chatID]
	locksMapMutex.RUnlock()

	if !exists {
		return loadChatLocks(chatID)
	}
	return m
}

// isChatLocked checks if a specific lock or "all" is active
func isChatLocked(chatID int64, key string) bool {
	locks := getChatLocks(chatID)
	if locks["all"] {
		return true
	}
	return locks[key]
}

// Helper: Identify emoji rune ranges
func isEmojiRune(r rune) bool {
	if (r >= 0x1F300 && r <= 0x1FAD6) ||
		(r >= 0x1F900 && r <= 0x1F9FF) ||
		(r >= 0x1F600 && r <= 0x1F64F) ||
		(r >= 0x1F680 && r <= 0x1F6FF) ||
		(r >= 0x2600 && r <= 0x27BF) ||
		(r >= 0xFE00 && r <= 0xFE0F) ||
		(r >= 0x1F1E6 && r <= 0x1F1FF) ||
		unicode.Is(unicode.Symbol, r) {
		return true
	}
	return false
}

// Helper: Check if string contains standard unicode emoji
func containsEmoji(s string) bool {
	for _, r := range s {
		if isEmojiRune(r) && r > 127 {
			return true
		}
	}
	return false
}

// Helper: Check if string contains ONLY emojis and whitespace
func isEmojiOnly(s string) bool {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return false
	}
	hasEmoji := false
	for _, r := range trimmed {
		if unicode.IsSpace(r) {
			continue
		}
		if isEmojiRune(r) {
			hasEmoji = true
			continue
		}
		return false
	}
	return hasEmoji
}

// CheckMessageLocks intercepts incoming messages and enforces active security locks
func CheckMessageLocks(bot *tgbotapi.BotAPI, message *tgbotapi.Message) bool {
	if message == nil || message.Chat == nil {
		return false
	}

	chatID := message.Chat.ID
	fromID := int64(0)
	var userName string

	if message.From != nil {
		fromID = message.From.ID
		userName = message.From.FirstName
	} else if message.SenderChat != nil {
		fromID = message.SenderChat.ID
		userName = message.SenderChat.Title
	}

	// Administrators always bypass locks
	if isAdmin(bot, chatID, fromID) {
		return false
	}

	deleteReason := ""

	// 1. Check Anonymous Channel / Linked Channel Comments
	if message.SenderChat != nil && message.SenderChat.ID != chatID {
		if isChatLocked(chatID, "anonchannel") {
			deleteReason = "📢 Anonymous Channel messages are locked."
		}
	}
	if deleteReason == "" && message.IsAutomaticForward && isChatLocked(chatID, "comment") {
		deleteReason = "💬 Discussion comments are locked."
	}

	// 2. Check Forwards & Reposts
	hasForward := message.ForwardDate != 0 || message.ForwardFrom != nil || message.ForwardFromChat != nil || message.ForwardSenderName != ""
	if deleteReason == "" && hasForward {
		if isChatLocked(chatID, "forward") {
			deleteReason = "⏩ Forwarded messages are locked."
		} else if message.ForwardFrom != nil && message.ForwardFrom.IsBot && isChatLocked(chatID, "forwardbot") {
			deleteReason = "🤖 Forwards from bots are locked."
		} else if message.ForwardFromChat != nil && message.ForwardFromChat.IsChannel() && isChatLocked(chatID, "forwardchannel") {
			deleteReason = "📢 Forwards from channels are locked."
		} else if message.ForwardFrom != nil && !message.ForwardFrom.IsBot && isChatLocked(chatID, "forwarduser") {
			deleteReason = "👤 Forwards from users are locked."
		} else if message.ForwardSenderName != "" && isChatLocked(chatID, "forwardstory") {
			deleteReason = "📖 Forwarded stories are locked."
		}
	}

	// 3. Check External Replies
	if deleteReason == "" && isChatLocked(chatID, "externalreply") {
		if message.ReplyToMessage != nil && message.ReplyToMessage.Chat != nil && message.ReplyToMessage.Chat.ID != chatID {
			deleteReason = "🔄 External replies from other chats are locked."
		}
	}

	// 4. Check Stickers & Animations
	if deleteReason == "" && message.Sticker != nil {
		if isChatLocked(chatID, "sticker") {
			deleteReason = "🎭 Stickers are locked."
		} else if message.Sticker.IsAnimated && isChatLocked(chatID, "stickeranimated") {
			deleteReason = "✨ Animated stickers are locked."
		} else if message.Sticker.IsAnimated && isChatLocked(chatID, "stickerpremium") {
			deleteReason = "💎 Premium stickers are locked."
		}
	}

	// 5. Check GIF Animations
	if deleteReason == "" && message.Animation != nil {
		if isChatLocked(chatID, "gif") || isChatLocked(chatID, "media") {
			deleteReason = "🎞️ GIF animations are locked."
		}
	}

	// 6. Check Media Album / Collage / Slideshow
	if deleteReason == "" && message.MediaGroupID != "" {
		if isChatLocked(chatID, "album") || isChatLocked(chatID, "collage") || isChatLocked(chatID, "slideshow") {
			deleteReason = "🖼️ Media albums & collages are locked."
		}
	}

	// 7. Check Photos
	if deleteReason == "" && len(message.Photo) > 0 {
		if isChatLocked(chatID, "photo") || isChatLocked(chatID, "media") {
			deleteReason = "📸 Photos are locked."
		}
	}

	// 8. Check Videos
	if deleteReason == "" && message.Video != nil {
		if isChatLocked(chatID, "video") || isChatLocked(chatID, "media") {
			deleteReason = "🎥 Videos are locked."
		}
	}

	// 9. Check Video Notes (Round Videos)
	if deleteReason == "" && message.VideoNote != nil {
		if isChatLocked(chatID, "videonote") || isChatLocked(chatID, "media") {
			deleteReason = "📹 Round video notes are locked."
		}
	}

	// 10. Check Audio Files
	if deleteReason == "" && message.Audio != nil {
		if isChatLocked(chatID, "audio") || isChatLocked(chatID, "media") {
			deleteReason = "🎵 Audio tracks are locked."
		}
	}

	// 11. Check Voice Notes
	if deleteReason == "" && message.Voice != nil {
		if isChatLocked(chatID, "voice") || isChatLocked(chatID, "media") {
			deleteReason = "🎙️ Voice notes are locked."
		}
	}

	// 12. Check Documents & Files
	if deleteReason == "" && message.Document != nil {
		if isChatLocked(chatID, "document") || isChatLocked(chatID, "media") {
			deleteReason = "📁 Documents & files are locked."
		}
	}

	// 13. Check Contacts
	if deleteReason == "" && message.Contact != nil {
		if isChatLocked(chatID, "contact") {
			deleteReason = "📇 Shared contacts are locked."
		}
	}

	// 14. Check Locations & Venues
	if deleteReason == "" && (message.Location != nil || message.Venue != nil) {
		if isChatLocked(chatID, "location") {
			deleteReason = "📍 Locations & venues are locked."
		}
	}

	// 15. Check Polls & Checklists
	if deleteReason == "" && message.Poll != nil {
		if isChatLocked(chatID, "poll") || isChatLocked(chatID, "checklist") {
			deleteReason = "📊 Polls & surveys are locked."
		}
	}

	// 16. Check Games & Dice
	if deleteReason == "" && (message.Game != nil || message.Dice != nil) {
		if isChatLocked(chatID, "emojigame") {
			deleteReason = "🎲 Interactive games & dice are locked."
		}
	}

	// 17. Check Inline Buttons
	if deleteReason == "" && message.ReplyMarkup != nil && len(message.ReplyMarkup.InlineKeyboard) > 0 {
		if isChatLocked(chatID, "button") {
			deleteReason = "🎛️ Inline keyboard buttons are locked."
		}
	}

	// 18. Text & Entity Analysis
	fullText := message.Text
	if message.Caption != "" {
		if fullText != "" {
			fullText += " " + message.Caption
		} else {
			fullText = message.Caption
		}
	}

	var allEntities []tgbotapi.MessageEntity
	if message.Entities != nil {
		allEntities = append(allEntities, message.Entities...)
	}
	if message.CaptionEntities != nil {
		allEntities = append(allEntities, message.CaptionEntities...)
	}

	if deleteReason == "" && fullText != "" {
		lowerText := strings.ToLower(fullText)

		// A. Commands
		if isChatLocked(chatID, "command") && (message.IsCommand() || strings.HasPrefix(fullText, "/") || strings.HasPrefix(fullText, "!")) {
			deleteReason = "⚡ Bot commands are locked for regular members."
		}

		// B. Telegram Invite Links
		if deleteReason == "" && isChatLocked(chatID, "invitelink") {
			if strings.Contains(lowerText, "t.me/+") || strings.Contains(lowerText, "t.me/joinchat/") ||
				strings.Contains(lowerText, "telegram.me/+") || strings.Contains(lowerText, "telegram.me/joinchat/") {
				deleteReason = "🔗 Group invite links are locked."
			}
		}

		// C. Bot Links & Usernames
		if deleteReason == "" && isChatLocked(chatID, "botlink") {
			if reBotLink.MatchString(fullText) {
				deleteReason = "🤖 Bot links are locked."
			}
		}

		// D. General URLs / Links
		if deleteReason == "" && isChatLocked(chatID, "url") {
			hasURL := strings.Contains(lowerText, "http://") || strings.Contains(lowerText, "https://") ||
				strings.Contains(lowerText, "t.me/") || strings.Contains(lowerText, "telegram.me/")
			if !hasURL {
				for _, ent := range allEntities {
					if ent.Type == "url" || ent.Type == "text_link" {
						hasURL = true
						break
					}
				}
			}
			if hasURL {
				deleteReason = "🌐 Web links & URLs are locked."
			}
		}

		// E. Email Addresses
		if deleteReason == "" && isChatLocked(chatID, "email") {
			for _, ent := range allEntities {
				if ent.Type == "email" {
					deleteReason = "📧 Email addresses are locked."
					break
				}
			}
			if deleteReason == "" && reEmail.MatchString(fullText) {
				deleteReason = "📧 Email addresses are locked."
			}
		}

		// F. Phone Numbers
		if deleteReason == "" && isChatLocked(chatID, "phone") {
			for _, ent := range allEntities {
				if ent.Type == "phone_number" {
					deleteReason = "📞 Phone numbers are locked."
					break
				}
			}
			if deleteReason == "" && rePhone.MatchString(fullText) {
				deleteReason = "📞 Phone numbers are locked."
			}
		}

		// G. Cashtags ($USD, $BTC)
		if deleteReason == "" && isChatLocked(chatID, "cashtag") {
			for _, ent := range allEntities {
				if ent.Type == "cashtag" {
					deleteReason = "💲 Cashtags are locked."
					break
				}
			}
			if deleteReason == "" && reCashtag.MatchString(fullText) {
				deleteReason = "💲 Cashtags are locked."
			}
		}

		// H. Spoilers
		if deleteReason == "" && isChatLocked(chatID, "spoiler") {
			for _, ent := range allEntities {
				if ent.Type == "spoiler" {
					deleteReason = "🙈 Hidden spoiler messages are locked."
					break
				}
			}
		}

		// I. Custom Emojis
		if deleteReason == "" && isChatLocked(chatID, "emojicustom") {
			for _, ent := range allEntities {
				if ent.Type == "custom_emoji" {
					deleteReason = "🎨 Custom premium emojis are locked."
					break
				}
			}
		}

		// J. Rich Formatting
		if deleteReason == "" && isChatLocked(chatID, "richmessage") {
			for _, ent := range allEntities {
				switch ent.Type {
				case "bold", "italic", "code", "pre", "spoiler", "blockquote", "expandable_blockquote", "strikethrough", "underline":
					deleteReason = "🖋️ Rich formatted text is locked."
				}
				if deleteReason != "" {
					break
				}
			}
		}

		// K. Foreign Alphabets & Scripts
		if deleteReason == "" && isChatLocked(chatID, "cjk") && reCJK.MatchString(fullText) {
			deleteReason = "🈯 Chinese / Japanese / Korean text is locked."
		}
		if deleteReason == "" && isChatLocked(chatID, "cyrillic") && reCyrillic.MatchString(fullText) {
			deleteReason = "🇷🇺 Cyrillic (Russian) text is locked."
		}
		if deleteReason == "" && isChatLocked(chatID, "rtl") && reRTL.MatchString(fullText) {
			deleteReason = "🇦🇪 Right-to-Left (Arabic/Hebrew) text is locked."
		}
		if deleteReason == "" && isChatLocked(chatID, "zalgo") && reZalgo.MatchString(fullText) {
			deleteReason = "⚡ Zalgo / glitch text is locked."
		}

		// L. Emojis
		if deleteReason == "" && isChatLocked(chatID, "emoji") && containsEmoji(fullText) {
			deleteReason = "😀 Emojis are locked."
		}
		if deleteReason == "" && isChatLocked(chatID, "emojionly") && isEmojiOnly(fullText) {
			deleteReason = "😃 Emoji-only messages are locked."
		}

		// M. Plain Text Lock
		if deleteReason == "" && isChatLocked(chatID, "text") {
			if len(message.Photo) == 0 && message.Video == nil && message.Audio == nil && message.Voice == nil &&
				message.Document == nil && message.Sticker == nil && message.Animation == nil && message.VideoNote == nil {
				deleteReason = "💬 Plain text messages are locked."
			}
		}
	}

	// 19. If restriction triggered, delete message and send self-destruct warning
	if deleteReason != "" {
		bot.Request(tgbotapi.NewDeleteMessage(chatID, message.MessageID))
		go func(name string, reason string) {
			noticeText := fmt.Sprintf("🛡️ <b>Security Shield:</b> Message from <b>%s</b> was removed.\n<blockquote>%s</blockquote>",
				html.EscapeString(name), reason)
			msg := tgbotapi.NewMessage(chatID, noticeText)
			msg.ParseMode = "HTML"
			sent, err := SafeSend(bot, msg)
			if err == nil && sent.MessageID != 0 {
				time.Sleep(5 * time.Second)
				bot.Request(tgbotapi.NewDeleteMessage(chatID, sent.MessageID))
			}
		}(userName, deleteReason)
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
		msg := tgbotapi.NewMessage(chatID, "❌ Only group administrators can configure security locks.")
		msg.ParseMode = "HTML"
		SafeSend(bot, msg)
		return
	}

	switch cmd {
	case "locktypes":
		help := `🔐 <b>Available Security Lock Types Directory:</b>

<blockquote expandable>📁 <b>Media &amp; Files:</b>
• <code>photo</code>, <code>video</code>, <code>audio</code>, <code>voice</code>
• <code>document</code>, <code>videonote</code>, <code>gif</code>, <code>media</code> (all)

🎭 <b>Stickers &amp; Visuals:</b>
• <code>sticker</code>, <code>stickeranimated</code>, <code>stickerpremium</code>

🖼️ <b>Layout &amp; Groups:</b>
• <code>album</code> (collages/slideshows), <code>poll</code>, <code>emojigame</code>, <code>contact</code>, <code>location</code>

🔗 <b>Links &amp; Contacts:</b>
• <code>url</code>, <code>invitelink</code>, <code>botlink</code>, <code>email</code>, <code>phone</code>, <code>cashtag</code>

⏩ <b>Forwards &amp; Reposting:</b>
• <code>forward</code> (all), <code>forwarduser</code>, <code>forwardbot</code>
• <code>forwardchannel</code>, <code>forwardstory</code>, <code>externalreply</code>

✍️ <b>Text &amp; Formatting:</b>
• <code>text</code>, <code>command</code>, <code>spoiler</code>, <code>emoji</code>
• <code>emojicustom</code>, <code>emojionly</code>, <code>button</code>, <code>richmessage</code>

🌐 <b>Foreign Scripts &amp; Glitch:</b>
• <code>cjk</code> (Chinese/Japanese/Korean)
• <code>cyrillic</code> (Russian)
• <code>rtl</code> (Arabic/Hebrew)
• <code>zalgo</code> (Glitch / stacked text)

🤖 <b>Senders &amp; Bots:</b>
• <code>bot</code> (Guest bots), <code>anonchannel</code>, <code>comment</code>

🌟 <b>Master Controls:</b>
• <code>all</code> — Locks/unlocks all 48 security shields at once</blockquote>

<b>Usage Examples:</b>
• <code>/lock url</code>
• <code>/lock cjk</code>
• <code>/lock sticker</code>
• <code>/lock all</code>
• <code>/unlock invitelink</code>`
		msg := tgbotapi.NewMessage(chatID, help)
		msg.ParseMode = "HTML"
		SafeSend(bot, msg)

	case "locks":
		locks := getChatLocks(chatID)
		isAll := locks["all"]

		badge := func(k string) string {
			if isAll || locks[k] {
				return "🔒"
			}
			return "🔓"
		}

		text := fmt.Sprintf(`🛡️ <b>Security Locks Dashboard for %s:</b>

<blockquote expandable>📁 <b>Media &amp; Files:</b>
%s <code>photo</code>  %s <code>video</code>  %s <code>audio</code>  %s <code>voice</code>
%s <code>document</code>  %s <code>videonote</code>  %s <code>gif</code>  %s <code>media</code>

🎭 <b>Stickers &amp; Groups:</b>
%s <code>sticker</code>  %s <code>animated</code>  %s <code>premium</code>
%s <code>album</code>  %s <code>poll</code>  %s <code>game</code>  %s <code>contact</code>  %s <code>location</code>

🔗 <b>Links &amp; Web:</b>
%s <code>url</code>  %s <code>invitelink</code>  %s <code>botlink</code>
%s <code>email</code>  %s <code>phone</code>  %s <code>cashtag</code>

⏩ <b>Forwards &amp; Reposts:</b>
%s <code>forward</code>  %s <code>fwduser</code>  %s <code>fwdbot</code>
%s <code>fwdchannel</code>  %s <code>fwdstory</code>  %s <code>externalreply</code>

✍️ <b>Text &amp; Typography:</b>
%s <code>text</code>  %s <code>command</code>  %s <code>spoiler</code>  %s <code>richmessage</code>
%s <code>emoji</code>  %s <code>emojicustom</code>  %s <code>emojionly</code>  %s <code>button</code>

🌐 <b>Scripts &amp; Glitch:</b>
%s <code>cjk</code>  %s <code>cyrillic</code>  %s <code>rtl</code>  %s <code>zalgo</code>

🤖 <b>Senders &amp; Bots:</b>
%s <code>bot</code>  %s <code>anonchannel</code>  %s <code>comment</code></blockquote>

💡 <i>Use <code>/lock &lt;type&gt;</code> or <code>/unlock &lt;type&gt;</code> to toggle shields.</i>
📖 <i>Use <code>/locktypes</code> for the full list of descriptions.</i>`,
			html.EscapeString(message.Chat.Title),
			badge("photo"), badge("video"), badge("audio"), badge("voice"),
			badge("document"), badge("videonote"), badge("gif"), badge("media"),
			badge("sticker"), badge("stickeranimated"), badge("stickerpremium"),
			badge("album"), badge("poll"), badge("emojigame"), badge("contact"), badge("location"),
			badge("url"), badge("invitelink"), badge("botlink"),
			badge("email"), badge("phone"), badge("cashtag"),
			badge("forward"), badge("forwarduser"), badge("forwardbot"),
			badge("forwardchannel"), badge("forwardstory"), badge("externalreply"),
			badge("text"), badge("command"), badge("spoiler"), badge("richmessage"),
			badge("emoji"), badge("emojicustom"), badge("emojionly"), badge("button"),
			badge("cjk"), badge("cyrillic"), badge("rtl"), badge("zalgo"),
			badge("bot"), badge("anonchannel"), badge("comment"),
		)

		msg := tgbotapi.NewMessage(chatID, text)
		msg.ParseMode = "HTML"
		SafeSend(bot, msg)

	case "lock", "unlock":
		rawArg := strings.TrimSpace(args)
		if rawArg == "" {
			msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Usage: <code>/%s &lt;lock_type&gt;</code>\nExample: <code>/%s url</code> or <code>/%s all</code>\nUse <code>/locktypes</code> for full directory.", cmd, cmd, cmd))
			msg.ParseMode = "HTML"
			SafeSend(bot, msg)
			return
		}

		isLocking := (cmd == "lock")
		normalized := normalizeLockKey(rawArg)

		locks := getChatLocks(chatID)
		// Clone map to avoid race condition during write
		newLocks := make(map[string]bool)
		for k, v := range locks {
			newLocks[k] = v
		}

		if normalized == "all" {
			newLocks["all"] = isLocking
			for _, k := range allLockKeys {
				newLocks[k] = isLocking
			}
		} else if normalized == "media" {
			newLocks["media"] = isLocking
			mediaKeys := []string{"photo", "video", "audio", "voice", "document", "videonote", "gif"}
			for _, mk := range mediaKeys {
				newLocks[mk] = isLocking
			}
		} else {
			newLocks[normalized] = isLocking
			if !isLocking {
				newLocks["all"] = false
			}
		}

		// Marshal to JSONB
		jsonBytes, err := json.Marshal(newLocks)
		if err != nil {
			log.Printf("JSON marshal error: %v", err)
			return
		}

		query := `
			INSERT INTO chat_locks (chat_id, locks, lock_links, lock_forwards, lock_stickers, lock_bots, lock_media, lock_invites, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
			ON CONFLICT (chat_id) DO UPDATE SET
				locks = EXCLUDED.locks,
				lock_links = EXCLUDED.lock_links,
				lock_forwards = EXCLUDED.lock_forwards,
				lock_stickers = EXCLUDED.lock_stickers,
				lock_bots = EXCLUDED.lock_bots,
				lock_media = EXCLUDED.lock_media,
				lock_invites = EXCLUDED.lock_invites,
				updated_at = NOW();
		`
		_, dbErr := database.Pool.Exec(context.Background(), query,
			chatID,
			jsonBytes,
			newLocks["url"] || newLocks["all"],
			newLocks["forward"] || newLocks["all"],
			newLocks["sticker"] || newLocks["all"],
			newLocks["bot"] || newLocks["all"],
			newLocks["media"] || newLocks["all"],
			newLocks["invitelink"] || newLocks["all"],
		)

		if dbErr != nil {
			log.Printf("DB error saving locks: %v", dbErr)
			SafeSend(bot, tgbotapi.NewMessage(chatID, "❌ Database error updating security locks."))
			return
		}

		locksMapMutex.Lock()
		locksMapCache[chatID] = newLocks
		locksMapMutex.Unlock()

		actionStr := "locked 🔒"
		if !isLocking {
			actionStr = "unlocked 🔓"
		}

		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Security Shield <b>%s</b> is now <b>%s</b>.", html.EscapeString(normalized), actionStr))
		msg.ParseMode = "HTML"
		SafeSend(bot, msg)
	}
}