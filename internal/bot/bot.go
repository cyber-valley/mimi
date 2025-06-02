package bot

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/golang/glog"
)

func Start(token string) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		glog.Fatal(err)
	}
	glog.Infoln("Authorized on account", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	udpates := bot.GetUpdatesChan(u)

	for update := range udpates {
		if update.Message != nil {
			glog.Infof("[%d]: %s", update.Message.Chat.ID, update.Message.Text)
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, update.Message.Text)
			bot.Send(msg)
		}
	}
}
