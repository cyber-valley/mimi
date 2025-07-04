package scraper

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/gotd/td/telegram/updates"
	"github.com/gotd/td/tg"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"mimi/internal/persist"
	"mimi/internal/provider/telegram"
)

const (
	telegramTopicDescriptionPrompt = "telegram-topic-description"
)

func Run(ctx context.Context, pool *pgxpool.Pool, g *genkit.Genkit) error {
	return telegram.StartClient(ctx, func(s telegram.ClientState) error {
		api := s.Client.API()

		// Check account validness & generate required data
		err := Setup(ctx, api, pool, g)
		if err != nil {
			return fmt.Errorf("failed to setup Telegram scraper with %w", err)
		}

		// Listen for updated
		setupDispatcher(ctx, &s.Dispatcher, s.Client.API(), g, pool, newSession())
		return s.Gaps.Run(ctx, s.Client.API(), s.CurrentUser.ID, updates.AuthOptions{
			OnStart: func(ctx context.Context) {
				slog.Info("listening for events")
			},
		})
	})
}

// setupDispatcher adds listeners for the telegram updates
//  1. Forum topics. If a new topic created, it should have a comprehensive description of itself as a first message
//     otherwise LLM wouldn't generate a good enough description
//  2. Groups. Simple message persistence
func setupDispatcher(ctx context.Context, d *tg.UpdateDispatcher, api *tg.Client, g *genkit.Genkit, pool *pgxpool.Pool, s *session) error {
	q := persist.New(pool)
	subscribeTo, err := q.FindTelegramPeers(ctx)
	if err != nil {
		slog.Error("failed to find peers to subscribe", "error", err)
		return err
	}
	if len(subscribeTo) < 1 {
		err = errors.New("got empty peers to subscribe")
		slog.Error("error", "error", err)
		return err
	}
	d.OnNewChannelMessage(func(ctx context.Context, e tg.Entities, u *tg.UpdateNewChannelMessage) error {
		// Validate types
		msg, ok := u.Message.(*tg.Message)
		if !ok {
			return nil
		}
		channel, ok := msg.PeerID.(*tg.PeerChannel)
		if !ok {
			slog.Warn("failed to extract channel from", "peer_id", msg.PeerID)
		}

		// Process only subscribed channels / groups
		if slices.IndexFunc(subscribeTo, func(s persist.FindTelegramPeersRow) bool {
			return s.ID == channel.ChannelID
		}) == -1 {
			return nil
		}

		// Extract forum topic info if exists
		replyTo, ok := msg.ReplyTo.(*tg.MessageReplyHeader)
		if !ok {
			slog.Warn("failed to extract reply to from", "reply_to", msg.ReplyTo)
			return nil
		}
		var topic *tg.ForumTopic
		if replyTo.ForumTopic {
			t, err := s.resolveTopic(ctx, api, channel.ChannelID, replyTo.ReplyToMsgID)
			if err != nil {
				slog.Error("failed to resolve topic", "error", err)
				return err
			}
			topic = t
		}

		// Begin transaction
		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("failed to begin transaction with %w", err)
		}
		defer tx.Rollback(ctx)

		qtx := q.WithTx(tx)

		// Save topic if does not exists
		if topic != nil {
			_, err := q.TelegramTopicExists(ctx, persist.TelegramTopicExistsParams{
				ID:     int32(topic.ID),
				PeerID: channel.ChannelID,
			})
			if err == pgx.ErrNoRows {
				// Resolve channel peer
				inputChannel := &tg.InputChannel{
					ChannelID:  channel.ChannelID,
					AccessHash: 0,
				}
				channels, err := api.ChannelsGetChannels(ctx, []tg.InputChannelClass{inputChannel})
				if err != nil {
					return fmt.Errorf("failed to resolve channel %d with %w", channel.ChannelID, err)
				}
				if len(channels.GetChats()) == 0 {
					return fmt.Errorf("no channels found")
				}
				channel, ok := channels.GetChats()[0].(*tg.Channel)
				if !ok {
					return fmt.Errorf("unexpected resolved channel type %#v", channels)
				}

				// Process new topic
				_, err = processNewTopic(ctx, g, qtx, api, channel, topic)
				if err != nil {
					return fmt.Errorf("failed to save telegram topic with %w", err)
				}
			}
		}

		// Save message
		var topicID pgtype.Int4
		if topic != nil {
			topicID.Int32 = int32(topic.ID)
			topicID.Valid = true
		}
		err = qtx.SaveTelegramMessage(ctx, persist.SaveTelegramMessageParams{
			ID:        int32(msg.ID),
			PeerID:    channel.ChannelID,
			TopicID:   topicID,
			Message:   msg.Message,
			CreatedAt: pgtype.Timestamptz{Time: time.Unix(int64(msg.Date), 0), Valid: true},
		})
		if err != nil {
			return fmt.Errorf("failed to save telegram message with %w", err)
		}

		return tx.Commit(ctx)
	})

	d.OnNewMessage(func(ctx context.Context, e tg.Entities, u *tg.UpdateNewMessage) error {
		// Validate types
		msg, ok := u.Message.(*tg.Message)
		if !ok {
			return nil
		}
		chat, ok := msg.PeerID.(*tg.PeerChat)
		if !ok {
			return fmt.Errorf("unexpected chat message peer %#v", msg)
		}

		// Process only subscribed channels / groups
		if slices.IndexFunc(subscribeTo, func(s persist.FindTelegramPeersRow) bool {
			return s.ID == chat.ChatID
		}) == -1 {
			return nil
		}

		// Persist message
		err = q.SaveTelegramMessage(ctx, persist.SaveTelegramMessageParams{
			ID:        int32(msg.ID),
			PeerID:    chat.ChatID,
			Message:   msg.Message,
			CreatedAt: pgtype.Timestamptz{Time: time.Unix(int64(msg.Date), 0), Valid: true},
		})
		if err != nil {
			return fmt.Errorf("failed to save telegram message with %w", err)
		}

		return nil
	})

	return nil
}

