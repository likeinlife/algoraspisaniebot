package bot

import (
	"context"
	"fmt"
	"log"
	"strings"
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

	log.Printf("authenticated with Telegram as @%s (bot_id=%d)", a.api.Self.UserName, a.api.Self.ID)
	chatID, err := a.store.ChatID(ctx)
	if err != nil {
		return fmt.Errorf("read target group: %w", err)
	}
	if chatID == 0 {
		log.Printf("target group is not bound yet; send /chat_id in the group")
	} else {
		chat, err := a.api.GetChat(tgbotapi.ChatInfoConfig{
			ChatConfig: tgbotapi.ChatConfig{ChatID: chatID},
		})
		if err != nil {
			log.Printf("cannot verify target group chat_id=%d: %v (check the ID and that the bot is still in the group)", chatID, err)
		} else {
			log.Printf("target group is accessible: title=%q chat_id=%d type=%s", chat.Title, chat.ID, chat.Type)
		}
	}

	updatesConfig := tgbotapi.NewUpdate(0)
	updatesConfig.Timeout = 60
	updatesConfig.AllowedUpdates = []string{"message"}
	updates := a.api.GetUpdatesChan(updatesConfig)
	log.Printf("long polling started; automatic polls enabled: %t", a.cfg.SchedulerEnabled)

	for {
		select {
		case <-ctx.Done():
			a.api.StopReceivingUpdates()
			return ctx.Err()
		case update, ok := <-updates:
			if !ok {
				log.Printf("Telegram update channel closed")
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
	return nil
}

func (a *App) handleMessage(ctx context.Context, message *tgbotapi.Message) error {
	if message == nil || message.Chat == nil {
		return nil
	}
	command := commandName(message)
	log.Printf("received message update: chat_id=%d type=%s command=%q", message.Chat.ID, message.Chat.Type, command)
	isChatIDCommand := command == "chat_id"
	isPollNowCommand := command == "poll_now"
	if !message.IsCommand() && !isChatIDCommand && !isPollNowCommand {
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
		if !isChatIDCommand {
			return a.send(message.Chat.ID, "Сначала привяжите группу командой /chat_id.")
		}
		if err := a.store.SetChatID(ctx, message.Chat.ID); err != nil {
			return err
		}
		log.Printf("bound target group: title=%q chat_id=%d", message.Chat.Title, message.Chat.ID)
	}

	switch {
	case isChatIDCommand:
		return a.send(message.Chat.ID, fmt.Sprintf("ID этой группы: %d", message.Chat.ID))
	case message.Command() == "start", message.Command() == "help":
		return a.send(message.Chat.ID, "Я публикую опрос о дне проведения алгоритмов. В опросе можно выбрать несколько ответов и переголосовать. Команды: /chat_id — ID группы, /poll_now — отправить опрос сейчас.")
	case isPollNowCommand:
		if err := a.publishPollToConfiguredChat(ctx); err != nil {
			log.Printf("send poll on command: %v", err)
			return a.send(message.Chat.ID, "Не удалось отправить опрос. Проверьте настройки и права бота.")
		}
		return nil
	default:
		return nil
	}
}

func commandName(message *tgbotapi.Message) string {
	if message.IsCommand() {
		return message.Command()
	}
	fields := strings.Fields(message.Text)
	if len(fields) == 0 || !strings.HasPrefix(fields[0], "/") {
		return ""
	}
	command := strings.TrimPrefix(fields[0], "/")
	if at := strings.IndexByte(command, '@'); at >= 0 {
		command = command[:at]
	}
	return command
}

func (a *App) publishPoll() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := a.publishPollToConfiguredChat(ctx); err != nil {
		log.Printf("send scheduled poll: %v", err)
	}
}

func (a *App) publishPollToConfiguredChat(ctx context.Context) error {
	chatID, err := a.store.ChatID(ctx)
	if err != nil {
		return fmt.Errorf("read target chat: %w", err)
	}
	if chatID == 0 {
		return fmt.Errorf("target chat is not bound; send /chat_id in the target group first")
	}
	poll := tgbotapi.NewPoll(chatID, "Когда проводим алгоритмы?", "Суббота", "Воскресенье", "Узнать результаты")
	poll.IsAnonymous = false
	poll.AllowsMultipleAnswers = true
	sent, err := a.api.Send(poll)
	if err != nil {
		return fmt.Errorf("send poll: %w", err)
	}
	log.Printf("scheduled poll published: poll_id=%s chat_id=%d message_id=%d", sent.Poll.ID, chatID, sent.MessageID)
	return nil
}

func (a *App) send(chatID int64, text string) error {
	_, err := a.api.Send(tgbotapi.NewMessage(chatID, text))
	return err
}
