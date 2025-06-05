CREATE EXTENSION vector;

CREATE TABLE IF NOT EXISTS embedding (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
  text TEXT NOT NULL,
  metadata JSONB NOT NULL,
  embedding vector(1536)
);

CREATE TABLE IF NOT EXISTS telegram_peers (
  peer_id bigint PRIMARY KEY,
  enabled boolean NOT NULL DEFAULT true
);
