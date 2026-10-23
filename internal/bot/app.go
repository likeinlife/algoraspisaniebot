package bot

import (
	"context"
	"fmt"
	"log"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/robfig/cron/v3"

	"algobot/internal/store"
)

type Config struct {
	SchedulerEnabled bool
	CronSpec         string
	Timezone         string
}

type App struct {
	api   *tgbotapi.BotAPI
	store *store.Store
	cfg   Config
	cron  *cron.Cron
}

func New(api *tgbotapi.BotAPI, db *store.Store, cfg Config) (*App, error) {
	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		return nil, fmt.Errorf("load timezone %q: %w", cfg.Timezone, err)
	}
	app := &App{api: api, store: db, cfg: cfg}
	if cfg.SchedulerEnabled {
		c := cron.New(cron.WithLocation(loc))
		if _, err := c.AddFunc(cfg.CronSpec, app.publishPoll); err != nil {
			return nil, fmt.Errorf("parse POLL_CRON: %w", err)
		}
		app.cron = c
	}
	return app, nil
}

func (a *App) Run(ctx context.Context) error {
	if a.cron != nil {
		a.cron.Start()
		defer a.cron.Stop()
	}

	updatesConfig := tgbotapi.NewUpdate(0)
	updatesConfig.Timeout = 60
	updatesConfig.AllowedUpdates = []string{"message", "poll_answer"}
	updates := a.api.GetUpdatesChan(updatesConfig)
	log.Printf("bot started; automatic polls enabled: %t", a.cfg.SchedulerEnabled)

	for {
		select {
		case <-ctx.Done():
			a.api.StopReceivingUpdates()
			return ctx.Err()
		case update, ok := <-updates:
			if !ok {
				return nil
			}
			if err := a.handleUpdate(ctx, update); err != nil {
				log.Printf("handle update %d: %v", update.UpdateID, err)
			}
		}
	}
}

func (a *App) handleUpdate(ctx context.Context, update tgbotapi.Update) error {
	if update.Message != nil {
		return a.handleMessage(ctx, update.Message)
	}
	if update.PollAnswer != nil {
		answer := update.PollAnswer
		if len(answer.OptionIDs) == 0 {
			return a.store.RemovePollAnswer(ctx, answer.PollID, answer.User.ID)
		}
		if len(answer.OptionIDs) > 1 {
			return nil
		}
		return a.store.SavePollAnswer(ctx, store.PollAnswer{
			PollID:    answer.PollID,
			UserID:    answer.User.ID,
			Option:    answer.OptionIDs[0],
			UpdatedAt: time.Now(),
		})
	}
	return nil
}

func (a *App) handleMessage(ctx context.Context, message *tgbotapi.Message) error {
	if message == nil || message.Chat == nil || !message.IsCommand() {
		return nil
	}
	if message.Chat.Type != "group" && message.Chat.Type != "supergroup" {
		return nil
	}

	chatID, err := a.store.ChatID(ctx)
	if err != nil {
		return err
	}
	if chatID != 0 && chatID != message.Chat.ID {
		return nil
	}
	if chatID == 0 {
		if message.Command() != "chat-id" {
			return a.send(message.Chat.ID, "Сначала привяжите группу командой /chat-id.")
		}
		if err := a.store.SetChatID(ctx, message.Chat.ID); err != nil {
			return err
		}
	}

	switch message.Command() {
	case "chat-id":
		return a.send(message.Chat.ID, fmt.Sprintf("ID этой группы: %d", message.Chat.ID))
	case "start", "help":
		return a.send(message.Chat.ID, "Я публикую опрос о дне проведения алгоритмов. Команда /chat-id показывает ID группы.")
	default:
		return nil
	}
}

func (a *App) publishPoll() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	chatID, err := a.store.ChatID(ctx)
	if err != nil {
		log.Printf("read target chat: %v", err)
		return
	}
	if chatID == 0 {
		log.Printf("automatic poll skipped: send /chat-id in the target group first")
		return
	}
	poll := tgbotapi.NewPoll(chatID, "Когда проводим алгоритмы?", "Суббота", "Воскресенье")
	poll.IsAnonymous = false
	poll.AllowsMultipleAnswers = false
	sent, err := a.api.Send(poll)
	if err != nil {
		log.Printf("send scheduled poll: %v", err)
		return
	}
	if err := a.store.SavePoll(ctx, sent.Poll.ID, chatID, int64(sent.MessageID)); err != nil {
		log.Printf("save poll %s: %v", sent.Poll.ID, err)
	}
}

func (a *App) send(chatID int64, text string) error {
	_, err := a.api.Send(tgbotapi.NewMessage(chatID, text))
	return err
}
