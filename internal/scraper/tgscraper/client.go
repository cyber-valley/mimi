package tgscraper

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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
		return errors.New("no phone")
	}
	// APP_HASH, APP_ID is from https://my.telegram.org/.
	appID, err := strconv.Atoi(os.Getenv(tgAppID))
	if err != nil {
		return errors.Wrap(err, " parse app id")
	}
	appHash := os.Getenv(tgAppHash)
	if appHash == "" {
		return errors.New("no app hash")
	}

	// Setting up session storage.
	// This is needed to reuse session and not login every time.
	sessionDir := filepath.Join("session", sessionFolder(phone))
	if err := os.MkdirAll(sessionDir, 0700); err != nil {
		return err
	}

	glog.Infof("Storing session in %s", sessionDir)

	// So, we are storing session information in current directory, under subdirectory "session/phone_hash"
	sessionStorage := &telegram.FileSessionStorage{
		Path: filepath.Join(sessionDir, "session.json"),
	}

	// Setting up client.
	//
	// Dispatcher is used to register handlers for events.
	dispatcher := tg.NewUpdateDispatcher()
	gaps := updates.New(updates.Config{
		Handler: dispatcher,
	})

	// Handler of FLOOD_WAIT that will automatically retry request.
	waiter := floodwait.NewWaiter().WithCallback(func(ctx context.Context, wait floodwait.FloodWait) {
		// Notifying about flood wait.
		glog.Info("Got FLOOD_WAIT. Will retry after", wait.Duration)
	})

	// Filling client options.
	options := telegram.Options{
		SessionStorage: sessionStorage, // Setting up session sessionStorage to store auth data.
		UpdateHandler:  gaps,
		Middlewares: []telegram.Middleware{
			// Setting up FLOOD_WAIT handler to automatically wait and retry request.
			waiter,
			// Setting up general rate limits to less likely get flood wait errors.
			ratelimit.New(rate.Every(time.Millisecond*100), 5),
			updhook.UpdateHook(gaps.Handle),
		},
	}
	client := telegram.NewClient(appID, appHash, options)
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

func sessionFolder(phone string) string {
	var out []rune
	for _, r := range phone {
		if r >= '0' && r <= '9' {
			out = append(out, r)
		}
	}
	return "phone-" + string(out)
}
