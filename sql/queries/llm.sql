-- name: FindChatMessages :one
SELECT
    messages
FROM
    llm_chat
WHERE
    telegram_peer_id = $1;

-- name: SaveChatMessages :exec
INSERT INTO
    llm_chat(telegram_peer_id, messages)
VALUES
    ($1, $2) ON conflict (telegram_peer_id) DO
UPDATE
SET
    messages = excluded.messages;
