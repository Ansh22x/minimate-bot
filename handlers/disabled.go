package handlers

import (
	"context"
	"fmt"
	"html"
	"strings"
	"sync"

	"minimate-bot/database"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var (
	disabledCmdsCache = make(map[int64]map[string]bool)
	disabledCmdsMutex sync.RWMutex
	disabledLoaded    = make(map[int64]bool)
)

// List of protected commands that CANNOT be disabled
var nonDisableableCommands = map[string]bool{
	"admin":         true,
	"adminlist":     true,
	"admins":        true,
	"staff":         true,
	"enable":        true,
	"disable":       true,
	"disabled":      true,
	"disables":      true,
	"disableable":   true,
	"enableall":     true,
	"help":          true,
	"commands":      true,
	"start":         true,
	"setvip":        true,
	"rmvip":         true,
	"viplist":       true,
	"vips":          true,
	"vip":           true,
	"vipstatus":     true,
	"premium":       true,
	"owner":         true,
	"creator":       true,
	"dashboard":     true,
	"stats":         true,
	"spam":          true,
	"chats":         true,
	"groups":        true,
	"addbotadmin":   true,
	"rmbotadmin":    true,
	"botadmins":     true,
	"sudolist":      true,
	"sudos":         true,
	"leave":         true,
	"kickme":        true,
	"broadcast":     true,
	"gcast":         true,
	"post":          true,
}

// LoadDisabledCommands loads the disabled commands for a chat from DB into RAM
func loadDisabledCommands(chatID int64) {
	disabledCmdsMutex.Lock()
	defer disabledCmdsMutex.Unlock()

	if disabledLoaded[chatID] {
		return
	}

	if disabledCmdsCache[chatID] == nil {
		disabledCmdsCache[chatID] = make(map[string]bool)
	}

	rows, err := database.Pool.Query(context.Background(),
		"SELECT command FROM chat_disabled_commands WHERE chat_id = $1", chatID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var cmd string
			if err := rows.Scan(&cmd); err == nil {
				disabledCmdsCache[chatID][strings.ToLower(cmd)] = true
			}
		}
		disabledLoaded[chatID] = true
	}
}

// IsCommandDisabled checks if a given command is currently disabled in the chat
func IsCommandDisabled(chatID int64, cmd string) bool {
	if chatID >= 0 {
		return false
	}

	cmdLower := strings.ToLower(strings.TrimPrefix(cmd, "/"))
	if nonDisableableCommands[cmdLower] {
		return false
	}

	disabledCmdsMutex.RLock()
	loaded := disabledLoaded[chatID]
	if loaded {
		isDisabled := disabledCmdsCache[chatID][cmdLower]
		disabledCmdsMutex.RUnlock()
		return isDisabled
	}
	disabledCmdsMutex.RUnlock()

	loadDisabledCommands(chatID)

	disabledCmdsMutex.RLock()
	defer disabledCmdsMutex.RUnlock()
	return disabledCmdsCache[chatID][cmdLower]
}

