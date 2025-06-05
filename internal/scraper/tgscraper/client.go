package tgscraper

import (
	"context"
	"errors"
	"fmt"
	"os"
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
)

const (
	tgPhone = "TG_PHONE"
)

func Run(ctx context.Context) error {
	phone := os.Getenv(tgPhone)
	if phone == "" {
		return fmt.Errorf("phone env variable %s is missing", tgPhone)
	}

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
		return err
	}
	api := client.API()

	dispatcher.OnNewChannelMessage(func(ctx context.Context, e tg.Entities, u *tg.UpdateNewChannelMessage) error {
		msg, ok := u.Message.(*tg.Message)
		if !ok {
			return nil
		}
		channel, ok := msg.PeerID.(*tg.PeerChannel)
		if !ok {
			glog.Warning("failed to extract channel from ", msg.PeerID)
		}
		replyTo, ok := msg.ReplyTo.(*tg.MessageReplyHeader)
		if !ok {
			glog.Warning("failed to extract reply to from ", msg.ReplyTo)
			return nil
		}
		glog.Info("reply to ", replyTo)
		if replyTo.ForumTopic {
			c := &tg.InputChannel{ChannelID: channel.ChannelID, AccessHash: 0}
			raw := tg.NewClient(client)
			resp, err := raw.ChannelsGetChannels(ctx, []tg.InputChannelClass{c})
			if err != nil {
				glog.Error("failed to resolve channel peer with ", err)
				return err
			}
			glog.Info("channel resolved to ", resp)
			chats := resp.(*tg.MessagesChats).Chats

			if l := len(chats); l != 1 {
				glog.Error("expected only one chat but got ", l)
				return errors.New("unexpected resolve of channel")
			}

			inputChan := chats[0].(*tg.Channel)

			c = &tg.InputChannel{
				ChannelID:  channel.ChannelID,
				AccessHash: inputChan.AccessHash,
			}
			req := &tg.ChannelsGetForumTopicsByIDRequest{
				Channel: c,
				Topics:  []int{replyTo.ReplyToMsgID},
			}

			topics, err := raw.ChannelsGetForumTopicsByID(ctx, req)
			if err != nil {
				glog.Error("failed to get forum topics with", err)
				return err
			}
			glog.Info("fetched forum topics", topics)
		}
		return handleNewChannelMessage(client, msg)
	})

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

func handleNewChannelMessage(c *telegram.Client, msg *tg.Message) error {
	glog.Infof("new channel message: %s", msg)
	return nil
}
