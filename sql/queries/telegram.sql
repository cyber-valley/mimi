-- name: FindTelegramPeers :many
SELECT
    id,
    chat_name
FROM
    telegram_peer
WHERE
    enabled;

-- name: FindTelegramPeersWithTopics :many
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
    p.enabled;

-- name: FindTelegramMessages :many
SELECT
    m.message,
    p.chat_name AS chat_name,
    t.title AS topic_title
FROM
    telegram_message m
    INNER JOIN telegram_peer p ON m.peer_id = p.id
    JOIN telegram_topic t ON m.topic_id = t.id;

-- name: SaveTelegramTopic :exec
INSERT INTO
    telegram_topic (id, peer_id, title, description)
VALUES
    ($1, $2, $3, $4) ON conflict (id, peer_id) DO
UPDATE
SET
    title = excluded.title,
    description = excluded.description;

-- name: SaveTelegramMessage :exec
INSERT INTO
    telegram_message (id, peer_id, topic_id, message)
VALUES
    ($1, $2, $3, $4);

-- name: TelegramTopicExists :one
SELECT
    1
FROM
    telegram_topic
WHERE
    peer_id = $1
    AND id = $2;
