package handlers

import (
	"strings"
	"testing"
)

func TestReplaceEmojis(t *testing.T) {
	input := "👑 <b>MiniMate Pro</b> 🌸 ✨ 🛡️ ⚡ ✅ ❌"
	output := ReplaceEmojis(input)

	if !strings.Contains(output, `<tg-emoji emoji-id="5433758796289685818">👑</tg-emoji>`) {
		t.Errorf("Expected crown custom emoji tag, got: %s", output)
	}
	if !strings.Contains(output, `<tg-emoji emoji-id="5375525443552162306">🌸</tg-emoji>`) {
		t.Errorf("Expected flower custom emoji tag, got: %s", output)
	}
	if !strings.Contains(output, `<tg-emoji emoji-id="5325547803936572038">✨</tg-emoji>`) {
		t.Errorf("Expected sparkles custom emoji tag, got: %s", output)
	}
	if !strings.Contains(output, `<tg-emoji emoji-id="5251203410396458957">🛡️</tg-emoji>`) {
		t.Errorf("Expected shield custom emoji tag, got: %s", output)
	}

	// Test idempotency: Calling ReplaceEmojis twice should NOT produce nested tags like <tg-emoji><tg-emoji>
	secondPass := ReplaceEmojis(output)
	if strings.Contains(secondPass, "<tg-emoji><tg-emoji") || strings.Contains(secondPass, "</tg-emoji></tg-emoji>") {
		t.Errorf("Found nested tags on second pass: %s", secondPass)
	}
	if secondPass != output {
		t.Errorf("Second pass did not match first pass!\nFirst:  %s\nSecond: %s", output, secondPass)
	}
}

