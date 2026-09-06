package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"minimate-bot/config"
	"minimate-bot/database"
	"minimate-bot/handlers"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	// 1. Immediately launch HTTP Health Check Server so Render's Port Scanner detects open port instantly
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "🌸 MiniMate Bot is Online 24/7!\nStatus: Healthy & Active")
	})

	go func() {
		log.Printf("🌐 Starting HTTP health-check server on port :%s ...", port)
		if err := http.ListenAndServe(":"+port, nil); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP health check server error: %v", err)
		}
	}()

	// 2. Load Configuration (.env)
	botToken := config.LoadConfig()

	// 3. Initialize Database Connection
	database.InitDB()
	database.CreateTables()
	defer database.CloseDB()

	// 4. Initialize Bot
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Panic("Failed to initialize bot: ", err)
	}

	log.Printf("✅ Authorized successfully on account: @%s", bot.Self.UserName)

	// Set native Telegram "/" menu commands
	botCommands := []tgbotapi.BotCommand{
		{Command: "start", Description: "Start Minimate"},
		{Command: "help", Description: "Full command directory"},
		{Command: "commands", Description: "Full command directory"},
		{Command: "ping", Description: "Check bot latency"},
		{Command: "rules", Description: "View chat rules"},
		{Command: "info", Description: "Get user info"},
		{Command: "id", Description: "Get user and chat IDs"},
		{Command: "warns", Description: "Check warning strikes"},
		{Command: "filters", Description: "List chat filters"},
		{Command: "notes", Description: "List saved notes"},
	}
	_, err = bot.Request(tgbotapi.NewSetMyCommands(botCommands...))
	if err != nil {
		log.Printf("Warning: Failed to set bot commands: %v", err)
	}

	// 5. Configure Polling
	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 60
	updates := bot.GetUpdatesChan(updateConfig)

	// Listen for OS interrupt signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	log.Println("⚡ Minimate is online and listening for messages...")

	// 6. The Event Loop with graceful exit
	for {
		select {
		case sig := <-sigChan:
			log.Printf("Received signal %v, shutting down...", sig)
			bot.StopReceivingUpdates()
			return
		case update, ok := <-updates:
			if !ok {
				log.Println("Updates channel closed, exiting...")
				return
			}
			go handlers.HandleUpdate(bot, update)
		}
	}
}
