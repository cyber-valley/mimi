-- name: FindChatMessages :many
SELECT
    messages
FROM
    llm_chat
WHERE
    telegram_peer_id = $1;
