package bot

import (
	"context"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/golang/glog"

	"mimi/internal/bot/llm"
)

func Start(token string) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		glog.Fatal(err)
	}
	glog.Infoln("Authorized on account", bot.Self.UserName)

	handler := UpdateHandler{
		bot: bot,
		llm: llm.New(),
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	udpates := bot.GetUpdatesChan(u)

	for update := range udpates {
		if update.Message != nil && len(update.Message.Text) > 0 {
			handler.handleMessage(update.Message)
		}
	}
}

type UpdateHandler struct {
	bot *tgbotapi.BotAPI
	llm llm.LLM
}

func (h UpdateHandler) handleMessage(m *tgbotapi.Message) error {
	glog.Infof("[%d]: %s", m.Chat.ID, m.Text)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	answer, err := h.llm.Answer(ctx, m.Text)
	if err != nil {
		return fmt.Errorf("failed to get answer from LLM with %w", err)
	}
	msg := tgbotapi.NewMessage(m.Chat.ID, answer)
	h.bot.Send(msg)
	return nil
}
