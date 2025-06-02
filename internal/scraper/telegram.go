package scraper

import (
	"context"
	"github.com/golang/glog"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/query"
	"github.com/gotd/td/telegram/query/dialogs"
	"github.com/gotd/td/telegram/query/messages"
	"github.com/gotd/td/tg"
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
		glog.Info("Auth succeed")

		raw := tg.NewClient(client)
		cb := func(ctx context.Context, dlg dialogs.Elem) error {
			// Skip deleted dialogs.
			if dlg.Deleted() {
				return nil
			}

			return dlg.Messages(raw).ForEach(ctx, func(ctx context.Context, elem messages.Elem) error {
				msg, ok := elem.Msg.(*tg.Message)
				if !ok {
					return nil
				}
				glog.Info(msg.Message)

				return nil
			})
		}

		return query.GetDialogs(raw).ForEach(ctx, cb)
	})
}
