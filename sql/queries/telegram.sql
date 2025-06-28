-- name: FindTelegramPeers :many
SELECT
    id,
    chat_name
FROM
    telegram_peer
WHERE
    enabled;

-- name: SaveTelegramTopic :exec
INSERT INTO
    telegram_topic (id, peer_id, title)
VALUES
    ($1, $2, $3) ON conflict (id, peer_id) DO
UPDATE
SET
    title = excluded.title;

-- name: SaveTelegramMessage :exec
INSERT INTO
    telegram_message (peer_id, topic_id, message)
VALUES
    ($1, $2, $3);

-- name: FindTelegramTopicDescription :one
SELECT
    description
FROM
    telegram_topic
WHERE
    peer_id = $1
    AND id = $2;

-- name: SaveTelegramTopicDescription :exec
UPDATE
    telegram_topic
SET
    description = $3
WHERE
    peer_id = $1
    AND id = $2;
