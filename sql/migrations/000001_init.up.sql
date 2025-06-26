-- Contains information about telegram groups and channels
CREATE TABLE IF NOT EXISTS telegram_peer (
    id bigint PRIMARY KEY,
    chat_name text NOT NULL,
    -- Set to false if given peer should not be used
    enabled boolean NOT NULL DEFAULT TRUE
);

-- Megagroups in telegram i.e. forums contain different topics
CREATE TABLE IF NOT EXISTS telegram_topic (
    id int,
    peer_id bigint NOT NULL REFERENCES telegram_peer(id),
    title text NOT NULL,
    PRIMARY KEY (id, peer_id)
);

-- Each telegram peer contains multiple messages
CREATE TABLE IF NOT EXISTS telegram_message (
    peer_id bigint PRIMARY KEY REFERENCES telegram_peer(id),
    topic_id int NULL,
    message text NOT NULL,
    created_at timestamp WITH time zone DEFAULT NOW(),
    FOREIGN KEY (peer_id, topic_id) REFERENCES telegram_topic(peer_id, id)
);

CREATE TABLE IF NOT EXISTS llm_chat (
    telegram_id bigint PRIMARY KEY,
    messages jsonb NOT NULL
);