// Setup runs checks and saved required data into db
// 1. Ensures all required chats exist in the current Telegram account
// 2. Generates descriptions for the topics and persists them
// 3. Save last messages if they wasn't persisted
func Setup(ctx context.Context, api *tg.Client, db *pgxpool.Pool, g *genkit.Genkit) error {
	// Get required chats
	q := persist.New(db)
	chats, err := q.FindTelegramPeers(ctx)
	if err != nil {
		return fmt.Errorf("failed to get peers to follow with %w", err)
	}

	// Fetch chat's list
	dialogs, err := api.MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
		OffsetPeer: &tg.InputPeerEmpty{},
		Limit:      100,
	})
	if err != nil {
		return fmt.Errorf("failed to get dialogs with %w", err)
	}
	modifiedDialogs, ok := dialogs.AsModified()
	if !ok {
		return fmt.Errorf("got unexpected dialogs value %#v", dialogs)
	}

	// Check properly added chats to the current account
	foundChats := make([]bool, len(chats))
	for _, chat := range modifiedDialogs.GetChats() {
		chat, ok := chat.AsNotEmpty()
		if !ok {
			slog.Error("chat is empty, skipping", "value", chat)
			continue
		}
		chatIdx := slices.IndexFunc(chats, func(c persist.FindTelegramPeersRow) bool {
			return c.ID == chat.GetID()
		})
		if chatIdx < 0 {
			slog.Debug("skipping unknown chat", "title", chat.GetTitle(), "id", chat.GetID())
			continue
		}
		channel, ok := chat.(*tg.Channel)
		if ok {
			topics, err := getForumTopics(ctx, api, channel.ID, channel.AccessHash)
			if err != nil {
				slog.Error("chat", "value", fmt.Sprintf("%#v", chat))
				return err
			}

			tx, err := db.Begin(ctx)
			if err != nil {
				return fmt.Errorf("faield to begin transaction with %w", err)
			}
			defer tx.Rollback(ctx)

			qtx := q.WithTx(tx)

			for _, topic := range topics {
				_, err := q.TelegramTopicExists(ctx, persist.TelegramTopicExistsParams{
					PeerID: channel.ID,
					ID:     int32(topic.ID),
				})
				switch err {
				default:
					// Unexpected error
					return fmt.Errorf("failed to find telegram topic description with %w", err)
				case nil:
					// Description was already generated and saved
				case pgx.ErrNoRows:
					// Topic's description should be generated and saved
					messages, err := processNewTopic(ctx, g, qtx, api, channel, topic)
					if err != nil {
						return fmt.Errorf("failed to process new topic with %w", err)
					}
					for _, msg := range messages {
						err = qtx.SaveTelegramMessage(ctx, persist.SaveTelegramMessageParams{
							ID:        int32(msg.ID),
							TopicID:   pgtype.Int4{Int32: int32(topic.ID), Valid: true},
							PeerID:    channel.ID,
							Message:   msg.Message, // Message isn't empty which is ensured my `processNewTopic` impl
							CreatedAt: pgtype.Timestamptz{Time: time.Unix(int64(msg.Date), 0), Valid: true},
						})
						if err != nil {
							return fmt.Errorf("failed to save topic message %#v with %w", msg, err)
						}
					}
					time.Sleep(7 * time.Second)
				}
			}
			if err := tx.Commit(ctx); err != nil {
				return fmt.Errorf("failed to commit new topic transaction with %w", err)
			}
		}
		foundChats[chatIdx] = true
	}

	// Log missing chats
	var errs []error
	for i, found := range foundChats {
		if found {
			continue
		}
		errs = append(errs, fmt.Errorf("chat was not found in the current account: %#v", chats[i]))
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	slog.Info("all required chats were found")
	return nil
}

