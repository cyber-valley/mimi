// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.29.0
// source: telegram.sql

package persist

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

const findTelegramMessages = `-- name: FindTelegramMessages :many
SELECT
    m.message,
    p.chat_name AS chat_name,
    t.title AS topic_title
FROM
    telegram_message m
    INNER JOIN telegram_peer p ON m.peer_id = p.id
    JOIN telegram_topic t ON m.topic_id = t.id
WHERE
    m.created_at >= $1
`

type FindTelegramMessagesRow struct {
	Message    string
	ChatName   string
	TopicTitle string
}

func (q *Queries) FindTelegramMessages(ctx context.Context, createdAt pgtype.Timestamptz) ([]FindTelegramMessagesRow, error) {
	rows, err := q.db.Query(ctx, findTelegramMessages, createdAt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []FindTelegramMessagesRow
	for rows.Next() {
		var i FindTelegramMessagesRow
		if err := rows.Scan(&i.Message, &i.ChatName, &i.TopicTitle); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const findTelegramPeers = `-- name: FindTelegramPeers :many
SELECT
    id,
    chat_name
FROM
    telegram_peer
WHERE
    enabled
`

type FindTelegramPeersRow struct {
	ID       int64
	ChatName string
}

func (q *Queries) FindTelegramPeers(ctx context.Context) ([]FindTelegramPeersRow, error) {
	rows, err := q.db.Query(ctx, findTelegramPeers)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []FindTelegramPeersRow
	for rows.Next() {
		var i FindTelegramPeersRow
		if err := rows.Scan(&i.ID, &i.ChatName); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const findTelegramPeersWithTopics = `-- name: FindTelegramPeersWithTopics :many
SELECT
    p.id AS chat_id,
    p.chat_name AS chat_name,
    p.description AS chat_description,
    t.id AS topic_id,
    t.title AS topic_title,
    t.description AS topic_description
FROM
    telegram_peer p
    JOIN telegram_topic t ON t.peer_id = p.id
WHERE
    p.enabled
`

type FindTelegramPeersWithTopicsRow struct {
	ChatID           int64
	ChatName         string
	ChatDescription  pgtype.Text
	TopicID          int32
	TopicTitle       string
	TopicDescription string
}

func (q *Queries) FindTelegramPeersWithTopics(ctx context.Context) ([]FindTelegramPeersWithTopicsRow, error) {
	rows, err := q.db.Query(ctx, findTelegramPeersWithTopics)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []FindTelegramPeersWithTopicsRow
	for rows.Next() {
		var i FindTelegramPeersWithTopicsRow
		if err := rows.Scan(
			&i.ChatID,
			&i.ChatName,
			&i.ChatDescription,
			&i.TopicID,
			&i.TopicTitle,
			&i.TopicDescription,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const saveTelegramMessage = `-- name: SaveTelegramMessage :exec
INSERT INTO
    telegram_message (id, peer_id, topic_id, message, created_at)
VALUES
    ($1, $2, $3, $4, $5)
`

type SaveTelegramMessageParams struct {
	ID        int32
	PeerID    int64
	TopicID   pgtype.Int4
	Message   string
	CreatedAt pgtype.Timestamptz
}

func (q *Queries) SaveTelegramMessage(ctx context.Context, arg SaveTelegramMessageParams) error {
	_, err := q.db.Exec(ctx, saveTelegramMessage,
		arg.ID,
		arg.PeerID,
		arg.TopicID,
		arg.Message,
		arg.CreatedAt,
	)
	return err
}

const saveTelegramTopic = `-- name: SaveTelegramTopic :exec
INSERT INTO
    telegram_topic (id, peer_id, title, description)
VALUES
    ($1, $2, $3, $4) ON conflict (id, peer_id) DO
UPDATE
SET
    title = excluded.title,
    description = excluded.description
`

type SaveTelegramTopicParams struct {
	ID          int32
	PeerID      int64
	Title       string
	Description string
}

func (q *Queries) SaveTelegramTopic(ctx context.Context, arg SaveTelegramTopicParams) error {
	_, err := q.db.Exec(ctx, saveTelegramTopic,
		arg.ID,
		arg.PeerID,
		arg.Title,
		arg.Description,
	)
	return err
}

const telegramTopicExists = `-- name: TelegramTopicExists :one
SELECT
    1
FROM
    telegram_topic
WHERE
    peer_id = $1
    AND id = $2
`

type TelegramTopicExistsParams struct {
	PeerID int64
	ID     int32
}

func (q *Queries) TelegramTopicExists(ctx context.Context, arg TelegramTopicExistsParams) (int32, error) {
	row := q.db.QueryRow(ctx, telegramTopicExists, arg.PeerID, arg.ID)
	var column_1 int32
	err := row.Scan(&column_1)
	return column_1, err
}
