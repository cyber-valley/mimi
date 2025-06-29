-- Contains information about telegram groups and channels
CREATE TABLE IF NOT EXISTS telegram_peer (
    id bigint PRIMARY KEY,
    chat_name text NOT NULL,
    description text,
    -- Set to false if given peer should not be used
    enabled boolean NOT NULL DEFAULT TRUE
);

-- Megagroups in telegram i.e. forums contain different topics
CREATE TABLE IF NOT EXISTS telegram_topic (
    id int,
    peer_id bigint NOT NULL REFERENCES telegram_peer(id) ON DELETE CASCADE,
    title text NOT NULL,
    description text NOT NULL,
    PRIMARY KEY (id, peer_id)
);

-- Each telegram peer contains multiple messages
CREATE TABLE IF NOT EXISTS telegram_message (
    id int,
    peer_id bigint NOT NULL,
    topic_id int,
    message text NOT NULL,
    created_at timestamp WITH time zone DEFAULT NOW(),
    PRIMARY KEY (id, peer_id),
    FOREIGN KEY (topic_id, peer_id) REFERENCES telegram_topic(id, peer_id) ON DELETE CASCADE,
    FOREIGN KEY (peer_id) REFERENCES telegram_peer(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS llm_chat (
    telegram_id bigint PRIMARY KEY,
    messages jsonb NOT NULL
);
