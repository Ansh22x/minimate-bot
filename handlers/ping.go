package handlers

import (
	"fmt"
	"html"
	"math"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// BotVersion current release tag
const BotVersion = "v.01"

// toSmallCaps converts ASCII letters to small capital Unicode characters
func toSmallCaps(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z':
			r = r + ('a' - 'A')
			fallthrough
		case r >= 'a' && r <= 'z':
			// Map ASCII a-z to small-caps
			smallCapsMap := []rune{
				'ᴀ', 'ʙ', 'ᴄ', 'ᴅ', 'ᴇ', 'ғ', 'ɢ', 'ʜ', 'ɪ', 'ᴊ', 'ᴋ', 'ʟ', 'ᴍ',
				'ɴ', 'ᴏ', 'ᴘ', 'ǫ', 'ʀ', 's', 'ᴛ', 'ᴜ', 'ᴠ', 'ᴡ', 'x', 'ʏ', 'ᴢ',
			}
			b.WriteRune(smallCapsMap[r-'a'])
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// formatUptime formats duration into "7d 14h 26m" or "2h 15m" format
func formatUptime(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

// SystemStats collects host, CPU, RAM, Disk, and runtime metrics
type SystemStats struct {
	CPUModel   string
	CPUUsage   float64
	CPUCores   int
	RAMUsedGB  float64
	RAMTotalGB float64
	RAMUsage   float64
	DiskUsedGB float64
	DiskTotGB  float64
	DiskFreeGB float64
	Uptime     string
	GoVersion  string
	Platform   string
	Hostname   string
	Overall    string
}

// getSystemMetrics gathers hardware and OS metrics safely
func getSystemMetrics() SystemStats {
	stats := SystemStats{
		CPUModel:  "Standard Processor",
		CPUCores:  runtime.NumCPU(),
		GoVersion: strings.TrimPrefix(runtime.Version(), "go"),
		Overall:   "🟢 ɴᴏʀᴍᴀʟ",
	}

	// 1. Host Info
	hInfo, err := host.Info()
	if err == nil && hInfo != nil {
		stats.Hostname = hInfo.Hostname
		if hInfo.Platform != "" {
			stats.Platform = fmt.Sprintf("%s %s", hInfo.Platform, hInfo.PlatformVersion)
		} else {
			stats.Platform = fmt.Sprintf("%s %s", hInfo.OS, hInfo.KernelVersion)
		}
	}
	if stats.Hostname == "" {
		if h, err := os.Hostname(); err == nil {
			stats.Hostname = h
		} else {
			stats.Hostname = "localhost"
		}
	}
	if stats.Platform == "" {
		stats.Platform = fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
	}

	// 2. CPU Info
	cpuInfos, err := cpu.Info()
	if err == nil && len(cpuInfos) > 0 {
		model := strings.TrimSpace(cpuInfos[0].ModelName)
		if model != "" {
			stats.CPUModel = model
		}
		if cpuInfos[0].Cores > 0 {
			stats.CPUCores = int(cpuInfos[0].Cores)
		}
	}

	// 3. CPU Usage
	cpuPercents, err := cpu.Percent(40*time.Millisecond, false)
	if err == nil && len(cpuPercents) > 0 {
		stats.CPUUsage = math.Round(cpuPercents[0]*10) / 10
	}

	// 4. Memory (RAM)
	vMem, err := mem.VirtualMemory()
	if err == nil && vMem != nil {
		stats.RAMTotalGB = float64(vMem.Total) / 1024 / 1024 / 1024
		stats.RAMUsedGB = float64(vMem.Used) / 1024 / 1024 / 1024
		stats.RAMUsage = math.Round(vMem.UsedPercent*10) / 10
	}

	// 5. Storage (Disk)
	diskPath := "/"
	if runtime.GOOS == "windows" {
		diskPath = "C:\\"
	}
	dUsage, err := disk.Usage(diskPath)
	if err != nil {
		// Fallback to current working directory
		dUsage, _ = disk.Usage(".")
	}
	if dUsage != nil {
		stats.DiskTotGB = float64(dUsage.Total) / 1024 / 1024 / 1024
		stats.DiskUsedGB = float64(dUsage.Used) / 1024 / 1024 / 1024
		stats.DiskFreeGB = float64(dUsage.Free) / 1024 / 1024 / 1024
	}

	// 6. Bot Uptime
	stats.Uptime = formatUptime(time.Since(botStartTime))

	// 7. Overall System Status
	if stats.CPUUsage >= 85.0 || stats.RAMUsage >= 90.0 {
		stats.Overall = "🔴 ʜɪɢʜ ʟᴏᴀᴅ"
	} else if stats.CPUUsage >= 60.0 || stats.RAMUsage >= 75.0 {
		stats.Overall = "🟡 ᴍᴏᴅᴇʀᴀᴛᴇ"
	} else {
		stats.Overall = "🟢 ɴᴏʀᴍᴀʟ"
	}

	return stats
}

// HandlePing renders the styled ping and system status report
func HandlePing(bot *tgbotapi.BotAPI, message *tgbotapi.Message, start time.Time) {
	chatID := message.Chat.ID

	// Measure API Roundtrip Latency
	apiStart := time.Now()
	initialMsg := tgbotapi.NewMessage(chatID, "🏓 <b>Pinging system...</b>")
	initialMsg.ParseMode = "HTML"
	sentMsg, err := SafeSend(bot, initialMsg)
	latency := time.Since(apiStart).Milliseconds()
	if latency <= 0 {
		latency = time.Since(start).Milliseconds()
	}

	// Determine Latency Rating
	var latencyRating string
	switch {
	case latency < 180:
		latencyRating = "🟢 ᴇxᴄᴇʟʟᴇɴᴛ"
	case latency <= 400:
		latencyRating = "🟡 ᴀᴠᴇʀᴀɢᴇ"
	default:
		latencyRating = "🔴 sʟᴏᴡ"
	}

	// Gather live system metrics
	stats := getSystemMetrics()

	// Determine bot name
	botName := "sᴘᴀᴍᴘʀᴏ"
	if bot != nil && bot.Self.FirstName != "" {
		botName = toSmallCaps(bot.Self.FirstName)
	}

	pingReport := formatPingReport(botName, latency, latencyRating, stats)

	if err == nil {
		editMsg := tgbotapi.NewEditMessageText(chatID, sentMsg.MessageID, pingReport)
		editMsg.ParseMode = "HTML"
		SafeSend(bot, editMsg)
	} else {
		newMsg := tgbotapi.NewMessage(chatID, pingReport)
		newMsg.ParseMode = "HTML"
		SafeSend(bot, newMsg)
	}
}

// formatPingReport constructs the stylized system status layout
func formatPingReport(botName string, latency int64, latencyRating string, stats SystemStats) string {
	return fmt.Sprintf(`╭━━〔 🤖 %s %s 〕━━╮
┃ 🏓 ᴘᴏɴɢ! sʏsᴛᴇᴍ sᴛᴀᴛᴜs
╰━━━━━━━━━━━━━━━━━━━━━━╯
⚡ ʀᴇsᴘᴏɴsᴇ
🏓 ʟᴀᴛᴇɴᴄʏ: %d ms %s
🟢 sᴛᴀᴛᴜs: ᴏɴʟɪɴᴇ
━━━━━━━━━━━━━━━━━━
🖥️ ʜᴏsᴛ sʏsᴛᴇᴍ
🧠 ᴄᴘᴜ: %s
📊 ᴄᴘᴜ ᴜsᴀɢᴇ: %.1f%%
🧵 ᴄᴏʀᴇs: %d

🎮 ɢᴘᴜ: ɴ/ᴀ
📈 ɢᴘᴜ ᴜsᴀɢᴇ: N/A%%
🌡️ ɢᴘᴜ ᴛᴇᴍᴘ: N/A°C
ℹ️ ɴᴏ ɢᴘᴜ ᴍᴏɴɪᴛᴏʀɪɴɢ ᴀᴠᴀɪʟᴀʙʟᴇ
━━━━━━━━━━━━━━━━━━
💾 ᴍᴇᴍᴏʀʏ &amp; sᴛᴏʀᴀɢᴇ
🧮 ʀᴀᴍ: %.2f / %.2f GB
📊 ʀᴀᴍ ᴜsᴀɢᴇ: %.1f%%

💽 sᴛᴏʀᴀɢᴇ: %.2f / %.2f GB
📦 ғʀᴇᴇ sᴘᴀᴄᴇ: %.2f GB
━━━━━━━━━━━━━━━━━━
⚙️ ʙᴏᴛ sʏsᴛᴇᴍ
⏱️ ᴜᴘᴛɪᴍᴇ: %s
🐹 ɢᴏʟᴀɴɢ: %s
🤖 ʙᴏᴛ ᴠᴇʀsɪᴏɴ: "%s"
🌐 ᴘʟᴀᴛғᴏʀᴍ: %s
━━━━━━━━━━━━━━━━━━
📡 ɴᴇᴛᴡᴏʀᴋ
🌐 ʜᴏsᴛ: %s
📡 ᴄᴏɴɴᴇᴄᴛɪᴏɴ: 🟢 "sᴛᴀʙʟᴇ"

📊 ᴏᴠᴇʀᴀʟʟ: %s

❝ ғᴀsᴛ ᴇɴᴏᴜɢʜ ᴛᴏ ʀᴇsᴘᴏɴᴅ. sᴛᴀʙʟᴇ ᴇɴᴏᴜɢʜ ᴛᴏ sᴛᴀʏ. ❞

🤖 %s %s`,
		botName, BotVersion,
		latency, latencyRating,
		html.EscapeString(stats.CPUModel), stats.CPUUsage, stats.CPUCores,
		stats.RAMUsedGB, stats.RAMTotalGB, stats.RAMUsage,
		stats.DiskUsedGB, stats.DiskTotGB, stats.DiskFreeGB,
		stats.Uptime, stats.GoVersion, BotVersion, html.EscapeString(stats.Platform),
		html.EscapeString(stats.Hostname),
		stats.Overall,
		botName, BotVersion,
	)
}
