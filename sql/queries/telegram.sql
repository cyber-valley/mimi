-- name: FindChannelsToFollow :many
SELECT
    id,
    chat_name
FROM
    telegram_peer
WHERE
    enabled;

-- name: SaveTelegramMessage :exec
INSERT INTO
    telegram_message (peer_id, topic_id, message)
VALUES
    ($1, $2, $3);
