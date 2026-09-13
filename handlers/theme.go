package handlers

import (
	"encoding/json"
	"fmt"
	"html"
	"log"
	"regexp"
	"sort"
	"strings"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Premium Custom Emoji IDs provided for MiniMate Pro
var EmojiMapping = map[string]string{
	// 1. Shiny Blue Shield (New)
	"🛡️": "5197288647275071607",
	"🛡":  "5197288647275071607",

	// 2. Pink Layered Theme Banner (New)
	"🌸": "5213205860498549992",
	"🌺": "5213205860498549992",
	"💮": "5213205860498549992",

	// 3. Screen Tap Hand UI (New)
	"👉":  "5199885118214255386",
	"👆":  "5199885118214255386",
	"📱":  "5199885118214255386",
	"🖥":  "5199885118214255386",
	"🖱️": "5199885118214255386",

	// 4. Books / Library Commands Explorer (New)
	"📚": "5357479219335012900",
	"📖": "5357479219335012900",
	"📜": "5357479219335012900",

	// 5. Pink Megaphone / Broadcast (New)
	"📢": "4967957395331351254",
	"📣": "4967957395331351254",

	// 6. Sparkling VIP Diamond (New)
	"💎": "5427168083074628963",
	"💍": "5427168083074628963",

	// 7. Artist Palette / Themes (New)
	"🎨": "5310039132297242441",

	// 8. Infinity Symbol / Unlimited (New)
	"♾️": "6332197763417118285",
	"♾":  "6332197763417118285",

	// 9. Fast Forward Arrows / Reposts (New)
	"⏩": "5222255721663449310",
	"⏭": "5222255721663449310",
	"➡️": "5222255721663449310",

	// 10. Police Officer Cat / Admin & Moderation (New)
	"👮":  "5298692507306040707",
	"👮‍♂️": "5298692507306040707",
	"👮‍♀️": "5298692507306040707",

	// 11. Red Document Sheet / Filters & Notes (New)
	"📝": "5033080906403808074",
	"📄": "5033080906403808074",
	"📑": "5033080906403808074",

	// 12. Blue 3D Exclamation Cube / Warning Notice (New)
	"⚠️":  "5334544901428229844",
	"⚠️️": "5334544901428229844",
	"🚨":  "5334544901428229844",
	"❗":  "5334544901428229844",

	// 13. Stopwatch / Latency & Uptime (New)
	"⏱️": "5015045170496799920",
	"⏱":  "5015045170496799920",
	"⏳":  "5015045170496799920",
	"⏰":  "5015045170496799920",
	"🕒":  "5015045170496799920",

	// 14. Cross / Error / Ban
	"❌": "5210952531676504517",
	"❎": "5210952531676504517",

	// 15. Pin / Location
	"📌": "5292291996717690768",
	"📍": "5292291996717690768",

	// 16. Stats / Charts / Dashboard
	"📊": "5231200819986047254",
	"📈": "5231200819986047254",
	"📉": "5231200819986047254",

	// 17. Gear / Settings / System
	"⚙️": "5341715473882955310",
	"⚙":  "5341715473882955310",

	// 18. Broom / Cleanup / Purge
	"🧹": "5235929467309796721",
	"🗑️": "5235929467309796721",
	"🗑":  "5235929467309796721",

	// 19. Unlock / Security
	"🔓": "5465443379917629504",
	"🔒": "5465443379917629504",

	// 20. Rocket / Speed / Pong
	"🚀": "5188481279963715781",
	"✈️": "5188481279963715781",

	// 21. Globe / Community
	"🤣": "6105003734444541829",
	"🌐": "6105003734444541829",
	"🌍": "6105003734444541829",
	"🌎": "6105003734444541829",
	"🌏": "6105003734444541829",

	// 22. Calendar / Date / Schedule
	"🗓️": "5413879192267805083",
	"🗓":  "5413879192267805083",
	"📅":  "5413879192267805083",
	"📆":  "5413879192267805083",

	// 23. Crown / Owner / VIP
	"👑": "5433758796289685818",

	// 24. Robot / Bot
	"🤖": "5355051922862653659",

	// 25. Sparkles / Stars / Premium
	"✨️": "5451636889717062286",
	"✨":  "5451636889717062286",
	"🌟":  "5451636889717062286",
	"⭐":  "5451636889717062286",
	"⭐️": "5451636889717062286",

	// 26. Thumbs Up / Verified
	"👍":  "5465465194056525619",
	"👍🏻": "5465465194056525619",
	"👍🏼": "5465465194056525619",
	"👍🏽": "5465465194056525619",
	"👍🏾": "5465465194056525619",
	"👍🏿": "5465465194056525619",
	"✅":  "5465465194056525619",
	"✔️":  "5465465194056525619",

	// 27. Butterfly
	"🦋": "5289862389552919154",

	// 28. Tools / Wrench / Config
	"🛠️": "5462921117423384478",
	"🛠":  "5462921117423384478",
	"🔧":  "5462921117423384478",
	"🔨":  "5462921117423384478",

	// 29. Bolt / Lightning / Latency
	"⚡️": "5438539112070002676",
	"⚡":  "5438539112070002676",
}

// Global Theme Emojis
var (
	IconCrown     = CustomEmoji("👑")
	IconShield    = CustomEmoji("🛡️")
	IconSparkles  = CustomEmoji("✨")
	IconCheck     = CustomEmoji("✅")
	IconCross     = CustomEmoji("❌")
	IconFlower    = CustomEmoji("🌸")
	IconBolt      = CustomEmoji("⚡")
	IconLock      = CustomEmoji("🔒")
	IconWarning   = CustomEmoji("⚠️")
	IconBroom     = CustomEmoji("🧹")
	IconRobot     = CustomEmoji("🤖")
	IconPin       = CustomEmoji("📌")
	IconStats     = CustomEmoji("📊")
	IconGear      = CustomEmoji("⚙️")
	IconThumbsUp  = CustomEmoji("👍")
	IconButterfly = CustomEmoji("🦋")
	IconTools     = CustomEmoji("🛠️")
	IconRocket    = CustomEmoji("🚀")
	IconCalendar  = CustomEmoji("🗓️")
	IconDiamond   = CustomEmoji("💎")
	IconBooks     = CustomEmoji("📚")
	IconMegaphone = CustomEmoji("📢")
	IconPolice    = CustomEmoji("👮")
	IconNotes     = CustomEmoji("📝")
	IconStopwatch = CustomEmoji("⏱️")
	IconInfinity  = CustomEmoji("♾️")
)

var (
	customEmojiRegex   = regexp.MustCompile(`<tg-emoji\s+emoji-id="[0-9]+">([^<]+)</tg-emoji>`)
	compiledEmojiRegex *regexp.Regexp
	compiledEmojiOnce  sync.Once
)

func CustomEmoji(emojiChar string) string {
	if id, ok := EmojiMapping[emojiChar]; ok && id != "" {
		return fmt.Sprintf(`<tg-emoji emoji-id="%s">%s</tg-emoji>`, id, emojiChar)
	}
	return emojiChar
}

func StripCustomEmojis(text string) string {
	return customEmojiRegex.ReplaceAllString(text, "$1")
}

func getEmojiRegex() *regexp.Regexp {
	compiledEmojiOnce.Do(func() {
		keys := make([]string, 0, len(EmojiMapping))
		for k := range EmojiMapping {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			return len(keys[i]) > len(keys[j])
		})
		var escaped []string
		for _, k := range keys {
			escaped = append(escaped, regexp.QuoteMeta(k))
		}
		pattern := strings.Join(escaped, "|")
		compiledEmojiRegex = regexp.MustCompile(pattern)
	})
	return compiledEmojiRegex
}

