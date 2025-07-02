CREATE TABLE IF NOT EXISTS github_repository (
    owner text NOT NULL,
    name text NOT NULL,
    PRIMARY KEY (owner, name)
);
