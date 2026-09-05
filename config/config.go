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

	return BotToken
}