func ReplaceEmojis(text string) string {
	cleanText := StripCustomEmojis(text)
	re := getEmojiRegex()
	return re.ReplaceAllStringFunc(cleanText, func(match string) string {
		if id, ok := EmojiMapping[match]; ok && id != "" {
			return fmt.Sprintf(`<tg-emoji emoji-id="%s">%s</tg-emoji>`, id, match)
		}
		return match
	})
}

// ShouldUseCustomEmojis returns true if chat is a private DM (> 0) or VIP group (< 0 with active VIP)
func ShouldUseCustomEmojis(chatID int64) bool {
	if chatID == 0 {
		return false
	}
	// Direct message (DM) with user in private chat
	if chatID > 0 {
		return true
	}
	// Group / Channel -> only if VIP subscription is active
	return IsVIPChat(chatID)
}

// getChatIDFromChattable extracts chatID from any tgbotapi message/edit config
func getChatIDFromChattable(c tgbotapi.Chattable) int64 {
	switch v := c.(type) {
	case tgbotapi.MessageConfig:
		return v.ChatID
	case tgbotapi.EditMessageTextConfig:
		return v.ChatID
	case tgbotapi.EditMessageCaptionConfig:
		return v.ChatID
	case tgbotapi.VideoConfig:
		return v.ChatID
	case tgbotapi.PhotoConfig:
		return v.ChatID
	case tgbotapi.AnimationConfig:
		return v.ChatID
	case tgbotapi.DocumentConfig:
		return v.ChatID
	case tgbotapi.AudioConfig:
		return v.ChatID
	case tgbotapi.VoiceConfig:
		return v.ChatID
	default:
		return 0
	}
}

