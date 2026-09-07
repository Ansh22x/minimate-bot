package main

import (
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
	// 1. Start HTTP Server immediately for Render Web Service 24/7 port binding
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("✅ MiniMate Bot is live and running 24/7!"))
	})
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
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
	bot.Request(tgbotapi.NewSetMyCommands(botCommands...))

	// 5. Setup Long Polling Update Stream
	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 60

	updates := bot.GetUpdatesChan(updateConfig)

	// 6. Graceful Shutdown Signal Handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("🛑 Shutting down gracefully...")
		bot.StopReceivingUpdates()
		database.CloseDB()
		os.Exit(0)
	}()

	log.Println("⚡ MiniMate is online and actively listening for updates...")

	// 7. Event Dispatcher Loop
	for update := range updates {
		go handlers.HandleUpdate(bot, update)
	}
}
