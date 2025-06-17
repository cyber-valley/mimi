package bot

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"strings"

	"github.com/ai-shift/tgmd"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"mimi/internal/bot/llm"
)

func Start(token string) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Fatal(err)
	}
	slog.Info("Authorized account", "username", bot.Self.UserName)

	handler := UpdateHandler{
		bot: bot,
		llm: llm.New(),
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	udpates := bot.GetUpdatesChan(u)

	for update := range udpates {
		if update.Message != nil && len(update.Message.Text) > 0 {
			if err := handler.handleMessage(update.Message); err != nil {
				slog.Error("failed to handle message", "with", err)
			}
		}
	}
}

type UpdateHandler struct {
	bot *tgbotapi.BotAPI
	llm llm.LLM
}

func (h UpdateHandler) handleMessage(m *tgbotapi.Message) error {
	slog.Info("new message", "chatId", m.Chat.ID, "text", m.Text)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set bot typing status
	_, err := h.bot.Send(tgbotapi.NewChatAction(m.Chat.ID, tgbotapi.ChatTyping))
	if err != nil {
		slog.Error("failed to set typing status", "with", err)
	}

	// Generate LLM answer
	answer, err := h.llm.Answer(ctx, m.Text)
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
