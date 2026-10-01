package workers

import (
	"context"
	"log"
	"time"

	"minimate-bot/database"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// StartSchedulerWorker polls scheduled_tasks every minute and runs due jobs.
func StartSchedulerWorker(ctx context.Context, bot *tgbotapi.BotAPI) {
	log.Println("⚙️ Scheduler worker started.")
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("⏹️ Scheduler worker stopped.")
			return
		case <-ticker.C:
			processScheduledTasks(bot)
		}
	}
}

// processScheduledTasks picks up due tasks and executes them.
func processScheduledTasks(bot *tgbotapi.BotAPI) {
	rows, err := database.Pool.Query(context.Background(), `
		SELECT id, chat_id, task_type, payload, recurrence
		FROM scheduled_tasks
		WHERE done = false AND run_at <= NOW()
		ORDER BY run_at ASC
		LIMIT 50
	`)
	if err != nil {
		log.Printf("⚠️ Scheduler query error: %v", err)
		return
	}
	defer rows.Close()

	type task struct {
		ID         int64
		ChatID     int64
		TaskType   string
		Payload    []byte
		Recurrence *string
	}
	var tasks []task
	for rows.Next() {
		var t task
		if err := rows.Scan(&t.ID, &t.ChatID, &t.TaskType, &t.Payload, &t.Recurrence); err == nil {
			tasks = append(tasks, t)
		}
	}
	rows.Close()

	for _, t := range tasks {
		executeScheduledTask(bot, t.ChatID, t.TaskType, t.Payload)

		// Mark one-time tasks as done; calculate next_run for recurring
		if t.Recurrence == nil || *t.Recurrence == "" {
			database.Pool.Exec(context.Background(),
				"UPDATE scheduled_tasks SET done = true, last_run = NOW() WHERE id = $1", t.ID)
		} else {
			nextRun := calculateNextRun(*t.Recurrence)
			database.Pool.Exec(context.Background(), `
				UPDATE scheduled_tasks SET last_run = NOW(), next_run = $1, run_at = $1
				WHERE id = $2
			`, nextRun, t.ID)
		}
	}
}

// executeScheduledTask dispatches a task by type.
func executeScheduledTask(bot *tgbotapi.BotAPI, chatID int64, taskType string, payload []byte) {
	log.Printf("🕐 Executing scheduled task type=%s for chat=%d", taskType, chatID)

	switch taskType {
	case "announcement":
		// Send a scheduled message
		var p struct {
			Text string `json:"text"`
		}
		if err := jsonUnmarshal(payload, &p); err == nil && p.Text != "" {
			msg := tgbotapi.NewMessage(chatID, p.Text)
			msg.ParseMode = "HTML"
			bot.Send(msg)
		}

	case "lockdown":
		// Enable lockdown for the group
		var p struct {
			Enable bool   `json:"enable"`
			Reason string `json:"reason"`
		}
		jsonUnmarshal(payload, &p)
		if p.Enable {
			// Restrict all members from sending messages
			bot.Request(tgbotapi.SetChatPermissionsConfig{
				ChatConfig: tgbotapi.ChatConfig{ChatID: chatID},
				Permissions: &tgbotapi.ChatPermissions{
					CanSendMessages: false,
				},
			})
			msg := tgbotapi.NewMessage(chatID, "🔒 <b>Scheduled Lockdown Activated</b>")
			msg.ParseMode = "HTML"
			bot.Send(msg)
		} else {
			bot.Request(tgbotapi.SetChatPermissionsConfig{
				ChatConfig: tgbotapi.ChatConfig{ChatID: chatID},
				Permissions: &tgbotapi.ChatPermissions{
					CanSendMessages:       true,
					CanSendMediaMessages:  true,
					CanSendPolls:          true,
					CanSendOtherMessages:  true,
					CanInviteUsers:        true,
				},
			})
		}

	case "slow_mode":
		var p struct {
			Seconds int `json:"seconds"`
		}
		jsonUnmarshal(payload, &p)
		params := make(tgbotapi.Params)
		params.AddNonZero64("chat_id", chatID)
		params.AddNonZero("slow_mode_delay", p.Seconds)
		bot.MakeRequest("setChatSlowModeDelay", params)

	default:
		log.Printf("⚠️ Unknown scheduled task type: %s", taskType)
	}
}

// calculateNextRun returns the next execution time based on recurrence string.
func calculateNextRun(recurrence string) time.Time {
	now := time.Now()
	switch recurrence {
	case "daily":
		return now.Add(24 * time.Hour)
	case "weekly":
		return now.Add(7 * 24 * time.Hour)
	case "hourly":
		return now.Add(1 * time.Hour)
	default:
		// Parse as duration (e.g. "30m", "2h")
		if d, err := time.ParseDuration(recurrence); err == nil {
			return now.Add(d)
		}
		return now.Add(24 * time.Hour)
	}
}

// jsonUnmarshal is a local alias to avoid import issues.
func jsonUnmarshal(data []byte, v interface{}) error {
	import_json := jsonUnmarshalFn
	return import_json(data, v)
}
