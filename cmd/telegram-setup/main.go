package main

import (
	"context"
	"fmt"
	"os"
	"log"
	"time"
	"log/slog"

	"github.com/golang/glog"
	"github.com/gotd/contrib/middleware/floodwait"
	"github.com/gotd/contrib/middleware/ratelimit"
	"golang.org/x/time/rate"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/jackc/pgx/v5"

	mimitg "mimi/internal/scraper/telegram"
)

const (
	tgPhone = "TG_PHONE"
)

func main() {
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("failed to connect to postgres with: %s", err)
	}
	defer conn.Close(ctx)

	if err := run(ctx, conn); err != nil {
		log.Fatalf("failed with %s", err)
	}
}

func run(ctx context.Context, conn *pgx.Conn) error {
	phone := os.Getenv(tgPhone)
	if phone == "" {
		return fmt.Errorf("phone env variable %s is missing", tgPhone)
	}

	waiter := floodwait.NewWaiter().WithCallback(func(ctx context.Context, wait floodwait.FloodWait) {
		glog.Warning("got FLOOD_WAIT. Will retry after", wait.Duration)
	})

	client, err := telegram.ClientFromEnvironment(telegram.Options{
		Middlewares: []telegram.Middleware{
			waiter,
			ratelimit.New(rate.Every(time.Millisecond*100), 5),
		},
	})
	if err != nil {
		glog.Error("failed to init client with ", err)
		return err
	}
	api := client.API()

	flow := auth.NewFlow(mimitg.TerminalUserAuthenticator{PhoneNumber: phone}, auth.SendCodeOptions{})

	return waiter.Run(ctx, func(ctx context.Context) error {
		if err := client.Run(ctx, func(ctx context.Context) error {
			if err := client.Auth().IfNecessary(ctx, flow); err != nil {
				return err
			}

			self, err := client.Self(ctx)
			if err != nil {
				return err
			}

			name := self.FirstName
			if self.Username != "" {
				name = fmt.Sprintf("%s (@%s)", name, self.Username)
			}
			slog.Info("Current user", "name", name)

			return mimitg.Validate(ctx, api, conn)
		}); err != nil {
			return err
		}
		return nil
	})
}
