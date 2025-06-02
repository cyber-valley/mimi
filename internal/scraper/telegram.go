package scraper

import (
	"context"
	"github.com/golang/glog"
	"github.com/gotd/td/telegram"
	"os"
	"os/signal"
)

func StartTelegramScraper(appID int, appHash string, telegramBotToken *string) error {
	glog.Info("Telegram scraper starts")
	client := telegram.NewClient(appID, appHash, telegram.Options{})
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	return client.Run(ctx, func(ctx context.Context) error {
		status, err := client.Auth().Status(ctx)
		if err != nil {
			glog.Error("Failed to get auth status")
			return err
		}
		if !status.Authorized {
			if _, err := client.Auth().Bot(ctx, *telegramBotToken); err != nil {
				glog.Error("Failed to auth via bot with %s", err)
				return err
			}
		}
		glog.Info("Scraper finished")
		return nil
	})
}
