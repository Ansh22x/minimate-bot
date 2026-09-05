package handlers

import (
	"context"
	"fmt"
	"html"
	"log"
	"strings"
	"sync"

	"minimate-bot/database"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// filterCache stores group filters in RAM for 0ms latency checks
// Map structure: chatID -> keyword -> reply
var (
	filterCache = make(map[int64]map[string]string)
	filterMutex sync.RWMutex
)

// loadFilters fetches a chat's filters from database and caches them
func loadFilters(chatID int64) {
	filterMutex.Lock()
	defer filterMutex.Unlock()

	// If already loaded by another concurrent thread, skip
	if _, exists := filterCache[chatID]; exists {
		return
	}

	filterCache[chatID] = make(map[string]string)

	query := "SELECT keyword, reply_text FROM filters WHERE chat_id = $1"
	rows, err := database.Pool.Query(context.Background(), query, chatID)
	if err != nil {
		log.Printf("Failed to load filters for chat %d: %v", chatID, err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var keyword, reply string
		if err := rows.Scan(&keyword, &reply); err == nil {
			filterCache[chatID][keyword] = reply
		}
	}
}

// handlePassiveFilters listens to standard chat messages to trigger automatic filter replies
func handlePassiveFilters(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	if message.Text == "" {
		return
	}

	chatID := message.Chat.ID

	// 1. Check if filters are cached for this chat
	filterMutex.RLock()
	_, exists := filterCache[chatID]
	filterMutex.RUnlock()

	// 2. Load from DB if missing
	if !exists {
		loadFilters(chatID)
	}

	// 3. Search matched filter while holding read lock to prevent concurrent iteration/write crashes
	var matchedReply string
	text := strings.ToLower(message.Text)

	filterMutex.RLock()
	chatFilters := filterCache[chatID]
	for keyword, reply := range chatFilters {
		// Checks if the keyword is exactly the text, or a standalone word in a sentence
		if text == keyword || strings.Contains(text, " "+keyword+" ") || strings.HasPrefix(text, keyword+" ") || strings.HasSuffix(text, " "+keyword) {
			matchedReply = reply
			break // Only trigger one filter per message to prevent spam
		}
	}
	filterMutex.RUnlock()

	if matchedReply != "" {
		bot.Send(tgbotapi.NewMessage(chatID, matchedReply))
	}
}

// HandleFilterCommand processes filter/note management commands
func HandleFilterCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message, cmd string, args string) {
	chatID := message.Chat.ID
	fromID := int64(0)
	if message.From != nil {
		fromID = message.From.ID
	} else if message.SenderChat != nil {
		fromID = message.SenderChat.ID
	}

	// Ensure cache is loaded so we can update it
	filterMutex.RLock()
	_, exists := filterCache[chatID]
	filterMutex.RUnlock()
	if !exists {
		loadFilters(chatID)
	}

	switch cmd {
	case "filter", "save":
		if !isAdmin(bot, chatID, fromID) {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Only admins can manage filters and notes."))
			return
		}

		parts := strings.SplitN(strings.TrimSpace(args), " ", 2)
		if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
			msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Usage: <code>/%s &lt;keyword&gt; &lt;reply text&gt;</code>", html.EscapeString(cmd)))
			msg.ParseMode = "HTML"
			bot.Send(msg)
			return
		}

		keyword := strings.ToLower(parts[0])
		replyText := parts[1]

		// PostgreSQL UPSERT logic (Insert, or update if it already exists)
		query := `
			INSERT INTO filters (chat_id, keyword, reply_text) 
			VALUES ($1, $2, $3)
			ON CONFLICT (chat_id, keyword) 
			DO UPDATE SET reply_text = EXCLUDED.reply_text;
		`
		_, err := database.Pool.Exec(context.Background(), query, chatID, keyword, replyText)
		if err != nil {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Database error while saving filter."))
			log.Printf("DB Save Error: %v", err)
			return
		}

		// Update RAM Cache instantly
		filterMutex.Lock()
		if filterCache[chatID] == nil {
			filterCache[chatID] = make(map[string]string)
		}
		filterCache[chatID][keyword] = replyText
		filterMutex.Unlock()

		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Saved <b>%s</b>!", html.EscapeString(keyword)))
		msg.ParseMode = "HTML"
		bot.Send(msg)

	case "stop", "clear":
		if !isAdmin(bot, chatID, fromID) {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Only admins can remove filters and notes."))
			return
		}
		keyword := strings.ToLower(strings.TrimSpace(args))
		if keyword == "" {
			msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Usage: <code>/%s &lt;keyword&gt;</code>", html.EscapeString(cmd)))
			msg.ParseMode = "HTML"
			bot.Send(msg)
			return
		}

		query := "DELETE FROM filters WHERE chat_id = $1 AND keyword = $2"
		_, err := database.Pool.Exec(context.Background(), query, chatID, keyword)
		if err != nil {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Failed to delete from database."))
			return
		}

		// Remove from RAM Cache
		filterMutex.Lock()
		if filterCache[chatID] != nil {
			delete(filterCache[chatID], keyword)
		}
		filterMutex.Unlock()

		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("🗑️ Deleted <b>%s</b>.", html.EscapeString(keyword)))
		msg.ParseMode = "HTML"
		bot.Send(msg)

	case "filters", "notes":
		filterMutex.RLock()
		chatFilters := filterCache[chatID]
		keys := make([]string, 0, len(chatFilters))
		for k := range chatFilters {
			keys = append(keys, k)
		}
		filterMutex.RUnlock()

		if len(keys) == 0 {
			bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("No active %s in this group.", html.EscapeString(cmd))))
			return
		}

		var builder strings.Builder
		builder.WriteString(fmt.Sprintf("📝 <b>Active %s:</b>\n\n", html.EscapeString(cmd)))
		for _, k := range keys {
			builder.WriteString(fmt.Sprintf("• <code>%s</code>\n", html.EscapeString(k)))
		}

		msg := tgbotapi.NewMessage(chatID, builder.String())
		msg.ParseMode = "HTML"
		bot.Send(msg)

	case "get":
		keyword := strings.ToLower(strings.TrimSpace(args))
		if keyword == "" {
			msg := tgbotapi.NewMessage(chatID, "❌ Usage: <code>/get &lt;notename&gt;</code>")
			msg.ParseMode = "HTML"
			bot.Send(msg)
			return
		}

		filterMutex.RLock()
		replyText, exists := filterCache[chatID][keyword]
		filterMutex.RUnlock()

		if exists {
			bot.Send(tgbotapi.NewMessage(chatID, replyText))
		} else {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Note not found."))
		}
	}
}