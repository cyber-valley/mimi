CREATE EXTENSION vector;

CREATE TABLE embedding (
  id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
  text TEXT NOT NULL,
  metadata JSONB NOT NULL,
  embedding vector(1536)
);

CREATE TABLE telegram_peers (
  peer_id bigint PRIMARY KEY,
  enabled boolean NOT NULL DEFAULT false
);
