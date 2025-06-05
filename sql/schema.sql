CREATE EXTENSION vector;

CREATE TABLE embedding (
  id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
  text TEXT NOT NULL,
  metadata JSONB NOT NULL,
  embedding vector(1536)
);

);
