package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/gotd/contrib/middleware/floodwait"
	"github.com/gotd/contrib/middleware/ratelimit"
	"github.com/gotd/td/telegram"
	tdauth "github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/telegram/updates"
	"github.com/gotd/td/telegram/updates/hook"
	"github.com/gotd/td/tg"
	"golang.org/x/time/rate"

	"mimi/internal/provider/telegram/auth"
)

const (
	tgPhone = "TG_PHONE"
)

type ClientState struct {
	Client      *telegram.Client
	Dispatcher  tg.UpdateDispatcher
	Gaps        *updates.Manager
	CurrentUser *tg.User
}

type ClientRunner = func(s ClientState) error

func StartClient(ctx context.Context, runner ClientRunner) error {
	phone := os.Getenv(tgPhone)
	if phone == "" {
		return fmt.Errorf("phone env variable %s is missing", tgPhone)
	}

	waiter := floodwait.NewWaiter().WithCallback(func(ctx context.Context, wait floodwait.FloodWait) {
		slog.Warn("FLOOD_WAIT", "retryAfter", wait.Duration)
	})

	dispatcher := tg.NewUpdateDispatcher()
	gaps := updates.New(updates.Config{
		Handler: dispatcher,
	})

	client, err := telegram.ClientFromEnvironment(telegram.Options{
		UpdateHandler: gaps,
		Middlewares: []telegram.Middleware{
			waiter,
			ratelimit.New(rate.Every(time.Millisecond*100), 5),
			hook.UpdateHook(gaps.Handle),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to init client with %w", err)
	}

	flow := tdauth.NewFlow(auth.TerminalUserAuthenticator{PhoneNumber: phone}, tdauth.SendCodeOptions{})

	return waiter.Run(ctx, func(ctx context.Context) error {
		return client.Run(ctx, func(ctx context.Context) error {
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
			slog.Info("logged in", "as", name)

			return runner(ClientState{
				Client:      client,
				Dispatcher:  dispatcher,
				Gaps:        gaps,
				CurrentUser: self,
			})
		})
	})
}
