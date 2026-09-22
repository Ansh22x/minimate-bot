package handlers

import (
	"encoding/json"
	"fmt"
	"html"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// CachedMessage stores message metadata in RAM for intelligent filtering and topic isolation
type CachedMessage struct {
	MessageID       int
	MessageThreadID int
	SenderID        int64
	SenderName      string
	SenderUsername  string
	IsSticker       bool
	IsMedia         bool
	IsBot           bool
	CreatedAt       time.Time
}

var (
	// chatID -> list of recent CachedMessage (up to 10,000 per chat)
	chatMsgCache    = make(map[int64][]CachedMessage)
	chatUserCache   = make(map[int64]map[string]*tgbotapi.User)
	chatUserIDCache = make(map[int64]map[int64]*tgbotapi.User)
	msgCacheMutex   sync.RWMutex

	// chatID -> msgID -> threadID (forum topics)
	msgThreadCache = make(map[int64]map[int]int)
	msgThreadMutex sync.RWMutex
)

// RegisterMessageThread stores thread ID for a message in forum topics
func RegisterMessageThread(chatID int64, msgID int, threadID int) {
	if chatID == 0 || msgID == 0 || threadID == 0 {
		return
	}
	msgThreadMutex.Lock()
	defer msgThreadMutex.Unlock()
	if msgThreadCache[chatID] == nil {
		msgThreadCache[chatID] = make(map[int]int)
	}
	msgThreadCache[chatID][msgID] = threadID
	if len(msgThreadCache[chatID]) > 10000 {
		for k := range msgThreadCache[chatID] {
			delete(msgThreadCache[chatID], k)
			if len(msgThreadCache[chatID]) <= 5000 {
				break
			}
		}
	}
}

// GetMessageThread returns thread ID for a message if known
func GetMessageThread(chatID int64, msgID int) int {
	if chatID == 0 || msgID == 0 {
		return 0
	}
	msgThreadMutex.RLock()
	defer msgThreadMutex.RUnlock()
	if msgThreadCache[chatID] == nil {
		return 0
	}
	return msgThreadCache[chatID][msgID]
}

// TrackMessage records incoming messages into the memory ring buffer with thread/topic tracking
func TrackMessage(msg *tgbotapi.Message) {
	if msg == nil || msg.Chat == nil {
		return
	}

	senderID := int64(0)
	senderName := ""
	senderUsername := ""
	isBot := false

	if msg.From != nil {
		senderID = msg.From.ID
		senderName = msg.From.FirstName
		senderUsername = strings.ToLower(msg.From.UserName)
		isBot = msg.From.IsBot
	} else if msg.SenderChat != nil {
		senderID = msg.SenderChat.ID
		senderName = msg.SenderChat.Title
		senderUsername = strings.ToLower(msg.SenderChat.UserName)
	}

	threadID := GetMessageThread(msg.Chat.ID, msg.MessageID)
	if threadID == 0 && msg.ReplyToMessage != nil {
		threadID = GetMessageThread(msg.Chat.ID, msg.ReplyToMessage.MessageID)
	}

	isSticker := msg.Sticker != nil
	isMedia := len(msg.Photo) > 0 || msg.Video != nil || msg.Audio != nil ||
		msg.Voice != nil || msg.Document != nil || msg.VideoNote != nil || msg.Animation != nil

	msgCacheMutex.Lock()
	defer msgCacheMutex.Unlock()

	// Update user lookup maps
	if msg.From != nil {
		if _, ok := chatUserCache[msg.Chat.ID]; !ok {
			chatUserCache[msg.Chat.ID] = make(map[string]*tgbotapi.User)
			chatUserIDCache[msg.Chat.ID] = make(map[int64]*tgbotapi.User)
		}
		if msg.From.UserName != "" {
			chatUserCache[msg.Chat.ID][strings.ToLower(msg.From.UserName)] = msg.From
		}
		chatUserIDCache[msg.Chat.ID][msg.From.ID] = msg.From
	}

	list := chatMsgCache[msg.Chat.ID]
	list = append(list, CachedMessage{
		MessageID:       msg.MessageID,
		MessageThreadID: threadID,
		SenderID:        senderID,
		SenderName:      senderName,
		SenderUsername:  senderUsername,
		IsSticker:       isSticker,
		IsMedia:         isMedia,
		IsBot:           isBot,
		CreatedAt:       time.Now(),
	})

	// Keep last 10,000 messages per chat for deep topic isolation
	if len(list) > 10000 {
		list = list[len(list)-10000:]
	}
	chatMsgCache[msg.Chat.ID] = list
}

// FindUserByUsername finds a user by their @username in a chat
func FindUserByUsername(chatID int64, username string) *tgbotapi.User {
	msgCacheMutex.RLock()
	defer msgCacheMutex.RUnlock()
	clean := strings.ToLower(strings.TrimPrefix(username, "@"))
	if m, ok := chatUserCache[chatID]; ok {
		if u, found := m[clean]; found {
			return u
		}
	}
	return nil
}

// FindUserByID finds a user by their numeric UserID in a chat
func FindUserByID(chatID int64, userID int64) *tgbotapi.User {
	msgCacheMutex.RLock()
	defer msgCacheMutex.RUnlock()
	if m, ok := chatUserIDCache[chatID]; ok {
		if u, found := m[userID]; found {
			return u
		}
	}
	return nil
}

// deleteMessagesBatch deletes message IDs concurrently with batch fallback and rate-limit backoff
func deleteMessagesBatch(bot *tgbotapi.BotAPI, chatID int64, messageIDs []int) int {
	if len(messageIDs) == 0 {
		return 0
	}

	// Deduplicate IDs and filter out zero/negative values
	seen := make(map[int]bool, len(messageIDs))
	uniqueIDs := make([]int, 0, len(messageIDs))
	for _, id := range messageIDs {
		if id > 0 && !seen[id] {
			seen[id] = true
			uniqueIDs = append(uniqueIDs, id)
		}
	}

	if len(uniqueIDs) == 0 {
		return 0
	}

	var totalDeleted int64
	batchSize := 100

	// Helper to attempt Telegram deleteMessages API (instant batch)
	tryDeleteBatch := func(chunk []int) bool {
		if len(chunk) == 0 {
			return false
		}
		jsonBytes, err := json.Marshal(chunk)
		if err != nil {
			return false
		}
		params := tgbotapi.Params{
			"chat_id":     strconv.FormatInt(chatID, 10),
			"message_ids": string(jsonBytes),
		}
		_, apiErr := bot.MakeRequest("deleteMessages", params)
		return apiErr == nil
	}

	// Helper to delete individual message with rate limit retry
	deleteSingleWithRetry := func(msgID int) bool {
		del := tgbotapi.NewDeleteMessage(chatID, msgID)
		for attempt := 0; attempt < 3; attempt++ {
			_, err := bot.Request(del)
			if err == nil {
				return true
			}
			errStr := strings.ToLower(err.Error())
			if strings.Contains(errStr, "too many requests") || strings.Contains(errStr, "retry after") {
				time.Sleep(350 * time.Millisecond)
				continue
			}
			// Not a rate limit (e.g. message already deleted or not found or older than 48h)
			break
		}
		return false
	}

	// Helper to delete a slice of IDs concurrently using parallel workers
	deleteConcurrently := func(ids []int) int {
		if len(ids) == 0 {
			return 0
		}
		var count int64
		var wg sync.WaitGroup
		idChan := make(chan int, len(ids))
		for _, id := range ids {
			idChan <- id
		}
		close(idChan)

		// 6 paced parallel workers
		numWorkers := 6
		if len(ids) < numWorkers {
			numWorkers = len(ids)
		}
		if numWorkers < 1 {
			numWorkers = 1
		}

		for w := 0; w < numWorkers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for msgID := range idChan {
					if deleteSingleWithRetry(msgID) {
						atomic.AddInt64(&count, 1)
					}
					time.Sleep(10 * time.Millisecond)
				}
			}()
		}
		wg.Wait()
		return int(count)
	}

	for i := 0; i < len(uniqueIDs); i += batchSize {
		end := i + batchSize
		if end > len(uniqueIDs) {
			end = len(uniqueIDs)
		}
		chunk := uniqueIDs[i:end]

		// 1. Try deleting entire 100-message chunk in 1 fast Telegram API request
		if tryDeleteBatch(chunk) {
			atomic.AddInt64(&totalDeleted, int64(len(chunk)))
			continue
		}

		// 2. If 100-message chunk failed (likely due to 1 missing/invalid message ID),
		// split into smaller 20-message sub-chunks
		subBatchSize := 20
		var remainingIDs []int
		for j := 0; j < len(chunk); j += subBatchSize {
			subEnd := j + subBatchSize
			if subEnd > len(chunk) {
				subEnd = len(chunk)
			}
			subChunk := chunk[j:subEnd]
			if tryDeleteBatch(subChunk) {
				atomic.AddInt64(&totalDeleted, int64(len(subChunk)))
			} else {
				// Sub-chunk failed, collect for individual concurrent deletion
				remainingIDs = append(remainingIDs, subChunk...)
			}
		}

		// 3. Delete any remaining failed sub-chunks concurrently with retry
		if len(remainingIDs) > 0 {
			c := deleteConcurrently(remainingIDs)
			atomic.AddInt64(&totalDeleted, int64(c))
		}
	}

	// Remove deleted messages from in-memory cache
	msgCacheMutex.Lock()
	if cachedList, exists := chatMsgCache[chatID]; exists {
		deletedMap := make(map[int]bool, len(uniqueIDs))
		for _, id := range uniqueIDs {
			deletedMap[id] = true
		}
		newList := make([]CachedMessage, 0, len(cachedList))
		for _, m := range cachedList {
			if !deletedMap[m.MessageID] {
				newList = append(newList, m)
			}
		}
		chatMsgCache[chatID] = newList
	}
	msgCacheMutex.Unlock()

	return int(totalDeleted)
}

