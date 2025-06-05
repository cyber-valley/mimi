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

	"mimi/persist"
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
	session := newSession()

	dispatcher.OnNewChannelMessage(func(ctx context.Context, e tg.Entities, u *tg.UpdateNewChannelMessage) error {
		msg, ok := u.Message.(*tg.Message)
		if !ok {
			return nil
		}
		channel, ok := msg.PeerID.(*tg.PeerChannel)
		if !ok {
			glog.Warning("failed to extract channel from ", msg.PeerID)
		}
		if !slices.Contains(subscribeTo, channel.ChannelID) {
			return nil
		}
		replyTo, ok := msg.ReplyTo.(*tg.MessageReplyHeader)
		if !ok {
			glog.Warning("failed to extract reply to from ", msg.ReplyTo)
			return nil
		}
		glog.Info("reply to ", replyTo)
		if replyTo.ForumTopic {
			topic, err := session.resolveTopic(ctx, tg.NewClient(client), channel.ChannelID, replyTo.ReplyToMsgID)
			if err != nil {
				glog.Error("failed to resolve topic with: ", err)
				return err
			}
			glog.Info("resolved topic: ", topic)
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

type session struct {
	forumTopics map[topicKey]*tg.ForumTopic
}

type topicKey struct {
	gID int64
	tID int
}

func newSession() *session {
	return &session{
		forumTopics: make(map[topicKey]*tg.ForumTopic),
	}
}

func (s *session) resolveTopic(ctx context.Context, raw *tg.Client, groupID int64, topicID int) (*tg.ForumTopic, error) {
	key := topicKey{groupID, topicID}
	topic, ok := s.forumTopics[key]
	if ok {
		return topic, nil
	}
	ic := &tg.InputChannel{ChannelID: groupID, AccessHash: 0}
	resp, err := raw.ChannelsGetChannels(ctx, []tg.InputChannelClass{ic})
	if err != nil {
		glog.Error("failed to resolve channel peer with ", err)
		return nil, err
	}
	glog.Info("channel resolved to ", resp)
	chats := resp.(*tg.MessagesChats).Chats

	if l := len(chats); l != 1 {
		glog.Error("expected only one chat but got ", l)
		return nil, errors.New("unexpected resolve of channel")
	}

	c, ok := chats[0].(*tg.Channel)
	if !ok {
		return nil, fmt.Errorf("unexpected result of resolving: %+v", chats)
	}

	req := &tg.ChannelsGetForumTopicsByIDRequest{
		Channel: &tg.InputChannel{
			ChannelID:  ic.ChannelID,
			AccessHash: c.AccessHash,
		},
		Topics: []int{topicID},
	}
	topics, err := raw.ChannelsGetForumTopicsByID(ctx, req)
	if err != nil {
		glog.Error("failed to get forum topics with", err)
		return nil, err
	}
	glog.Info("fetched forum topics", topics)

	switch topic := topics.Topics[0].(type) {
	default:
		return nil, fmt.Errorf("unexpected topic type: %+v", chats)
	case *tg.ForumTopic:
		s.forumTopics[key] = topic
		return topic, nil
	}
}

func handleNewChannelMessage(c *telegram.Client, msg *tg.Message) error {
	glog.Infof("new channel message: %s", msg)
	return nil
}
