package bot

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"strings"

	"github.com/ai-shift/tgmd"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/jackc/pgx/v5"

	"mimi/internal/bot/llm"
	"mimi/internal/persist"
)

func Start(ctx context.Context, token string) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Fatal(err)
	}
	slog.Info("Authorized account", "username", bot.Self.UserName)

	conn, err := pgx.Connect(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("failed to connect to postgres with: %s", err)
	}
	defer conn.Close(ctx)

	q := persist.New(conn)
	handler := UpdateHandler{
		bot: bot,
		llm: llm.New(q),
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	udpates := bot.GetUpdatesChan(u)

	for update := range udpates {
		if update.Message != nil && len(update.Message.Text) > 0 {
			ctx, cancel := context.WithCancel(ctx)
			if err := handler.handleMessage(ctx, update.Message); err != nil {
				slog.Error("failed to handle message", "with", err)
			}
			cancel()
		}
	}
}

type UpdateHandler struct {
	bot *tgbotapi.BotAPI
	llm llm.LLM
}

func (h UpdateHandler) handleMessage(ctx context.Context, m *tgbotapi.Message) error {
	slog.Info("new message", "chatId", m.Chat.ID, "text", m.Text)

	// Set bot typing status
	_, _ = h.bot.Send(tgbotapi.NewChatAction(m.Chat.ID, tgbotapi.ChatTyping))

	// Generate LLM answer
	answer, err := h.llm.Answer(ctx, m.Chat.ID, m.Text)
	if err != nil {
		return fmt.Errorf("failed to get answer from LLM with %w", err)
	}
	slog.Info("got LLM answer", "length", len(answer))

	// Response to the user's query
	escaped := tgmd.Telegramify(strings.ReplaceAll(answer, "\n\n", "\n"))
	msg := tgbotapi.NewMessage(m.Chat.ID, escaped)
	msg.ParseMode = "MarkdownV2"
	_, err = h.bot.Send(msg)
	if err != nil {
		return fmt.Errorf("failed to send LLM response with %w", err)
	}

	return nil
}
