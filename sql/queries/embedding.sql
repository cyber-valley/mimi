-- name: SaveEmbedding :one
INSERT INTO
    embedding (text, metadata, embedding)
VALUES
    ($1, $2, $3)
RETURNING
    id;

-- name: FindCosine :many
SELECT
    id,
    text,
    metadata
FROM
    embedding
WHERE
    embedding <=> $1;
