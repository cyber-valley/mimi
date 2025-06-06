-- name: FindChannelsToFollow :many
SELECT
    id,
    chat_name
FROM
    telegram_peer
WHERE
    enabled;