func TestAllTabLengths(t *testing.T) {
	tabs := map[string]string{
		"tab_home": `╭━━━━━━━━━━━━━━━━━━━━━━╮
   🌸 <b>𝐌𝐢𝐧𝐢𝐌𝐚𝐭𝐞 𝐏𝐫𝐨</b> 🌸
╰━━━━━━━━━━━━━━━━━━━━━━╯

👋 Hey, <b>Ansh</b>!

<blockquote expandable>🤖 <b>Next-Gen Telegram Group Management</b>
⚡ Fast • Reliable • Zero-Latency
🛡️ Next-Gen Anti-Raid & Security Locks
✨ Premium Animated UI & Smart Math Captcha</blockquote>

👇 <i>Click any category tab to view commands:</i>`,

		"tab_member_cmds": `👥 <b>𝐆𝐞𝐧𝐞𝐫𝐚𝐥 & 𝐌𝐞𝐦𝐛𝐞𝐫 𝐂𝐨𝐦𝐦𝐚𝐧𝐝𝐬</b>

<blockquote expandable>• <code>/start</code> — Main bot menu
• <code>/ping</code> — Latency & status
• <code>/help</code> — Commands directory
• <code>/id</code> / <code>/info</code> — User & chat ID
• <code>/rules</code> — Group rules (or <code>/privaterules</code>)
• <code>/warns</code> — Check warning count
• <code>/filters</code> — Group auto-replies
• <code>/notes</code> / <code>/get &lt;name&gt;</code> — Saved notes
• <code>/premium</code> — VIP status & expiry</blockquote>`,

		"tab_admin_mod": `🔨 <b>𝐌𝐨𝐝𝐞𝐫𝐚𝐭𝐢𝐨𝐧 & 𝐏𝐮𝐧𝐢𝐬𝐡𝐦𝐞𝐧𝐭𝐬</b>
<i>(Reply to a user to execute)</i>

<blockquote expandable>• <code>/ban</code> / <code>/unban</code> — Permanent ban / unban
• <code>/tban &lt;time&gt;</code> — Temp-ban (e.g. <code>/tban 2h</code>)
• <code>/kick</code> — Kick user from group
• <code>/mute</code> / <code>/unmute</code> — Permanent mute / unmute
• <code>/tmute &lt;time&gt;</code> — Temp-mute (e.g. <code>/tmute 30m</code>)
• <code>/warn</code> / <code>/dwarn</code> — Strike (3 = auto-ban)
• <code>/unwarn</code> / <code>/rmwarns</code> — Remove / reset warnings
• <code>/promote</code> / <code>/demote</code> — Promote / demote admin</blockquote>`,

		"tab_admin_locks": `🛡️ <b>𝐒𝐞𝐜𝐮𝐫𝐢𝐭𝐲 𝐋𝐨𝐜𝐤𝐬 & 𝐂𝐚𝐩𝐭𝐜𝐡𝐚</b>

<blockquote expandable><b>🔐 Content Locks:</b>
• <code>/lock &lt;type&gt;</code> — <code>links</code>, <code>forwards</code>, <code>stickers</code>, <code>media</code>, <code>bots</code>, <code>all</code>
• <code>/unlock &lt;type&gt;</code> — Unlock specified type
• <code>/locks</code> — View active group locks
• <code>/locktypes</code> — List all lock types

<b>🤖 Smart Captcha:</b>
• <code>/captcha &lt;on/off&gt;</code> — Toggle join verification
• <code>/captchamode &lt;button|math&gt;</code> — Set mode
• <code>/captchatime &lt;sec&gt;</code> — Set timeout (30-600s)</blockquote>`,

		"tab_admin_tools": `🧹 <b>𝐂𝐡𝐚𝐭 𝐓𝐨𝐨𝐥𝐬, 𝐂𝐥𝐞𝐚𝐧𝐮𝐩 & 𝐆𝐫𝐞𝐞𝐭𝐢𝐧𝐠𝐬</b>

<blockquote expandable><b>🧹 Tools & Cleanup:</b>
• <code>/purge</code> / <code>/del</code> — Mass / single delete
• <code>/pin</code> / <code>/unpin</code> / <code>/unpinall</code> — Message pinning

<b>🌸 Greetings & Rules:</b>
• <code>/welcome &lt;on/off&gt;</code>, <code>/setwelcome</code>, <code>/rmwelcome</code>
• <code>/goodbye &lt;on/off&gt;</code>, <code>/setgoodbye</code>, <code>/rmgoodbye</code>
• <code>/setrules</code> / <code>/clearrules</code> — Manage rules
• <code>/filter &lt;word&gt; &lt;reply&gt;</code> / <code>/stop</code> — Auto-replies</blockquote>`,

		"tab_vip": `👑 <b>𝐕𝐈𝐏 𝐏𝐫𝐞𝐦𝐢𝐮𝐦 𝐒𝐮𝐛𝐬𝐜𝐫𝐢𝐩𝐭𝐢𝐨𝐧</b>

<blockquote expandable><b>💎 Premium Features:</b>
• ⚡ <b>Zero-Latency Engine:</b> Instant filter execution
• 🛡️ <b>Anti-Raid Shield:</b> High-speed join flood defense
• 🎨 <b>Custom Greeting Graphics:</b> Banner cards
• 📊 <b>Unlimited Limits:</b> Unlimited filters & notes

<b>🔍 Check Subscription:</b>
• Use <code>/premium</code> to check status & expiry.

💬 <i>Contact @Royal_Rahul_00 to activate VIP!</i></blockquote>`,

		"tab_info": `ℹ️ <b>𝐁𝐨𝐭 𝐒𝐭𝐚𝐭𝐮𝐬 & 𝐈𝐧𝐟𝐨</b>

<blockquote expandable>🤖 <b>Bot:</b> @MiniMateBot
⏱️ <b>Uptime:</b> 2h 15m
⚡ <b>Engine:</b> Go (Golang) + PostgreSQL
🛡️ <b>Security:</b> Anti-Raid Shield Active
✅ <b>Status:</b> All systems operational</blockquote>`,
	}

	for name, text := range tabs {
		replaced := ReplaceEmojis(text)
		t.Logf("[%s] Raw: %d chars, Replaced: %d chars", name, len(text), len(replaced))
		if len(replaced) > 1024 {
			t.Errorf("[%s] EXCEEDS Telegram 1024 caption limit! (%d chars)", name, len(replaced))
		}
	}
}
