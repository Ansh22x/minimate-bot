package handlers

import (
	"fmt"
	"html"
	"regexp"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Verified Public Custom Emoji IDs (derived from PokeEmpire & verified public Telegram packs)
var EmojiMapping = map[string]string{
	// Statuses & Shields
	"👑":  "5433758796289685818",
	"🛡️": "5251203410396458957",
	"🛡":  "5251203410396458957",
	"✨":  "5325547803936572038",
	"✨️": "5325547803936572038",
	"✅":  "5427009714745517609",
	"❌":  "5210952531676504517",
	"⚡":  "5411590687663608498",
	"⚡️": "5411590687663608498",
	"🔒":  "5465443379917629504",
	"⚠️":  "5420323339723881652",
	"ℹ️":  "6203791465471022369",
	"ℹ":   "6203791465471022369",
	"⚙️":  "5341715473882955310",
	"⚙":   "5341715473882955310",
	"📊":  "5231200819986047254",
	"🤖":  "5355051922862653659",
	"💎":  "5197350061012436657",
	"📜":  "5258500400918587241",
	"📌":  "5397782960512444700",
	"⏳":  "5269539162654010758",
	"⌛":  "5269539162654010758",
	"📣":  "5469903029144657419",
	"📢":  "5789428375261023681",
	"👤":  "5373012449597335010",
	"👥":  "5372926953978341366",
	"🔥":  "5424972470023104089",
	"⭐":  "5438496463044752972",
	"⭐️": "5438496463044752972",
	"💡":  "5323743114513373152",
	"🔗":  "5215288447190711367",
	"🔍":  "5231012545799666522",
	"🚫":  "5240241223632954241",
	"🛑":  "5341806819247401359",
	"🛠️": "5461047575379466857",
	"🛠":  "5461047575379466857",
	"🌸":  "5375525443552162306",
}

// Global Theme Emojis with custom tag formatting
var (
	IconCrown    = CustomEmoji("👑")
	IconShield   = CustomEmoji("🛡️")
	IconSparkles = CustomEmoji("✨")
	IconCheck    = CustomEmoji("✅")
	IconCross    = CustomEmoji("❌")
	IconFlower   = CustomEmoji("🌸")
	IconBolt     = CustomEmoji("⚡")
	IconLock     = CustomEmoji("🔒")
	IconWarning  = CustomEmoji("⚠️")
	IconBroom    = CustomEmoji("🧹")
	IconRobot    = CustomEmoji("🤖")
	IconPin      = CustomEmoji("📌")
	IconStats    = CustomEmoji("📊")
	IconGear     = CustomEmoji("⚙️")
)

// CustomEmoji converts an emoji to a Telegram custom emoji tag using verified IDs
func CustomEmoji(emoji string) string {
	if id, ok := EmojiMapping[emoji]; ok && id != "" {
		return fmt.Sprintf(`<tg-emoji emoji-id="%s">%s</tg-emoji>`, id, emoji)
	}
	return emoji
}

var tgEmojiRegex = regexp.MustCompile(`<tg-emoji[^>]*>(.*?)</tg-emoji>`)

// StripCustomEmojis removes tg-emoji wrapper tags if Telegram API raises parsing errors
func StripCustomEmojis(text string) string {
	return tgEmojiRegex.ReplaceAllString(text, "$1")
}

// ReplaceEmojis automatically scans any HTML string and transforms standard emojis into custom emojis
func ReplaceEmojis(text string) string {
	for emoji, id := range EmojiMapping {
		if strings.Contains(text, emoji) {
			tag := fmt.Sprintf(`<tg-emoji emoji-id="%s">%s</tg-emoji>`, id, emoji)
			text = strings.ReplaceAll(text, emoji, tag)
		}
	}
	return text
}

// SafeSend sends or edits a message with automatic fallback if custom emoji parsing fails
func SafeSend(bot *tgbotapi.BotAPI, chattable tgbotapi.Chattable) (tgbotapi.Message, error) {
	msg, err := bot.Send(chattable)
	if err != nil && (strings.Contains(err.Error(), "can't parse entities") || strings.Contains(err.Error(), "custom emoji")) {
		// Fallback: strip custom emoji tags and retry
		switch c := chattable.(type) {
		case tgbotapi.MessageConfig:
			c.Text = StripCustomEmojis(c.Text)
			return bot.Send(c)
		case tgbotapi.EditMessageTextConfig:
			c.Text = StripCustomEmojis(c.Text)
			return bot.Send(c)
		case tgbotapi.VideoConfig:
			c.Caption = StripCustomEmojis(c.Caption)
			return bot.Send(c)
		case tgbotapi.PhotoConfig:
			c.Caption = StripCustomEmojis(c.Caption)
			return bot.Send(c)
		case tgbotapi.EditMessageCaptionConfig:
			c.Caption = StripCustomEmojis(c.Caption)
			return bot.Send(c)
		}
	}
	return msg, err
}

// ColoredNotice helper using diff syntax highlighting for colored terminal cards
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
