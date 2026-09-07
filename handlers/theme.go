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

// Verified Public Custom Emoji IDs
var EmojiMapping = map[string]string{
	"👑":  "5433758796289685818",
	"🛡️": "5251203410396458957",
	"🛡":  "5251203410396458957",
	"✨":  "5325547803936572038",
	"✨️": "5325547803936572038",
	"🌟":  "5469741319330996757",
	"⭐":  "5438496463044752972",
	"⭐️": "5438496463044752972",
	"✅":  "5427009714745517609",
	"✔️": "5427009714745517609",
	"❌":  "5426990022328849767",
	"❎":  "5426990022328849767",
	"🌸":  "5375525443552162306",
	"🌺":  "5375525443552162306",
	"⚡":  "5445284980978629559",
	"⚡️": "5445284980978629559",
	"🔥":  "5425029094158917849",
	"🔒":  "5472097787438965706",
	"🔓":  "5472097787438965706",
	"⚠️": "5469903029144657419",
	"⚠️️": "5469903029144657419",
	"🚨":  "5469903029144657419",
	"🤖":  "5429197991992899022",
	"📌":  "5465223395895427227",
	"📍":  "5465223395895427227",
	"📊":  "5431736780883764835",
	"📈":  "5431736780883764835",
	"📉":  "5431736780883764835",
	"⚙️": "5472164874811352210",
	"⚙":   "5472164874811352210",
	"🔧":  "5472164874811352210",
	"🛠️": "5472164874811352210",
	"💎":  "5406631276042002796",
	"💍":  "5406631276042002796",
	"💬":  "5409006440209724128",
	"🗨️": "5409006440209724128",
	"👤":  "5373147822998708337",
	"👥":  "5373147822998708337",
	"⏱️": "5465451838880894084",
	"⏱":   "5465451838880894084",
	"⏳":  "5465451838880894084",
	"⏰":  "5465451838880894084",
	"🕒":  "5465451838880894084",
	"🚀":  "5461152000538680615",
	"✈️": "5461152000538680615",
	"🛸":  "5461152000538680615",
	"🎯":  "5438496463044752972",
	"🏆":  "5433758796289685818",
	"🥇":  "5433758796289685818",
	"🔔":  "5438496463044752972",
	"🔕":  "5469903029144657419",
	"📢":  "5409006440209724128",
	"📣":  "5409006440209724128",
	"💡":  "5469741319330996757",
	"🔍":  "5431736780883764835",
	"🔎":  "5431736780883764835",
	"📜":  "5449660075184508972",
	"📋":  "5449660075184508972",
	"📁":  "5449660075184508972",
	"📂":  "5449660075184508972",
	"📄":  "5449660075184508972",
	"📖":  "5449660075184508972",
	"🎫":  "5377599075237502153",
	"🏷️": "5235582317988171528",
	"🏷":   "5235582317988171528",
	"🔙":  "5400169738263352182",
	"🔄":  "6122764622509380932",
	"👋":  "5472354553527541051",
	"🔨":  "5453991094435997597",
	"🧹":  "5461047575379466857",
	"🌐":  "5812392946917445652",
	"🔇":  "5469903029144657419",
	"🗑️": "5461047575379466857",
	"🗑":  "5461047575379466857",
}

// Global Theme Emojis
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

var (
	customEmojiRegex    = regexp.MustCompile(`<tg-emoji\s+emoji-id="[0-9]+">([^<]+)</tg-emoji>`)
	compiledEmojiRegex  *regexp.Regexp
	compiledEmojiOnce   sync.Once
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

// SafeSend sends or edits a message with automatic custom emoji injection and fallback
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
		}
		return c
	}

	firstTry := applyEmojis(chattable, true)
	resp, err := bot.Request(firstTry)
	if err == nil {
		var msg tgbotapi.Message
		_ = json.Unmarshal(resp.Result, &msg)
		return msg, nil
	}

	log.Printf("⚠️ SafeSend initial attempt failed: %v. Retrying without custom emojis...", err)
	fallbackTry := applyEmojis(chattable, false)
	resp2, err2 := bot.Request(fallbackTry)
	if err2 == nil {
		var msg tgbotapi.Message
		_ = json.Unmarshal(resp2.Result, &msg)
		return msg, nil
	}
	log.Printf("❌ SafeSend fallback failed: %v", err2)
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
