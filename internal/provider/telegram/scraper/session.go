package scraper

import (
	"context"
	"errors"
	"fmt"

	"log/slog"

	"github.com/gotd/td/tg"
)

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
		slog.Error("failed to resolve channel peer", "error", err)
		return nil, err
	}
	slog.Info("channel resolved", "value", resp)
	chats := resp.(*tg.MessagesChats).Chats

	if l := len(chats); l != 1 {
		slog.Error("expected only one chat", "got", l)
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
		slog.Error("failed to get forum topics", "error", err)
		return nil, err
	}
	slog.Info("fetched forum topics", "value", topics)

	switch topic := topics.Topics[0].(type) {
	default:
		return nil, fmt.Errorf("unexpected topic type: %+v", chats)
	case *tg.ForumTopic:
		s.forumTopics[key] = topic
		return topic, nil
	}
}
