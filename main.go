package main

import (
	"log"

	"minimate-bot/config"
	"minimate-bot/database" // Import your new database package
	"minimate-bot/handlers"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	// 1. Load Configuration (.env)
	botToken := config.LoadConfig()

	// 2. Initialize Database Connection
	database.InitDB()
        database.CreateTables()

	// 3. Initialize Bot
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Panic("Failed to initialize bot: ", err)
	}

	log.Printf("✅ Authorized successfully on account: @%s", bot.Self.UserName)

	// 4. Configure Polling
	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 60
	updates := bot.GetUpdatesChan(updateConfig)

	log.Println("⚡ Minimate is online and listening for messages...")

	// 5. The Event Loop
	for update := range updates {
		go handlers.HandleUpdate(bot, update)
	}
}