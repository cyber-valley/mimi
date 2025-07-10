package bot

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/ai-shift/tgmd"
	"github.com/cozodb/cozo-lib-go"
	"github.com/firebase/genkit/go/genkit"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mimi/internal/bot/llm"
	"mimi/internal/provider/logseq"
)

func Start(ctx context.Context, token string, logseqPath string, g *genkit.Genkit, conn cozo.CozoDB) error {
	slog.Info("starting Telegram Bot")
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return fmt.Errorf("failed to initialize bot api with %w", err)
	}
	slog.Info("Authorized account", "username", bot.Self.UserName)

	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return fmt.Errorf("failed to connect to postgres with %w", err)
	}

	graph := logseq.NewRegexGraph(logseqPath)
	handler := UpdateHandler{
		bot: bot,
		g:   graph,
		llm: llm.New(pool, graph, g, conn),
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)
	slog.Info("Telegram bot started")
	for {
		select {
		case <-ctx.Done():
			return nil
		case update := <-updates:
			if update.Message != nil && len(update.Message.Text) > 0 {
				go func() {
					slog.Info("got new message in Telegram bot")
					if err := handler.handleMessage(ctx, update.Message); err != nil {
						slog.Error("failed to handle message", "with", err)
						msg := tgbotapi.NewMessage(update.Message.Chat.ID, err.Error())
						_, err = bot.Send(msg)
						if err != nil {
							slog.Error("failed to answer after failed message handling", "with", err)
						}
					}
				}()
			}
		}
	}
}

type UpdateHandler struct {
	bot *tgbotapi.BotAPI
	g   logseq.RegexGraph
	llm llm.LLM
}

func (h UpdateHandler) handleMessage(ctx context.Context, m *tgbotapi.Message) error {
	slog.Info("new message", "chatId", m.Chat.ID, "text", m.Text)

	// Set bot typing status
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	go func() {
		for {
			_, _ = h.bot.Send(tgbotapi.NewChatAction(m.Chat.ID, tgbotapi.ChatTyping))
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				continue
			}
		}
	}()

	// Generate LLM answer
	answer, err := h.llm.Answer(ctx, m.Chat.ID, m.Text)
	if err != nil {
		return fmt.Errorf("failed to get answer from LLM with %w", err)
	}
	slog.Info("got LLM answer", "length", len(answer))

	// Response to the user's query
	escaped := tgmd.Telegramify(strings.ReplaceAll(answer, "\n\n", "\n"))
	if err := sendLongMessage(h.bot, m.Chat.ID, escaped); err != nil {
		return fmt.Errorf("failed to send LLM response with %w", err)
	}

	return nil
}

// sendLongMessage splits text into chunks and may send several messages
// to prevent error of exceeding Telegram's limit
func sendLongMessage(bot *tgbotapi.BotAPI, chatID int64, text string) error {
	var buf []string
	var curLen int
	for _, line := range strings.Split(text, "\n") {
		if curLen+len(line) <= 4096 {
			// Under the limit, continue accumulating
			buf = append(buf, line)
			curLen += len(line)
			continue
		}
		// The time to send is come
		if err := sendShortMessage(bot, chatID, strings.Join(buf, "\n")); err != nil {
			return nil
		}
		// Clean up state
		buf = buf[:0]
		curLen = 0
	}
	if len(buf) == 0 {
		return nil
	}
	return sendShortMessage(bot, chatID, strings.Join(buf, "\n"))
}

func sendShortMessage(bot *tgbotapi.BotAPI, chatID int64, text string) error {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "MarkdownV2"
	_, err := bot.Send(msg)
	return err
}
