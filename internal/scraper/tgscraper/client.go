package tgscraper

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/go-faster/errors"
	"github.com/golang/glog"
	"github.com/gotd/contrib/middleware/floodwait"
	"github.com/gotd/contrib/middleware/ratelimit"
	"github.com/gotd/td/telegram/updates"
	updhook "github.com/gotd/td/telegram/updates/hook"
	"golang.org/x/time/rate"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
)

const (
	tgAppID    = "TG_APP_ID"
	tgAppHash  = "TG_APP_HASH"
	tgBotToken = "TG_BOT_TOKEN"
	tgPhone    = "TG_PHONE"
)

func Run(ctx context.Context) error {
	flag.Lookup("stderrthreshold").Value.Set("INFO")
	flag.Parse()

	phone := os.Getenv(tgPhone)
	if phone == "" {
		return errors.New(fmt.Sprintf("phone env variable %s is missing", tgPhone))
	}

	dispatcher := tg.NewUpdateDispatcher()
	gaps := updates.New(updates.Config{
		Handler: dispatcher,
	})

	waiter := floodwait.NewWaiter().WithCallback(func(ctx context.Context, wait floodwait.FloodWait) {
		glog.Warning("Got FLOOD_WAIT. Will retry after", wait.Duration)
	})

	client, err := telegram.ClientFromEnvironment(telegram.Options{
		UpdateHandler: gaps,
		Middlewares: []telegram.Middleware{
			waiter,
			ratelimit.New(rate.Every(time.Millisecond*100), 5),
			updhook.UpdateHook(gaps.Handle),
		},
	})
	if err != nil {
		return err
	}
	api := client.API()

	// Registering handler for new private messages.
	dispatcher.OnNewMessage(func(ctx context.Context, e tg.Entities, u *tg.UpdateNewMessage) error {
		msg, ok := u.Message.(*tg.Message)
		if !ok {
			return nil
		}
		if msg.Out {
			// Outgoing message.
			return nil
		}

		glog.Infof("new message: %s", msg.Message)
		return nil
	})

	// Authentication flow handles authentication process, like prompting for code and 2FA password.
	flow := auth.NewFlow(terminalUserAuthenticator{PhoneNumber: phone}, auth.SendCodeOptions{})

	return waiter.Run(ctx, func(ctx context.Context) error {
		// Spawning main goroutine.
		if err := client.Run(ctx, func(ctx context.Context) error {
			// Perform auth if no session is available.
			if err := client.Auth().IfNecessary(ctx, flow); err != nil {
				return errors.Wrap(err, "auth")
			}

			// Getting info about current user.
			self, err := client.Self(ctx)
			if err != nil {
				return errors.Wrap(err, "call self")
			}

			name := self.FirstName
			if self.Username != "" {
				// Username is optional.
				name = fmt.Sprintf("%s (@%s)", name, self.Username)
			}
			fmt.Println("Current user:", name)

			// Editing some stuff
			return gaps.Run(ctx, api, self.ID, updates.AuthOptions{
				OnStart: func(ctx context.Context) {
					glog.Info("listening for events")
				},
			})
		}); err != nil {
			return errors.Wrap(err, "run")
		}
		return nil
	})
}
