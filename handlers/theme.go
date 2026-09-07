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
	// Statuses, Shields & Badges
	"👑":  "5433758796289685818",
	"🛡️": "5251203410396458957",
	"🛡":  "5251203410396458957",
	"✨":  "5325547803936572038",
	"✨️": "5325547803936572038",
	"🌟":  "5469741319330996757",
	"⭐":  "5438496463044752972",
	"⭐️": "5438496463044752972",
	"✅":  "5427009714745517609",
	"❌":  "5210952531676504517",
	"⚡":  "5411590687663608498",
	"⚡️": "5411590687663608498",
	"🔒":  "5465443379917629504",
	"🔓":  "5465443379917629504",
	"🔐":  "5465443379917629504",
	"⚠️":  "5420323339723881652",
	"ℹ️":  "6203791465471022369",
	"ℹ":   "6203791465471022369",
	"⚙️":  "5341715473882955310",
	"⚙":   "5341715473882955310",
	"📊":  "5231200819986047254",
	"📈":  "5282950412784117735",
	"🤖":  "5355051922862653659",
	"💎":  "5197350061012436657",
	"📜":  "5258500400918587241",
	"📌":  "5397782960512444700",
	"⏳":  "5269539162654010758",
	"⌛":  "5269539162654010758",
	"⏱️": "5382194935057372936",
	"⏱":   "5382194935057372936",
	"📣":  "5469903029144657419",
	"📢":  "5789428375261023681",
	"👤":  "5373012449597335010",
	"👥":  "5372926953978341366",
	"🗣️": "5370765563226236970",
	"🗣":  "5370765563226236970",
	"🔥":  "5424972470023104089",
	"💡":  "5323743114513373152",
	"🔗":  "5215288447190711367",
	"🔍":  "5231012545799666522",
	"🚫":  "5240241223632954241",
	"🛑":  "5341806819247401359",
	"⛔":  "5260293700088511294",
	"🛠️": "5461047575379466857",
	"🛠":  "5461047575379466857",
	"🌸":  "5375525443552162306",
	"🪙":  "5382164415019768638",
	"💰":  "5287231198098117669",
	"💳":  "5445353829304387411",
	"💸":  "5864068125112144897",
	"🏆":  "5188344996356448758",
	"🥇":  "5440539497383087970",
	"🥈":  "5447203607294265305",
	"🥉":  "5453902265922376865",
	"💼":  "5359785904535774578",
	"🎉":  "5461151367559141950",
	"🧬":  "5431884253518375171",
	"💥":  "5240492311716054039",
	"🔴":  "5411225014148014586",
	"🟢":  "5215522595922779944",
	"🔵":  "4965219701572503640",
	"🟣":  "5197368799954738967",
	"🟡":  "6005661956931850799",
	"⚪":  "5391014263852647327",
	"⚪️": "5391014263852647327",
	"📋":  "5877618313139327986",
	"❓":  "5436113877181941026",
	"✏️": "5213305971891248967",
	"✏":   "5213305971891248967",
	"💬":  "5224617957971206703",
	"🔀":  "5222151079080246525",
	"🧠":  "5377510010500699911",
	"💭":  "5411199759740325999",
	"💣":  "5454225015534805938",
	"🎰":  "5255765065096774716",
	"⚔️": "5453991094435997597",
	"⚔":   "5453991094435997597",
	"🎲":  "5280816565657300091",
	"🎯":  "5350460637182993292",
	"🤝":  "5357080225463149588",
	"📦":  "5449800250032143374",
	"🎁":  "5203996991054432397",
	"🛒":  "5312361253610475399",
	"📖":  "5449660075184508972",
	"🎫":  "5377599075237502153",
	"🏷️": "5235582317988171528",
	"🏷":   "5235582317988171528",
	"🎒":  "5409234219496907243",
	"📺":  "5371074616187969568",
	"🖼️": "5895427227528467580",
	"🖼":   "5895427227528467580",
	"🎬":  "5375464961822695044",
	"🎨":  "5431456208487716895",
	"📷":  "5235837920081887219",
	"📸":  "5235837920081887219",
	"➡️":  "5416117059207572332",
	"⬅️":  "5386806351248768717",
	"🔙":  "5400169738263352182",
	"🔄":  "6122764622509380932",
	"💀":  "5370971163310693562",
	"👾":  "5370869711888194012",
	"🏁":  "5411520005386806155",
	"🏃":  "5397809391741181485",
	"✉️": "5406631276042002796",
	"✉":   "5406631276042002796",
	"💾":  "5462956611033117422",
	"📝":  "5334882760735598374",
	"📡":  "5321304062715517873",
	"📤":  "5433614747381538714",
	"📥":  "5433811242135331842",
	"🗳️": "5359741159566484212",
	"🗳":   "5359741159566484212",
	"😀":  "5429197991992899022",
	"👋":  "5472354553527541051",
	"🔨":  "5453991094435997597",
	"🧹":  "5461047575379466857",
	"🌐":  "5812392946917445652",
	"🦧":  "6300910296760323319",
	"🧿":  "5426900601101374618",
	"🧮":  "5343545160015829015",
	"🔇":  "5469903029144657419",
	"🔈":  "5469903029144657419",
	"🔊":  "5469903029144657419",
	"🗑️": "5461047575379466857",
	"🗑":  "5461047575379466857",
	"🚷":  "5397809391741181485",
}

