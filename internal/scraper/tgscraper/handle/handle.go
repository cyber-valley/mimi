package handle

import (
	"context"

	"github.com/golang/glog"
	"github.com/gotd/td/tg"

	"mimi/internal/persist"
)

type ChannelMessageRequest struct {
	Topic   *tg.ForumTopic
	Channel *tg.PeerChannel
	Msg     *tg.Message
}

func ChannelMessage(ctx context.Context, q *persist.Queries, r *ChannelMessageRequest) error {
	glog.Info("processing new channel message")
	return nil
}
