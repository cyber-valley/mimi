package tgscraper

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"time"

	"github.com/golang/glog"
	"github.com/gotd/contrib/middleware/floodwait"
	"github.com/gotd/contrib/middleware/ratelimit"
	"github.com/gotd/td/telegram/updates"
	"github.com/gotd/td/telegram/updates/hook"
	"golang.org/x/time/rate"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
	"github.com/jackc/pgx/v5"

	"mimi/internal/persist"
	"mimi/internal/scraper/tgscraper/handle"
)

const (
	tgPhone = "TG_PHONE"
)

func Run(ctx context.Context, conn *pgx.Conn) error {
	phone := os.Getenv(tgPhone)
	if phone == "" {
		return fmt.Errorf("phone env variable %s is missing", tgPhone)
	}

	q := persist.New(conn)

	dispatcher := tg.NewUpdateDispatcher()

	gaps := updates.New(updates.Config{
		Handler: dispatcher,
	})

	waiter := floodwait.NewWaiter().WithCallback(func(ctx context.Context, wait floodwait.FloodWait) {
		glog.Warning("got FLOOD_WAIT. Will retry after", wait.Duration)
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
		glog.Error("failed to init client with ", err)
		return err
	}
	api := client.API()
	session := newSession()

	setupDispatcher(ctx, &dispatcher, client, q, session)

	flow := auth.NewFlow(terminalUserAuthenticator{PhoneNumber: phone}, auth.SendCodeOptions{})

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
			glog.Info("Current user:", name)

			return gaps.Run(ctx, api, self.ID, updates.AuthOptions{
				OnStart: func(ctx context.Context) {
					glog.Info("listening for events")
				},
			})
		}); err != nil {
			return err
		}
		return nil
	})
}

func setupDispatcher(ctx context.Context, d *tg.UpdateDispatcher, c *telegram.Client, q *persist.Queries, s *session) error {
	subscribeTo, err := q.FindChannelsToFollow(ctx)
	if err != nil {
		glog.Error("failed to find peers to subscribe ", err)
		return err
	}
	if len(subscribeTo) < 1 {
		err = errors.New("got empty peers to subscribe")
		glog.Error(err)
		return err
	}
	d.OnNewChannelMessage(func(ctx context.Context, e tg.Entities, u *tg.UpdateNewChannelMessage) error {
		msg, ok := u.Message.(*tg.Message)
		if !ok {
			return nil
		}
		channel, ok := msg.PeerID.(*tg.PeerChannel)
		if !ok {
			glog.Warning("failed to extract channel from ", msg.PeerID)
		}
		if slices.IndexFunc(subscribeTo, func(s persist.FindChannelsToFollowRow) bool {
			return s.ID == channel.ChannelID
		}) == -1 {
			return nil
		}
		replyTo, ok := msg.ReplyTo.(*tg.MessageReplyHeader)
		if !ok {
			glog.Warning("failed to extract reply to from ", msg.ReplyTo)
			return nil
		}
		var topic *tg.ForumTopic
		if replyTo.ForumTopic {
			t, err := s.resolveTopic(ctx, tg.NewClient(c), channel.ChannelID, replyTo.ReplyToMsgID)
			if err != nil {
				glog.Error("failed to resolve topic with: ", err)
				return err
			}
			topic = t
		}
		return handle.ChannelMessage(ctx, q, &handle.ChannelMessageRequest{
			Topic:   topic,
			Channel: channel,
			Msg:     msg,
		})
	})

	return nil
}
