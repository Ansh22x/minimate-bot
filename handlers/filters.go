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

// FilterItem represents a saved filter or note
type FilterItem struct {
	ReplyText string
	FileID    string
	MediaType string // "text", "photo", "video", "animation", "sticker", "document"
}

// filterCache stores group filters in RAM for 0ms latency checks
// Map structure: chatID -> keyword -> FilterItem
var (
	filterCache = make(map[int64]map[string]FilterItem)
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

	filterCache[chatID] = make(map[string]FilterItem)

	query := "SELECT keyword, reply_text, COALESCE(file_id, ''), COALESCE(media_type, 'text') FROM filters WHERE chat_id = $1"
	rows, err := database.Pool.Query(context.Background(), query, chatID)
	if err != nil {
		log.Printf("Failed to load filters for chat %d: %v", chatID, err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var keyword, reply, fileID, mediaType string
		if err := rows.Scan(&keyword, &reply, &fileID, &mediaType); err == nil {
			filterCache[chatID][keyword] = FilterItem{
				ReplyText: reply,
				FileID:    fileID,
				MediaType: mediaType,
			}
		}
	}
}

// sendFilterItem dispatches a saved filter item according to its media type
func sendFilterItem(bot *tgbotapi.BotAPI, chatID int64, item FilterItem, replyToID int) {
	switch item.MediaType {
	case "photo":
		photoMsg := tgbotapi.NewPhoto(chatID, tgbotapi.FileID(item.FileID))
		photoMsg.Caption = item.ReplyText
		photoMsg.ParseMode = "HTML"
		if replyToID != 0 {
			photoMsg.ReplyToMessageID = replyToID
		}
		SafeSend(bot, photoMsg)

	case "video":
		videoMsg := tgbotapi.NewVideo(chatID, tgbotapi.FileID(item.FileID))
		videoMsg.Caption = item.ReplyText
		videoMsg.ParseMode = "HTML"
		if replyToID != 0 {
			videoMsg.ReplyToMessageID = replyToID
		}
		SafeSend(bot, videoMsg)

	case "animation":
		animMsg := tgbotapi.NewAnimation(chatID, tgbotapi.FileID(item.FileID))
		animMsg.Caption = item.ReplyText
		animMsg.ParseMode = "HTML"
		if replyToID != 0 {
			animMsg.ReplyToMessageID = replyToID
		}
		SafeSend(bot, animMsg)

	case "sticker":
		stickerMsg := tgbotapi.NewSticker(chatID, tgbotapi.FileID(item.FileID))
		if replyToID != 0 {
			stickerMsg.ReplyToMessageID = replyToID
		}
		bot.Send(stickerMsg)

	case "document":
		docMsg := tgbotapi.NewDocument(chatID, tgbotapi.FileID(item.FileID))
		docMsg.Caption = item.ReplyText
		docMsg.ParseMode = "HTML"
		if replyToID != 0 {
			docMsg.ReplyToMessageID = replyToID
		}
		SafeSend(bot, docMsg)

	default: // "text"
		msg := tgbotapi.NewMessage(chatID, item.ReplyText)
		msg.ParseMode = "HTML"
		if replyToID != 0 {
			msg.ReplyToMessageID = replyToID
		}
		SafeSend(bot, msg)
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
	var matchedItem FilterItem
	var found bool
	text := strings.ToLower(message.Text)

	filterMutex.RLock()
	chatFilters := filterCache[chatID]
	for keyword, item := range chatFilters {
		// Checks if the keyword is exactly the text, or a standalone word in a sentence
		if text == keyword || strings.Contains(text, " "+keyword+" ") || strings.HasPrefix(text, keyword+" ") || strings.HasSuffix(text, " "+keyword) {
			matchedItem = item
			found = true
			break // Only trigger one filter per message to prevent spam
		}
	}
	filterMutex.RUnlock()

	if found {
		sendFilterItem(bot, chatID, matchedItem, message.MessageID)
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
			msg := tgbotapi.NewMessage(chatID, "❌ Only admins can manage filters and notes.")
			msg.ParseMode = "HTML"
			SafeSend(bot, msg)
			return
		}

		trimmedArgs := strings.TrimSpace(args)
		if trimmedArgs == "" {
			helpMsg := fmt.Sprintf(`❌ <b>Usage Guide:</b>

• <b>Text Filter:</b> <code>/%s &lt;keyword&gt; &lt;reply text&gt;</code>
• <b>Media Filter (Photo/Video/GIF/Sticker):</b> Reply to any media or message with <code>/%s &lt;keyword&gt;</code>`, html.EscapeString(cmd), html.EscapeString(cmd))
			msg := tgbotapi.NewMessage(chatID, helpMsg)
			msg.ParseMode = "HTML"
			SafeSend(bot, msg)
			return
		}

		var keyword, replyText, fileID, mediaType string
		mediaType = "text"

		// 1. Check if replying to another message
		if message.ReplyToMessage != nil {
			replied := message.ReplyToMessage

			parts := strings.SplitN(trimmedArgs, " ", 2)
			keyword = strings.ToLower(parts[0])
			if len(parts) > 1 {
				replyText = parts[1] // Custom caption provided with command
			}

			if len(replied.Photo) > 0 {
				mediaType = "photo"
				fileID = replied.Photo[len(replied.Photo)-1].FileID
				if replyText == "" {
					replyText = replied.Caption
				}
			} else if replied.Video != nil {
				mediaType = "video"
				fileID = replied.Video.FileID
				if replyText == "" {
					replyText = replied.Caption
				}
			} else if replied.Animation != nil {
				mediaType = "animation"
				fileID = replied.Animation.FileID
				if replyText == "" {
					replyText = replied.Caption
				}
			} else if replied.Sticker != nil {
				mediaType = "sticker"
				fileID = replied.Sticker.FileID
			} else if replied.Document != nil {
				mediaType = "document"
				fileID = replied.Document.FileID
				if replyText == "" {
					replyText = replied.Caption
				}
			} else if replied.Audio != nil {
				mediaType = "document"
				fileID = replied.Audio.FileID
				if replyText == "" {
					replyText = replied.Caption
				}
			} else if replied.Text != "" {
				mediaType = "text"
				if replyText == "" {
					replyText = replied.Text
				}
			} else if replied.Caption != "" {
				mediaType = "text"
				if replyText == "" {
					replyText = replied.Caption
				}
			}
		} else {
			// Not replying to a message: expect <keyword> <reply text>
			parts := strings.SplitN(trimmedArgs, " ", 2)
			if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
				helpMsg := fmt.Sprintf(`❌ <b>Usage:</b> <code>/%s &lt;keyword&gt; &lt;reply text&gt;</code>
<i>(Or reply to any photo, video, gif or sticker with <code>/%s &lt;keyword&gt;</code>)</i>`, html.EscapeString(cmd), html.EscapeString(cmd))
				msg := tgbotapi.NewMessage(chatID, helpMsg)
				msg.ParseMode = "HTML"
				SafeSend(bot, msg)
				return
			}
			keyword = strings.ToLower(parts[0])
			replyText = parts[1]
			mediaType = "text"
		}

		if keyword == "" {
			msg := tgbotapi.NewMessage(chatID, "❌ Please specify a valid keyword for this filter.")
			msg.ParseMode = "HTML"
			SafeSend(bot, msg)
			return
		}

		// PostgreSQL UPSERT logic (Insert or update if exists)
		query := `
			INSERT INTO filters (chat_id, keyword, reply_text, file_id, media_type) 
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (chat_id, keyword) 
			DO UPDATE SET 
				reply_text = EXCLUDED.reply_text,
				file_id = EXCLUDED.file_id,
				media_type = EXCLUDED.media_type;
		`
		_, err := database.Pool.Exec(context.Background(), query, chatID, keyword, replyText, fileID, mediaType)
		if err != nil {
			msg := tgbotapi.NewMessage(chatID, "❌ Database error while saving filter.")
			msg.ParseMode = "HTML"
			SafeSend(bot, msg)
			log.Printf("DB Save Error: %v", err)
			return
		}

		// Update RAM Cache instantly
		filterMutex.Lock()
		if filterCache[chatID] == nil {
			filterCache[chatID] = make(map[string]FilterItem)
		}
		filterCache[chatID][keyword] = FilterItem{
			ReplyText: replyText,
			FileID:    fileID,
			MediaType: mediaType,
		}
		filterMutex.Unlock()

		mediaLabel := strings.ToUpper(mediaType)
		if mediaType == "animation" {
			mediaLabel = "GIF"
		}
		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Saved filter <b>%s</b> [%s]!", html.EscapeString(keyword), mediaLabel))
		msg.ParseMode = "HTML"
		SafeSend(bot, msg)

	case "stop", "clear":
		if !isAdmin(bot, chatID, fromID) {
			msg := tgbotapi.NewMessage(chatID, "❌ Only admins can remove filters and notes.")
			msg.ParseMode = "HTML"
			SafeSend(bot, msg)
			return
		}
		keyword := strings.ToLower(strings.TrimSpace(args))
		if keyword == "" {
			msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Usage: <code>/%s &lt;keyword&gt;</code>", html.EscapeString(cmd)))
			msg.ParseMode = "HTML"
			SafeSend(bot, msg)
			return
		}

		query := "DELETE FROM filters WHERE chat_id = $1 AND keyword = $2"
		_, err := database.Pool.Exec(context.Background(), query, chatID, keyword)
		if err != nil {
			msg := tgbotapi.NewMessage(chatID, "❌ Failed to delete from database.")
			msg.ParseMode = "HTML"
			SafeSend(bot, msg)
			return
		}

		// Remove from RAM Cache
		filterMutex.Lock()
		if filterCache[chatID] != nil {
			delete(filterCache[chatID], keyword)
		}
		filterMutex.Unlock()

		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("🗑️ Deleted filter <b>%s</b>.", html.EscapeString(keyword)))
		msg.ParseMode = "HTML"
		SafeSend(bot, msg)

	case "filters", "notes":
		filterMutex.RLock()
		chatFilters := filterCache[chatID]
		keys := make([]string, 0, len(chatFilters))
		for k := range chatFilters {
			keys = append(keys, k)
		}
		filterMutex.RUnlock()

		if len(keys) == 0 {
			msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("ℹ️ No active %s in this group.", html.EscapeString(cmd)))
			msg.ParseMode = "HTML"
			SafeSend(bot, msg)
			return
		}

		var builder strings.Builder
		builder.WriteString(fmt.Sprintf("📝 <b>Active %s in this Chat:</b>\n\n", html.EscapeString(cmd)))
		for _, k := range keys {
			item := chatFilters[k]
			icon := "💬"
			switch item.MediaType {
			case "photo":
				icon = "🖼️"
			case "video":
				icon = "🎥"
			case "animation":
				icon = "🎬"
			case "sticker":
				icon = "👾"
			case "document":
				icon = "📦"
			}
			builder.WriteString(fmt.Sprintf("• %s <code>%s</code>\n", icon, html.EscapeString(k)))
		}

		msg := tgbotapi.NewMessage(chatID, builder.String())
		msg.ParseMode = "HTML"
		SafeSend(bot, msg)

	case "get":
		keyword := strings.ToLower(strings.TrimSpace(args))
		if keyword == "" {
			msg := tgbotapi.NewMessage(chatID, "❌ Usage: <code>/get &lt;notename&gt;</code>")
			msg.ParseMode = "HTML"
			SafeSend(bot, msg)
			return
		}

		filterMutex.RLock()
		item, exists := filterCache[chatID][keyword]
		filterMutex.RUnlock()

		if exists {
			sendFilterItem(bot, chatID, item, message.MessageID)
		} else {
			msg := tgbotapi.NewMessage(chatID, "❌ Filter or note not found.")
			msg.ParseMode = "HTML"
			SafeSend(bot, msg)
		}
	}
}
