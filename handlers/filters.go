package handlers

import (
	"context"
	"fmt"
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

// loadFilters fetches a chat's filters from Supabase and caches them
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

	// 3. Read filters from memory instantly
	filterMutex.RLock()
	chatFilters := filterCache[chatID]
	filterMutex.RUnlock()

	text := strings.ToLower(message.Text)

	for keyword, reply := range chatFilters {
		// Checks if the keyword is exactly the text, or a standalone word in a sentence
		if text == keyword || strings.Contains(text, " "+keyword+" ") || strings.HasPrefix(text, keyword+" ") || strings.HasSuffix(text, " "+keyword) {
			bot.Send(tgbotapi.NewMessage(chatID, reply))
			return // Only trigger one filter per message to prevent spam
		}
	}
}

// HandleFilterCommand processes filter/note management commands
func HandleFilterCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message, cmd string, args string) {
	chatID := message.Chat.ID

	// Ensure cache is loaded so we can update it
	filterMutex.RLock()
	_, exists := filterCache[chatID]
	filterMutex.RUnlock()
	if !exists {
		loadFilters(chatID)
	}

	switch cmd {
	case "filter", "save":
		if !isAdmin(bot, chatID, message.From.ID) {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Only admins can manage filters and notes."))
			return
		}

		parts := strings.SplitN(args, " ", 2)
		if len(parts) < 2 {
			bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Usage: `/%s <keyword> <reply text>`", cmd)))
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
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Database error while saving."))
			log.Printf("DB Save Error: %v", err)
			return
		}

		// Update RAM Cache instantly
		filterMutex.Lock()
		filterCache[chatID][keyword] = replyText
		filterMutex.Unlock()

		bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Saved **%s**!", keyword)))

	case "stop", "clear":
		if !isAdmin(bot, chatID, message.From.ID) {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Only admins can remove filters and notes."))
			return
		}
		if args == "" {
			bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Usage: `/%s <keyword>`", cmd)))
			return
		}

		keyword := strings.ToLower(args)

		query := "DELETE FROM filters WHERE chat_id = $1 AND keyword = $2"
		_, err := database.Pool.Exec(context.Background(), query, chatID, keyword)
		if err != nil {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Failed to delete from database."))
			return
		}

		// Remove from RAM Cache
		filterMutex.Lock()
		delete(filterCache[chatID], keyword)
		filterMutex.Unlock()

		bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("🗑️ Deleted **%s**.", keyword)))

	case "filters", "notes":
		filterMutex.RLock()
		chatFilters := filterCache[chatID]
		filterMutex.RUnlock()

		if len(chatFilters) == 0 {
			bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("No active %s in this group.", cmd)))
			return
		}

		var builder strings.Builder
		builder.WriteString(fmt.Sprintf("📝 **Active %s:**\n\n", cmd))
		for k := range chatFilters {
			builder.WriteString(fmt.Sprintf("• `%s`\n", k))
		}

		msg := tgbotapi.NewMessage(chatID, builder.String())
		msg.ParseMode = "Markdown"
		bot.Send(msg)

	case "get":
		if args == "" {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Usage: `/get <notename>`"))
			return
		}

		keyword := strings.ToLower(args)

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