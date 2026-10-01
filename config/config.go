package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

var (
	BotToken      string
	OwnerID       int64
	OwnerUsername string

	// Database
	DatabaseURL string

	// Feature flags
	AntiSpamEnabled  bool
	AntiRaidEnabled  bool
	CaptchaEnabled   bool

	// Logging
	LogChannelID int64

	// Analytics
	AnalyticsEnabled bool

	// Default warn thresholds (can be overridden per-group in group_settings)
	DefaultWarnLimit int
)

// LoadConfig reads the .env file and initializes bot credentials
func LoadConfig() string {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, relying on system environment variables")
	}

	BotToken = os.Getenv("BOT_TOKEN")
	if BotToken == "" {
		log.Fatal("BOT_TOKEN must be set in .env")
	}

	ownerIDStr := os.Getenv("OWNER_ID")
	if ownerIDStr != "" {
		parsedID, err := strconv.ParseInt(ownerIDStr, 10, 64)
		if err == nil {
			OwnerID = parsedID
		}
	}

	OwnerUsername = os.Getenv("OWNER_USERNAME")
	if OwnerUsername == "" {
		OwnerUsername = "TheDarkKratos"
	}

	DatabaseURL = os.Getenv("DATABASE_URL")

	AntiSpamEnabled = parseBool(os.Getenv("ANTISPAM_ENABLED"), false)
	AntiRaidEnabled = parseBool(os.Getenv("ANTIRAID_ENABLED"), false)
	CaptchaEnabled = parseBool(os.Getenv("CAPTCHA_ENABLED"), false)
	AnalyticsEnabled = parseBool(os.Getenv("ANALYTICS_ENABLED"), true)

	if logChanStr := os.Getenv("LOG_CHANNEL_ID"); logChanStr != "" {
		LogChannelID, _ = strconv.ParseInt(logChanStr, 10, 64)
	}

	DefaultWarnLimit = parseInt(os.Getenv("DEFAULT_WARN_LIMIT"), 3)

	log.Printf("✅ Config loaded — Owner: %d | LogChannel: %d | AntiSpam: %v | AntiRaid: %v",
		OwnerID, LogChannelID, AntiSpamEnabled, AntiRaidEnabled)

	return BotToken
}

func parseBool(s string, defaultVal bool) bool {
	if s == "" {
		return defaultVal
	}
	return s == "true" || s == "1" || s == "yes"
}

func parseInt(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return v
}