func getForumTopics(ctx context.Context, api *tg.Client, chatID, accessHash int64) ([]*tg.ForumTopic, error) {
	resp, err := api.ChannelsGetForumTopics(ctx, &tg.ChannelsGetForumTopicsRequest{
		Channel: &tg.InputChannel{
			ChannelID:  chatID,
			AccessHash: accessHash,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list channel topics with %w", err)
	}
	slog.Info("resolved topics", "length", len(resp.Topics))
	topics := make([]*tg.ForumTopic, len(resp.Topics))
	for i, topic := range resp.Topics {
		topic, ok := topic.(*tg.ForumTopic)
		if !ok {
			return nil, fmt.Errorf("unexpected type of forum topic: %#v", topic)
		}
		topics[i] = topic
	}
	return topics, nil
}

// processNewTopic retrieves last messages from the given topic, generates description based on them and persist topic entity
// returns all processed messages
func processNewTopic(ctx context.Context, g *genkit.Genkit, q *persist.Queries, api *tg.Client, channel *tg.Channel, topic *tg.ForumTopic) ([]*tg.Message, error) {
	// Get topic's messages
	msgReplies, err := api.MessagesGetReplies(ctx, &tg.MessagesGetRepliesRequest{
		Peer: &tg.InputPeerChannel{
			ChannelID:  channel.ID,
			AccessHash: channel.AccessHash,
		},
		MsgID: topic.ID,
		Limit: 50,
	})
	time.Sleep(1 * time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to get forum topic's messages. topic '%s', chat '%s', with %w", topic.Title, channel.Title, err)
	}
	topicMessages, ok := msgReplies.(*tg.MessagesChannelMessages)
	if !ok {
		return nil, fmt.Errorf("unexpected topic messages response type %#v", msgReplies)
	}

	summary, err := extractMessagesSummary(ctx, g, topicMessages.Messages)
	if err != nil {
		return nil, fmt.Errorf("failed to extract messages summary with %w", err)
	}

	// Save topic
	err = q.SaveTelegramTopic(ctx, persist.SaveTelegramTopicParams{
		PeerID:      channel.ID,
		ID:          int32(topic.ID),
		Title:       topic.Title,
		Description: summary.Description,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to save telegram topic description with %w", err)
	}

	return summary.Messages, nil
}

type messagesSummary struct {
	Description string
	Messages    []*tg.Message
}

func extractMessagesSummary(ctx context.Context, g *genkit.Genkit, messages []tg.MessageClass) (summary messagesSummary, _ error) {
	// Lookup prompt
	prompt := genkit.LookupPrompt(g, telegramTopicDescriptionPrompt)
	if prompt == nil {
		return summary, fmt.Errorf("no prompt named '%s' found", telegramTopicDescriptionPrompt)
	}

	// Prepare LLM prompt input
	var input telegramTopicDescriptionInput
loop:
	for _, msg := range messages {
		switch msg := msg.(type) {
		case *tg.Message:
			if msg.Message == "" {
				continue loop
			}
			summary.Messages = append(summary.Messages, msg)
			input.Messages = append(input.Messages, topicMessage{From: msg.FromID.String(), Text: msg.Message})
		case *tg.MessageService:
			// Someone was invited, kicked, etc.
		default:
			slog.Warn("got unexpected message type", "value", fmt.Sprintf("%#v", msg))
		}
	}
	slog.Info("extracting summary from non empty messages", "amount", len(input.Messages))

	// Evaluate LLM prompt
	resp, err := prompt.Execute(ctx, ai.WithInput(input))
	if err != nil {
		return summary, fmt.Errorf("failed to describe messages '%#v' with %w", input, err)
	}
	var output telegramTopicDescriptionOutput
	if err := resp.Output(&output); err != nil {
		return summary, fmt.Errorf("failed to deserialize LLM output with %w", err)
	}
	summary.Description = output.Description

	return summary, nil
}

type telegramTopicDescriptionInput struct {
	Messages []topicMessage `json:"messages"`
}

type topicMessage struct {
	From string `json:"from"`
	Text string `json:"text"`
}

type telegramTopicDescriptionOutput struct {
	Description string `json:"description"`
}