// HandlePurge handles /purge, /purge all, /purge user, /purge stickers, /spurge, etc. with Topic awareness
func HandlePurge(bot *tgbotapi.BotAPI, message *tgbotapi.Message, cmd string, args string) {
	chatID := message.Chat.ID
	fromID := int64(0)
	if message.From != nil {
		fromID = message.From.ID
	} else if message.SenderChat != nil {
		fromID = message.SenderChat.ID
	}

	if !isAdmin(bot, chatID, fromID) {
		sendHTMLMessage(bot, chatID, "❌ Only administrators can purge messages.")
		return
	}

	cmdLower := strings.ToLower(cmd)
	argsClean := strings.TrimSpace(args)
	argsLower := strings.ToLower(argsClean)

	// Topic / Thread detection
	threadID := GetMessageThread(chatID, message.MessageID)
	if threadID == 0 && message.ReplyToMessage != nil {
		threadID = GetMessageThread(chatID, message.ReplyToMessage.MessageID)
	}
	isForumTopic := threadID != 0

	// Determine purge mode: "stickers", "user", "all"
	isStickersPurge := cmdLower == "purgestickers" || cmdLower == "purgesticker" || cmdLower == "stickerpurge" ||
		strings.HasPrefix(argsLower, "sticker") || strings.HasPrefix(argsLower, "stickers")
	isUserPurge := cmdLower == "spurge" || cmdLower == "userpurge" ||
		strings.HasPrefix(argsLower, "user") || strings.HasPrefix(argsLower, "@")
	isMePurge := cmdLower == "purgeme"

	// ----------------------------------------------------
	// 1. PURGE STICKERS ONLY (/purge stickers, /purgestickers)
	// ----------------------------------------------------
	if isStickersPurge {
		go func() {
			startID := 0
			if message.ReplyToMessage != nil {
				startID = message.ReplyToMessage.MessageID
			}

			limitCount := 100
			fields := strings.Fields(argsClean)
			for _, f := range fields {
				if n, err := strconv.Atoi(f); err == nil && n > 0 {
					limitCount = n
					break
				}
			}

			msgCacheMutex.RLock()
			cached := chatMsgCache[chatID]
			var idsToDelete []int

			// Always delete the purge command message first
			idsToDelete = append(idsToDelete, message.MessageID)

			if startID > 0 {
				for i := len(cached) - 1; i >= 0; i-- {
					m := cached[i]
					if m.MessageID >= startID && m.MessageID <= message.MessageID && m.IsSticker {
						// In forum topic, ensure message belongs to the current thread or general
						if !isForumTopic || m.MessageThreadID == threadID || m.MessageThreadID == 0 {
							idsToDelete = append(idsToDelete, m.MessageID)
						}
					}
				}
				// If replied message is a sticker and wasn't in cache
				if message.ReplyToMessage.Sticker != nil {
					found := false
					for _, id := range idsToDelete {
						if id == startID {
							found = true
							break
						}
					}
					if !found {
						idsToDelete = append(idsToDelete, startID)
					}
				}
			} else {
				// Purge recent stickers up to limitCount
				for i := len(cached) - 1; i >= 0 && len(idsToDelete) <= limitCount; i-- {
					m := cached[i]
					if m.IsSticker {
						if !isForumTopic || m.MessageThreadID == threadID || m.MessageThreadID == 0 {
							idsToDelete = append(idsToDelete, m.MessageID)
						}
					}
				}
			}
			msgCacheMutex.RUnlock()

			deleted := deleteMessagesBatch(bot, chatID, idsToDelete)
			stickerCount := deleted - 1
			if stickerCount < 0 {
				stickerCount = 0
			}

			confirmText := fmt.Sprintf("🎭 Purged <b>%d</b> sticker(s).", stickerCount)
			sent, err := sendHTMLMessage(bot, chatID, confirmText)
			if err == nil && sent.MessageID != 0 {
				time.Sleep(3500 * time.Millisecond)
				bot.Request(tgbotapi.NewDeleteMessage(chatID, sent.MessageID))
			}
		}()
		return
	}

	// ----------------------------------------------------
	// 2. PURGE SPECIFIC USER (/purge user, /spurge, /userpurge, /purgeme)
	// ----------------------------------------------------
	if isUserPurge || isMePurge {
		targetUserID := int64(0)
		targetName := ""

		if isMePurge {
			targetUserID = fromID
			targetName = "you"
		} else if message.ReplyToMessage != nil && message.ReplyToMessage.From != nil {
			targetUserID = message.ReplyToMessage.From.ID
			targetName = message.ReplyToMessage.From.FirstName
		} else if strings.HasPrefix(argsLower, "@") {
			targetUsername := strings.ToLower(strings.TrimPrefix(argsLower, "@"))
			u := FindUserByUsername(chatID, targetUsername)
			if u != nil {
				targetUserID = u.ID
				targetName = u.FirstName
			}
		}

		if targetUserID == 0 {
			sendHTMLMessage(bot, chatID, "❌ Reply to a user's message with <code>/purge user</code> or <code>/spurge</code> (or specify <code>/purge @username</code>) to purge their messages.")
			return
		}

		go func(uID int64, uName string) {
			startID := 0
			if message.ReplyToMessage != nil {
				startID = message.ReplyToMessage.MessageID
			}

			limitCount := 100
			fields := strings.Fields(argsClean)
			for _, f := range fields {
				if n, err := strconv.Atoi(f); err == nil && n > 0 {
					limitCount = n
					break
				}
			}

			msgCacheMutex.RLock()
			cached := chatMsgCache[chatID]
			var idsToDelete []int

			// Include the command message itself first
			idsToDelete = append(idsToDelete, message.MessageID)

			if startID > 0 {
				for i := len(cached) - 1; i >= 0; i-- {
					m := cached[i]
					if m.MessageID >= startID && m.MessageID <= message.MessageID && m.SenderID == uID {
						if !isForumTopic || m.MessageThreadID == threadID || m.MessageThreadID == 0 {
							idsToDelete = append(idsToDelete, m.MessageID)
						}
					}
				}
				// Guarantee replied message is included if from target user
				if message.ReplyToMessage != nil && message.ReplyToMessage.From != nil && message.ReplyToMessage.From.ID == uID {
					found := false
					for _, id := range idsToDelete {
						if id == startID {
							found = true
							break
						}
					}
					if !found {
						idsToDelete = append(idsToDelete, startID)
					}
				}
			} else {
				for i := len(cached) - 1; i >= 0 && len(idsToDelete) <= limitCount; i-- {
					m := cached[i]
					if m.SenderID == uID {
						if !isForumTopic || m.MessageThreadID == threadID || m.MessageThreadID == 0 {
							idsToDelete = append(idsToDelete, m.MessageID)
						}
					}
				}
			}
			msgCacheMutex.RUnlock()

			deleted := deleteMessagesBatch(bot, chatID, idsToDelete)
			userMsgCount := deleted - 1
			if userMsgCount < 0 {
				userMsgCount = 0
			}

			confirmText := fmt.Sprintf("🧹 Purged <b>%d</b> message(s) from <b>%s</b>.", userMsgCount, html.EscapeString(uName))
			sent, err := sendHTMLMessage(bot, chatID, confirmText)
			if err == nil && sent.MessageID != 0 {
				time.Sleep(3500 * time.Millisecond)
				bot.Request(tgbotapi.NewDeleteMessage(chatID, sent.MessageID))
			}
		}(targetUserID, targetName)
		return
	}

	// ----------------------------------------------------
	// 3. PURGE ALL / GENERAL PURGE (/purge, /purge all, /purge <count>)
	// ----------------------------------------------------
	var startID, endID int
	if message.ReplyToMessage != nil {
		startID = message.ReplyToMessage.MessageID
		endID = message.MessageID
	} else if argsClean != "" {
		// e.g. /purge 50 or /purge all 50
		count := 0
		fields := strings.Fields(argsClean)
		for _, f := range fields {
			if n, err := strconv.Atoi(f); err == nil && n > 0 {
				count = n
				break
			}
		}
		if count > 0 {
			if count > 1000 {
				count = 1000
			}
			startID = message.MessageID - count
			endID = message.MessageID
		}
	}

	if startID == 0 {
		sendHTMLMessage(bot, chatID, `❌ <b>Purge Usage:</b>
• <code>/purge</code> <i>(Reply to message)</i> — Purge all messages from reply to here
• <code>/purge all</code> <i>(Reply to message)</i> — Purge all messages in range
• <code>/purge &lt;count&gt;</code> — Purge last N messages (e.g. <code>/purge 30</code>)
• <code>/purge user</code> or <code>/spurge</code> <i>(Reply to user)</i> — Purge only that user's messages
• <code>/purge stickers</code> — Purge all stickers in range
• <code>/purgeme</code> — Purge your own recent messages`)
		return
	}

	if startID > endID {
		startID, endID = endID, startID
	}

	go func(sID, eID int) {
		var idsToDelete []int

		// Always delete the purge command message first
		idsToDelete = append(idsToDelete, eID)

		if isForumTopic && threadID != 0 {
			// IN FORUM TOPICS: Strictly filter by active threadID from cache to avoid deleting other topics' messages
			msgCacheMutex.RLock()
			cached := chatMsgCache[chatID]
			for i := len(cached) - 1; i >= 0; i-- {
				m := cached[i]
				if m.MessageID >= sID && m.MessageID < eID {
					if m.MessageThreadID == threadID || m.MessageThreadID == 0 {
						idsToDelete = append(idsToDelete, m.MessageID)
					}
				}
			}
			msgCacheMutex.RUnlock()

			// Ensure replied start message is included
			foundStart := false
			for _, id := range idsToDelete {
				if id == sID {
					foundStart = true
					break
				}
			}
			if !foundStart {
				idsToDelete = append(idsToDelete, sID)
			}
		} else {
			// IN STANDARD GROUPS (Single timeline): Delete all IDs in descending order
			// First check if count mode can be accurately satisfied from cache
			for id := eID - 1; id >= sID; id-- {
				idsToDelete = append(idsToDelete, id)
			}
		}

		deleted := deleteMessagesBatch(bot, chatID, idsToDelete)
		msgCount := deleted - 1
		if msgCount < 0 {
			msgCount = 0
		}

		confirmText := fmt.Sprintf("🧹 Purged <b>%d</b> message(s).", msgCount)
		sent, err := sendHTMLMessage(bot, chatID, confirmText)
		if err == nil && sent.MessageID != 0 {
			time.Sleep(3500 * time.Millisecond)
			bot.Request(tgbotapi.NewDeleteMessage(chatID, sent.MessageID))
		}
	}(startID, endID)
}