// SafeSend sends or edits a message with smart custom emoji handling (DM & VIP groups) and automatic fallback
func SafeSend(bot *tgbotapi.BotAPI, chattable tgbotapi.Chattable) (tgbotapi.Message, error) {
	applyEmojis := func(c tgbotapi.Chattable, replace bool) tgbotapi.Chattable {
		switch v := c.(type) {
		case tgbotapi.MessageConfig:
			if v.ParseMode == "HTML" || v.ParseMode == "" {
				v.ParseMode = "HTML"
				if replace {
					v.Text = ReplaceEmojis(v.Text)
				} else {
					v.Text = StripCustomEmojis(v.Text)
				}
				return v
			}
		case tgbotapi.EditMessageTextConfig:
			if v.ParseMode == "HTML" || v.ParseMode == "" {
				v.ParseMode = "HTML"
				if replace {
					v.Text = ReplaceEmojis(v.Text)
				} else {
					v.Text = StripCustomEmojis(v.Text)
				}
				return v
			}
		case tgbotapi.EditMessageCaptionConfig:
			if v.ParseMode == "HTML" || v.ParseMode == "" {
				v.ParseMode = "HTML"
				if replace {
					v.Caption = ReplaceEmojis(v.Caption)
				} else {
					v.Caption = StripCustomEmojis(v.Caption)
				}
				return v
			}
		case tgbotapi.VideoConfig:
			if v.ParseMode == "HTML" || v.ParseMode == "" {
				v.ParseMode = "HTML"
				if replace {
					v.Caption = ReplaceEmojis(v.Caption)
				} else {
					v.Caption = StripCustomEmojis(v.Caption)
				}
				return v
			}
		case tgbotapi.PhotoConfig:
			if v.ParseMode == "HTML" || v.ParseMode == "" {
				v.ParseMode = "HTML"
				if replace {
					v.Caption = ReplaceEmojis(v.Caption)
				} else {
					v.Caption = StripCustomEmojis(v.Caption)
				}
				return v
			}
		case tgbotapi.AnimationConfig:
			if v.ParseMode == "HTML" || v.ParseMode == "" {
				v.ParseMode = "HTML"
				if replace {
					v.Caption = ReplaceEmojis(v.Caption)
				} else {
					v.Caption = StripCustomEmojis(v.Caption)
				}
				return v
			}
		case tgbotapi.DocumentConfig:
			if v.ParseMode == "HTML" || v.ParseMode == "" {
				v.ParseMode = "HTML"
				if replace {
					v.Caption = ReplaceEmojis(v.Caption)
				} else {
					v.Caption = StripCustomEmojis(v.Caption)
				}
				return v
			}
		case tgbotapi.AudioConfig:
			if v.ParseMode == "HTML" || v.ParseMode == "" {
				v.ParseMode = "HTML"
				if replace {
					v.Caption = ReplaceEmojis(v.Caption)
				} else {
					v.Caption = StripCustomEmojis(v.Caption)
				}
				return v
			}
		case tgbotapi.VoiceConfig:
			if v.ParseMode == "HTML" || v.ParseMode == "" {
				v.ParseMode = "HTML"
				if replace {
					v.Caption = ReplaceEmojis(v.Caption)
				} else {
					v.Caption = StripCustomEmojis(v.Caption)
				}
				return v
			}
		}
		return c
	}

	chatID := getChatIDFromChattable(chattable)
	useCustom := ShouldUseCustomEmojis(chatID)

	// 1. Free group path: Send clean standard unicode emojis in 1 single call (0 overhead)
	if !useCustom {
		cleanMsg := applyEmojis(chattable, false)
		resp, err := bot.Request(cleanMsg)
		if err == nil {
			var msg tgbotapi.Message
			_ = json.Unmarshal(resp.Result, &msg)
			return msg, nil
		}
		return tgbotapi.Message{}, err
	}

	// 2. DM / VIP Group path: Try sending with premium custom emojis
	premiumMsg := applyEmojis(chattable, true)
	resp, err := bot.Request(premiumMsg)
	if err == nil {
		var msg tgbotapi.Message
		_ = json.Unmarshal(resp.Result, &msg)
		return msg, nil
	}

	// 3. Fallback: Telegram rejected custom emojis (e.g. DOCUMENT_INVALID) -> Send clean unicode message
	log.Printf("⚠️ SafeSend: Custom emojis rejected for chat %d (%v). Falling back to clean emojis.", chatID, err)
	fallbackMsg := applyEmojis(chattable, false)
	resp2, err2 := bot.Request(fallbackMsg)
	if err2 == nil {
		var msg tgbotapi.Message
		_ = json.Unmarshal(resp2.Result, &msg)
		return msg, nil
	}

	return tgbotapi.Message{}, err2
}

func ColoredNotice(statusType string, title string, details string) string {
	var prefix string
	switch statusType {
	case "success":
		prefix = "+"
	case "error", "ban":
		prefix = "-"
	case "warning":
		prefix = "!"
	default:
		prefix = "+"
	}
	return fmt.Sprintf(`<pre><code class="language-diff">%s [%s] %s</code></pre>`,
		prefix, html.EscapeString(title), html.EscapeString(details))
}
