// Package workers provides background goroutines for the bot.
package workers

import (
	"context"
	"log"
	"time"

	"minimate-bot/database"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// StartTempActionWorker polls the temp_actions table every 30 seconds
// and automatically lifts expired bans and mutes.
func StartTempActionWorker(ctx context.Context, bot *tgbotapi.BotAPI) {
	log.Println("⚙️ TempAction worker started.")
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("⏹️ TempAction worker stopped.")
			return
		case <-ticker.C:
			processTempActions(bot)
		}
	}
}

// processTempActions finds all expired temp_actions and restores permissions.
func processTempActions(bot *tgbotapi.BotAPI) {
	rows, err := database.Pool.Query(context.Background(), `
		SELECT id, chat_id, user_id, action_type
		FROM temp_actions
		WHERE done = false AND expires_at <= NOW()
		ORDER BY expires_at ASC
		LIMIT 100
	`)
	if err != nil {
		log.Printf("⚠️ TempAction query error: %v", err)
		return
	}
	defer rows.Close()

	type tempAction struct {
		ID         int64
		ChatID     int64
		UserID     int64
		ActionType string
	}
	var actions []tempAction

	for rows.Next() {
		var ta tempAction
		if err := rows.Scan(&ta.ID, &ta.ChatID, &ta.UserID, &ta.ActionType); err == nil {
			actions = append(actions, ta)
		}
	}
	rows.Close()

	for _, ta := range actions {
		var apiErr error

		switch ta.ActionType {
		case "mute":
			// Restore full send permissions
			restrictConfig := tgbotapi.RestrictChatMemberConfig{
				ChatMemberConfig: tgbotapi.ChatMemberConfig{
					ChatID: ta.ChatID,
					UserID: ta.UserID,
				},
				UntilDate: 0,
				Permissions: &tgbotapi.ChatPermissions{
					CanSendMessages:       true,
					CanSendMediaMessages:  true,
					CanSendPolls:          true,
					CanSendOtherMessages:  true,
					CanAddWebPagePreviews: true,
					CanInviteUsers:        true,
				},
			}
			_, apiErr = bot.Request(restrictConfig)
			if apiErr == nil {
				log.Printf("🔊 Auto-unmuted user %d in chat %d", ta.UserID, ta.ChatID)
			}

		case "ban":
			unbanConfig := tgbotapi.UnbanChatMemberConfig{
				ChatMemberConfig: tgbotapi.ChatMemberConfig{
					ChatID: ta.ChatID,
					UserID: ta.UserID,
				},
				OnlyIfBanned: true,
			}
			_, apiErr = bot.Request(unbanConfig)
			if apiErr == nil {
				log.Printf("🔓 Auto-unbanned user %d in chat %d", ta.UserID, ta.ChatID)
			}
		}

		// Mark done regardless (even if Telegram API fails, we don't want to loop forever)
		if apiErr != nil {
			log.Printf("⚠️ TempAction %d (%s) for user %d in %d: %v", ta.ID, ta.ActionType, ta.UserID, ta.ChatID, apiErr)
		}
		database.Pool.Exec(context.Background(),
			"UPDATE temp_actions SET done = true WHERE id = $1", ta.ID)
	}
}
