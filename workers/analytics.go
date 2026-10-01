package workers

import (
	"context"
	"log"
	"sync"
	"time"

	"minimate-bot/database"
)

// analyticsBuffer holds in-memory counters before flushing to DB.
var (
	analyticsBuffer   = make(map[int64]map[string]int64)
	analyticsBufferMu sync.Mutex
)

// IncrMetric increments an analytics counter in the in-memory buffer.
// Thread-safe. Buffer is flushed to DB every 5 minutes.
func IncrMetric(chatID int64, metric string) {
	analyticsBufferMu.Lock()
	defer analyticsBufferMu.Unlock()
	if analyticsBuffer[chatID] == nil {
		analyticsBuffer[chatID] = make(map[string]int64)
	}
	analyticsBuffer[chatID][metric]++
}

// StartAnalyticsWorker flushes the in-memory analytics buffer to the database every 5 minutes.
func StartAnalyticsWorker(ctx context.Context) {
	log.Println("⚙️ Analytics worker started.")
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Final flush before shutdown
			flushAnalytics()
			log.Println("⏹️ Analytics worker stopped.")
			return
		case <-ticker.C:
			flushAnalytics()
		}
	}
}

// flushAnalytics drains the in-memory buffer and writes increments to analytics_daily.
func flushAnalytics() {
	analyticsBufferMu.Lock()
	if len(analyticsBuffer) == 0 {
		analyticsBufferMu.Unlock()
		return
	}
	// Snapshot and clear buffer
	snapshot := analyticsBuffer
	analyticsBuffer = make(map[int64]map[string]int64)
	analyticsBufferMu.Unlock()

	today := time.Now().UTC().Format("2006-01-02")
	for chatID, metrics := range snapshot {
		for metric, value := range metrics {
			_, err := database.Pool.Exec(context.Background(), `
				INSERT INTO analytics_daily (chat_id, date, metric, value)
				VALUES ($1, $2, $3, $4)
				ON CONFLICT (chat_id, date, metric) DO UPDATE
				SET value = analytics_daily.value + $4
			`, chatID, today, metric, value)
			if err != nil {
				log.Printf("⚠️ Analytics flush error for chat %d metric %s: %v", chatID, metric, err)
			}
		}
	}
}

// GetAnalytics returns daily metric totals for a chat over N days.
func GetAnalytics(chatID int64, days int) (map[string]int64, error) {
	rows, err := database.Pool.Query(context.Background(), `
		SELECT metric, SUM(value)
		FROM analytics_daily
		WHERE chat_id = $1 AND date >= CURRENT_DATE - $2::int
		GROUP BY metric
	`, chatID, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]int64)
	for rows.Next() {
		var metric string
		var val int64
		if err := rows.Scan(&metric, &val); err == nil {
			result[metric] = val
		}
	}
	return result, nil
}

// Analytics metric constants
const (
	MetricMessage          = "messages"
	MetricJoin             = "joins"
	MetricLeave            = "leaves"
	MetricBan              = "bans"
	MetricMute             = "mutes"
	MetricWarn             = "warns"
	MetricSpamBlocked      = "spam_blocked"
	MetricVerifyPass       = "verify_pass"
	MetricVerifyFail       = "verify_fail"
	MetricFilterTriggered  = "filter_triggered"
	MetricRaidEvent        = "raid_events"
)
