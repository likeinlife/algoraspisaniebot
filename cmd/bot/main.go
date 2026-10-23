package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"algobot/internal/bot"
	"algobot/internal/store"
)

func main() {
	token := strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN"))
	if token == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN is required")
	}

	path := envOr("SQLITE_PATH", "./data/bot.db")
	db, err := store.Open(path)
	if err != nil {
		log.Fatalf("open sqlite: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("close sqlite: %v", err)
		}
	}()

	if raw := strings.TrimSpace(os.Getenv("TELEGRAM_CHAT_ID")); raw != "" {
		chatID, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			log.Fatalf("TELEGRAM_CHAT_ID must be an integer: %v", err)
		}
		if err := db.SetChatID(context.Background(), chatID); err != nil {
			log.Fatalf("save configured chat ID: %v", err)
		}
	}

	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Fatalf("create telegram bot: %v", err)
	}

	app, err := bot.New(api, db, bot.Config{
		SchedulerEnabled: envBool("POLL_SCHEDULER_ENABLED", true),
		CronSpec:         envOr("POLL_CRON", "0 12 * * 5"),
		Timezone:         envOr("TIMEZONE", "Europe/Moscow"),
	})
	if err != nil {
		log.Fatalf("configure bot: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := app.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("run bot: %v", err)
	}
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		log.Fatalf("%s must be true or false: %v", key, err)
	}
	return parsed
}
