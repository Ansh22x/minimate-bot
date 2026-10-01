// Package services: Anti-Raid Engine with join velocity monitoring and emergency lockdown.
package services

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"minimate-bot/config"
	"minimate-bot/database"
	"minimate-bot/models"
	"minimate-bot/workers"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type joinEvent struct {
	Timestamp time.Time
	UserID    int64
	Username  string
}

var (
	// chatID -> list of recent join events
	joinTracker   = make(map[int64][]joinEvent)
	joinTrackerMu sync.Mutex

	// chatID -> bool (is currently in active raid lockdown)
	activeRaidLockdowns   = make(map[int64]bool)
	activeRaidLockdownsMu sync.RWMutex
)

// RecordJoin registers a new member join and checks for raid velocity.
// Returns true if a raid was detected and lockdown activated.
func RecordJoin(bot *tgbotapi.BotAPI, chatID int64, user *tgbotapi.User) bool {
	if chatID >= 0 || user == nil {
		return false
	}

	settings := GetGroupSettings(chatID)
	if !settings.AntiRaidEnabled {
		return false
	}

	now := time.Now()
	joinTrackerMu.Lock()
	if joinTracker[chatID] == nil {
		joinTracker[chatID] = make([]joinEvent, 0)
	}

	// Clean older than 2 minutes
	cutoff := now.Add(-2 * time.Minute)
	var activeJoins []joinEvent
	for _, je := range joinTracker[chatID] {
		if je.Timestamp.After(cutoff) {
			activeJoins = append(activeJoins, je)
		}
	}

	currentJoin := joinEvent{
		Timestamp: now,
		UserID:    user.ID,
		Username:  user.UserName,
	}
	activeJoins = append(activeJoins, currentJoin)
	joinTracker[chatID] = activeJoins
	joinsSnapshot := make([]joinEvent, len(activeJoins))
	copy(joinsSnapshot, activeJoins)
	joinTrackerMu.Unlock()

	// Calculate raid sensitivity limits
	windowSecs := settings.RaidWindow
	if windowSecs <= 0 {
		windowSecs = 30
	}
	limitJoins := settings.RaidJoinLimit
	if limitJoins <= 0 {
		switch settings.RaidSensitivity {
		case "LOW":
			limitJoins = 20
			windowSecs = 30
		case "HIGH":
			limitJoins = 5
			windowSecs = 30
		case "CUSTOM":
			// use settings.RaidJoinLimit
			if limitJoins == 0 {
				limitJoins = 10
			}
		default: // "MEDIUM"
			limitJoins = 10
			windowSecs = 30
		}
	}

	windowCutoff := now.Add(-time.Duration(windowSecs) * time.Second)
	recentJoinCount := 0
	for _, je := range joinsSnapshot {
		if je.Timestamp.After(windowCutoff) {
			recentJoinCount++
		}
	}

	if recentJoinCount >= limitJoins {
		// Check if raid lockdown is already active to prevent duplicate triggers
		activeRaidLockdownsMu.RLock()
		alreadyActive := activeRaidLockdowns[chatID]
		activeRaidLockdownsMu.RUnlock()

		if !alreadyActive {
			triggerRaidLockdown(bot, chatID, recentJoinCount, settings.RaidSensitivity, joinsSnapshot)
			return true
		}
	}

	return false
}

