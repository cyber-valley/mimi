package tgscraper

import (
	"context"
	"errors"
	"fmt"

	"github.com/golang/glog"

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
