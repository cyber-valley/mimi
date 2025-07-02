CREATE TABLE IF NOT EXISTS llm_chat (
    telegram_id bigint PRIMARY KEY,
    messages jsonb NOT NULL
);
