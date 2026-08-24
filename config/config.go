package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

// LoadConfig reads the .env file and returns the Bot Token
func LoadConfig() string {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, relying on system environment variables")
	}

	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		log.Fatal("BOT_TOKEN must be set in .env")
	}

	return token
}