// Global Theme Emojis
var (
	IconCrown    = "👑"
	IconShield   = "🛡️"
	IconSparkles = "✨"
	IconCheck    = "✅"
	IconCross    = "❌"
	IconFlower   = "🌸"
	IconBolt     = "⚡"
	IconLock     = "🔒"
	IconWarning  = "⚠️"
	IconBroom    = "🧹"
	IconRobot    = "🤖"
	IconPin      = "📌"
	IconStats    = "📊"
	IconGear     = "⚙️"
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
	cleanText := StripCustomEmojis(text)
	for emoji, id := range EmojiMapping {
		if strings.Contains(cleanText, emoji) {
			tag := fmt.Sprintf(`<tg-emoji emoji-id="%s">%s</tg-emoji>`, id, emoji)
			cleanText = strings.ReplaceAll(cleanText, emoji, tag)
		}
	}
	return cleanText
}

// SafeSend sends or edits a message with automatic custom emoji injection and fallback
func SafeSend(bot *tgbotapi.BotAPI, chattable tgbotapi.Chattable) (tgbotapi.Message, error) {
	switch c := chattable.(type) {
	case tgbotapi.MessageConfig:
		if c.ParseMode == "HTML" || c.ParseMode == "" {
			c.ParseMode = "HTML"
			c.Text = ReplaceEmojis(c.Text)
			chattable = c
		}
	case tgbotapi.EditMessageTextConfig:
		if c.ParseMode == "HTML" || c.ParseMode == "" {
			c.ParseMode = "HTML"
			c.Text = ReplaceEmojis(c.Text)
			chattable = c
		}
	case tgbotapi.VideoConfig:
		if c.ParseMode == "HTML" || c.ParseMode == "" {
			c.ParseMode = "HTML"
			c.Caption = ReplaceEmojis(c.Caption)
			chattable = c
		}
	case tgbotapi.PhotoConfig:
		if c.ParseMode == "HTML" || c.ParseMode == "" {
			c.ParseMode = "HTML"
			c.Caption = ReplaceEmojis(c.Caption)
			chattable = c
		}
	case tgbotapi.EditMessageCaptionConfig:
		if c.ParseMode == "HTML" || c.ParseMode == "" {
			c.ParseMode = "HTML"
			c.Caption = ReplaceEmojis(c.Caption)
			chattable = c
		}
	}

	msg, err := bot.Send(chattable)
	if err != nil && (strings.Contains(err.Error(), "can't parse entities") || strings.Contains(err.Error(), "custom emoji") || strings.Contains(err.Error(), "entity_bounds_invalid")) {
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
