package main

import (
	"log"

	"minimate-bot/config"
	"minimate-bot/handlers"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	botToken := config.LoadConfig()

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Panic("Failed to initialize bot: ", err)
	}

	log.Printf("✅ Authorized successfully on account: @%s", bot.Self.UserName)

	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 60
	updates := bot.GetUpdatesChan(updateConfig)

	log.Println("⚡ Minimate is online and listening for messages...")

	for update := range updates {
		// THE MAGIC: 'go' spins up a new thread instantly for every message.
		go handlers.HandleUpdate(bot, update)
	}
}