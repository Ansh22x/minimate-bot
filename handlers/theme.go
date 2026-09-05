package handlers

import (
	"fmt"
	"html"
)

// PremiumEmoji returns the clean emoji symbol.
// Note: Telegram Bot API rejects <tg-emoji id="..."> from bots unless the bot itself
// holds custom emoji rights from Fragment/BotFather. Using direct Unicode emojis
// guarantees instant delivery, video compatibility, and 0 entity parse errors across all clients.
func PremiumEmoji(customID string, fallback string) string {
	return fallback
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