CREATE EXTENSION vector;

CREATE TABLE IF NOT EXISTS embedding (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    text TEXT NOT NULL,
    metadata JSONB NOT NULL,
    embedding vector(1536)
);

CREATE TABLE IF NOT EXISTS telegram_peer (
    id bigint PRIMARY KEY,
    chat_name text NOT NULL,
    enabled boolean NOT NULL DEFAULT TRUE
);

CREATE TABLE IF NOT EXISTS telegram_message (
    peer_id bigint PRIMARY KEY REFERENCES telegram_peer(id),
    message text NOT NULL,
    created_at timestamp WITH time zone DEFAULT NOW()
);
