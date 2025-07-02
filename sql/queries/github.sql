-- name: FindGitHubRepositories :many
SELECT
    owner,
    name
FROM
    github_repository;