// triggerRaidLockdown activates emergency lockdown and alerts administrators
func triggerRaidLockdown(bot *tgbotapi.BotAPI, chatID int64, joinCount int, sensitivity string, recentJoins []joinEvent) {
	activeRaidLockdownsMu.Lock()
	activeRaidLockdowns[chatID] = true
	activeRaidLockdownsMu.Unlock()

	// 1. Enable Chat Lockdown (Restrict sending messages for non-admins)
	bot.Request(tgbotapi.SetChatPermissionsConfig{
		ChatConfig: tgbotapi.ChatConfig{ChatID: chatID},
		Permissions: &tgbotapi.ChatPermissions{
			CanSendMessages: false,
		},
	})

	// 2. Record Raid Event in DB
	var raidID int64
	err := database.Pool.QueryRow(context.Background(), `
		INSERT INTO raid_events (chat_id, join_count, sensitivity, triggered_at, action)
		VALUES ($1, $2, $3, NOW(), 'lockdown')
		RETURNING id
	`, chatID, joinCount, sensitivity).Scan(&raidID)
	if err != nil {
		log.Printf("⚠️ Error recording raid event: %v", err)
	}

	// 3. Increment analytics
	workers.IncrMetric(chatID, workers.MetricRaidEvent)

	// 4. Send interactive alert in group
	alertText := fmt.Sprintf(`🚨 <b>RAID PROTECTION ACTIVATED!</b>

<blockquote expandable>⚠️ <b>Mass Join Raid Detected:</b>
• 👥 <b>Join Velocity:</b> <code>%d joins</code> within threshold
• 🛡️ <b>Sensitivity:</b> <code>%s</code>
• 🔒 <b>Action:</b> Temporary Group Lockdown Enabled

New messages are temporarily restricted to protect the community.</blockquote>`,
		joinCount, sensitivity)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔓 Disable Lockdown", fmt.Sprintf("raid_unlock:%d:%d", chatID, raidID)),
			tgbotapi.NewInlineKeyboardButtonData("🛡️ Keep Active", fmt.Sprintf("raid_ack:%d", chatID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⚙️ Raid Settings", fmt.Sprintf("panel_sec:%d", chatID)),
		),
	)

	msg := tgbotapi.NewMessage(chatID, alertText)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = keyboard
	bot.Send(msg)

	// 5. Send alert to configured Log Channel if any
	if config.LogChannelID != 0 {
		channelAlert := tgbotapi.NewMessage(config.LogChannelID, fmt.Sprintf(
			"🚨 <b>Raid Alert</b> for Chat <code>%d</code>: %d joins detected. Lockdown enabled.",
			chatID, joinCount,
		))
		channelAlert.ParseMode = "HTML"
		bot.Send(channelAlert)
	}

	logAction(chatID, 0, bot.Self.ID, models.ActionLockdown, fmt.Sprintf("Auto-Raid Protection: %d joins", joinCount), nil, map[string]interface{}{
		"raid_id":    raidID,
		"join_count": joinCount,
	})
}

// SetLockdown manually turns on or off the emergency chat lockdown.
func SetLockdown(bot *tgbotapi.BotAPI, chatID int64, enable bool, reason string, modID int64) error {
	var permissions *tgbotapi.ChatPermissions
	if enable {
		permissions = &tgbotapi.ChatPermissions{
			CanSendMessages: false,
		}
	} else {
		permissions = &tgbotapi.ChatPermissions{
			CanSendMessages:       true,
			CanSendMediaMessages:  true,
			CanSendPolls:          true,
			CanSendOtherMessages:  true,
			CanAddWebPagePreviews: true,
			CanInviteUsers:        true,
		}
	}

	_, err := bot.Request(tgbotapi.SetChatPermissionsConfig{
		ChatConfig:  tgbotapi.ChatConfig{ChatID: chatID},
		Permissions: permissions,
	})
	if err != nil {
		return err
	}

	activeRaidLockdownsMu.Lock()
	activeRaidLockdowns[chatID] = enable
	activeRaidLockdownsMu.Unlock()

	action := models.ActionUnlockdown
	if enable {
		action = models.ActionLockdown
	}
	logAction(chatID, 0, modID, action, reason, nil, nil)

	return nil
}

// IsLockdownActive checks if a chat is currently in active raid lockdown.
func IsLockdownActive(chatID int64) bool {
	activeRaidLockdownsMu.RLock()
	defer activeRaidLockdownsMu.RUnlock()
	return activeRaidLockdowns[chatID]
}
