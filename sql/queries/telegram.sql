-- TODO: Rename channels part
-- name: FindChannelsToFollow :many
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
