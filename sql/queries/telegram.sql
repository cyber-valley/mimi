-- name: FindChannelsToFollow :many
SELECT
    peer_id
FROM
    telegram_peers
WHERE
    enabled;