// HandleDisableCommand processes /disable, /enable, /disabled, /disableable, /enableall
func HandleDisableCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message, cmd string, args string) {
	chatID := message.Chat.ID
	fromID := int64(0)
	if message.From != nil {
		fromID = message.From.ID
	} else if message.SenderChat != nil {
		fromID = message.SenderChat.ID
	}

	cmdLower := strings.ToLower(cmd)
	argsClean := strings.TrimSpace(args)

	switch cmdLower {
	case "disable":
		if !isAdmin(bot, chatID, fromID) {
			sendHTMLMessage(bot, chatID, "❌ Only administrators can disable commands.")
			return
		}

		if argsClean == "" {
			sendHTMLMessage(bot, chatID, `❌ <b>Usage:</b> <code>/disable &lt;command&gt;</code>
<i>Example:</i> <code>/disable ping</code> or <code>/disable rules</code>
<i>Use <code>/disabled</code> to view currently disabled commands.</i>`)
			return
		}

		targetCmd := strings.ToLower(strings.TrimPrefix(argsClean, "/"))
		targetCmd = strings.Fields(targetCmd)[0]

		if nonDisableableCommands[targetCmd] {
			sendHTMLMessage(bot, chatID, fmt.Sprintf("⚠️ <b>/%s</b> is a core system command and cannot be disabled.", html.EscapeString(targetCmd)))
			return
		}

		// Insert into DB
		query := `
			INSERT INTO chat_disabled_commands (chat_id, command, disabled_at)
			VALUES ($1, $2, NOW())
			ON CONFLICT (chat_id, command) DO NOTHING;
		`
		_, err := database.Pool.Exec(context.Background(), query, chatID, targetCmd)
		if err != nil {
			sendHTMLMessage(bot, chatID, "❌ Database error while disabling command.")
			return
		}

		disabledCmdsMutex.Lock()
		if disabledCmdsCache[chatID] == nil {
			disabledCmdsCache[chatID] = make(map[string]bool)
		}
		disabledCmdsCache[chatID][targetCmd] = true
		disabledLoaded[chatID] = true
		disabledCmdsMutex.Unlock()

		sendHTMLMessage(bot, chatID, fmt.Sprintf("🚫 Disabled command <code>/%s</code> for regular members in this chat.\n\n<i>Note: Administrators can still execute disabled commands.</i>", html.EscapeString(targetCmd)))

	case "enable":
		if !isAdmin(bot, chatID, fromID) {
			sendHTMLMessage(bot, chatID, "❌ Only administrators can enable commands.")
			return
		}

		if argsClean == "" {
			sendHTMLMessage(bot, chatID, `❌ <b>Usage:</b> <code>/enable &lt;command&gt;</code>
<i>Example:</i> <code>/enable ping</code>
<i>Use <code>/disabled</code> to view currently disabled commands.</i>`)
			return
		}

		targetCmd := strings.ToLower(strings.TrimPrefix(argsClean, "/"))
		targetCmd = strings.Fields(targetCmd)[0]

		query := "DELETE FROM chat_disabled_commands WHERE chat_id = $1 AND command = $2"
		_, err := database.Pool.Exec(context.Background(), query, chatID, targetCmd)
		if err != nil {
			sendHTMLMessage(bot, chatID, "❌ Database error while enabling command.")
			return
		}

		disabledCmdsMutex.Lock()
		if disabledCmdsCache[chatID] != nil {
			delete(disabledCmdsCache[chatID], targetCmd)
		}
		disabledCmdsMutex.Unlock()

		sendHTMLMessage(bot, chatID, fmt.Sprintf("✅ Re-enabled command <code>/%s</code> for all members in this chat.", html.EscapeString(targetCmd)))

	case "enableall":
		if !isAdmin(bot, chatID, fromID) {
			sendHTMLMessage(bot, chatID, "❌ Only administrators can enable all commands.")
			return
		}

		query := "DELETE FROM chat_disabled_commands WHERE chat_id = $1"
		_, err := database.Pool.Exec(context.Background(), query, chatID)
		if err != nil {
			sendHTMLMessage(bot, chatID, "❌ Database error while enabling all commands.")
			return
		}

		disabledCmdsMutex.Lock()
		disabledCmdsCache[chatID] = make(map[string]bool)
		disabledLoaded[chatID] = true
		disabledCmdsMutex.Unlock()

		sendHTMLMessage(bot, chatID, "✅ All previously disabled commands have been re-enabled for this chat.")

	case "disabled", "disables", "disabledlist":
		loadDisabledCommands(chatID)

		disabledCmdsMutex.RLock()
		cmdsMap := disabledCmdsCache[chatID]
		var list []string
		for c, disabled := range cmdsMap {
			if disabled {
				list = append(list, c)
			}
		}
		disabledCmdsMutex.RUnlock()

		if len(list) == 0 {
			sendHTMLMessage(bot, chatID, "✅ <b>No commands are currently disabled in this chat.</b>\n\n<i>Use <code>/disable &lt;cmd&gt;</code> to disable a command.</i>")
			return
		}

		var b strings.Builder
		b.WriteString("🚫 <b>Disabled Commands in this Chat:</b>\n\n<blockquote expandable>")
		for _, c := range list {
			b.WriteString(fmt.Sprintf("• <code>/%s</code>\n", html.EscapeString(c)))
		}
		b.WriteString("</blockquote>\n💡 <i>Use <code>/enable &lt;cmd&gt;</code> or <code>/enableall</code> to re-enable.</i>")
		sendHTMLMessage(bot, chatID, b.String())

	case "disableable", "disableablelist":
		text := `📋 <b>Disableable Commands Reference</b>

<blockquote expandable><b>General &amp; Info:</b>
<code>ping</code>, <code>info</code>, <code>id</code>, <code>rules</code>, <code>privaterules</code>, <code>warns</code>

<b>Filters &amp; Notes:</b>
<code>filter</code>, <code>filters</code>, <code>stop</code>, <code>notes</code>, <code>get</code>, <code>save</code>, <code>clear</code>

<b>Chat Tools:</b>
<code>purge</code>, <code>spurge</code>, <code>purgestickers</code>, <code>purgeme</code>, <code>del</code>, <code>pin</code>, <code>unpin</code>, <code>unpinall</code>

<b>Greetings &amp; Locks:</b>
<code>welcome</code>, <code>goodbye</code>, <code>lock</code>, <code>unlock</code>, <code>locks</code>, <code>locktypes</code>, <code>captcha</code>

<b>Moderation:</b>
<code>warn</code>, <code>dwarn</code>, <code>unwarn</code>, <code>rmwarn</code>, <code>rmwarns</code>, <code>resetwarns</code>, <code>ban</code>, <code>tban</code>, <code>unban</code>, <code>kick</code>, <code>mute</code>, <code>tmute</code>, <code>unmute</code>, <code>promote</code>, <code>demote</code>, <code>title</code></blockquote>

💡 <i>Use <code>/disable &lt;command&gt;</code> to turn any of these off for regular members.</i>`
		sendHTMLMessage(bot, chatID, text)
	}
}
