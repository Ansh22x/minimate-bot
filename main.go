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
	// 1. Immediately bind HTTP server for Render / Koyeb Web Service health-checks
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
		log.Printf("🌐 HTTP Health Server listening on port :%s", port)
		if err := http.ListenAndServe(":"+port, nil); err != nil && err != http.ErrServerClosed {
			log.Printf("⚠️ HTTP server error: %v", err)
		}
	}()

	// 2. Load Configuration
	config.LoadConfig()

	// 3. Connect to Database & Create Tables
	database.InitDB()
	defer database.Pool.Close()
	database.CreateTables()

	// 4. Initialize Telegram Bot
	bot, err := tgbotapi.NewBotAPI(config.BotToken)
	if err != nil {
		log.Fatalf("❌ Failed to initialize bot: %v", err)
	}

	bot.Debug = false
	log.Printf("🚀 Authorized as @%s (ID: %d)", bot.Self.UserName, bot.Self.ID)

	// 5. Setup Long Polling Update Stream
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	// 6. Graceful Shutdown Signal Handler
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-stopChan
		log.Println("🛑 Shutting down gracefully...")
		bot.StopReceivingUpdates()
		database.Pool.Close()
		os.Exit(0)
	}()

	log.Println("⚡ MiniMate is online and actively listening for updates...")

	// 7. Event Dispatcher Loop
	for update := range updates {
		go handlers.HandleUpdate(bot, update)
	}
}
