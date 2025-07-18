// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.29.0
// source: github.sql

package persist

import (
	"context"
)

const findGitHubRepositories = `-- name: FindGitHubRepositories :many
SELECT
    owner,
    name
FROM
    github_repository
`

func (q *Queries) FindGitHubRepositories(ctx context.Context) ([]GithubRepository, error) {
	rows, err := q.db.Query(ctx, findGitHubRepositories)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []GithubRepository
	for rows.Next() {
		var i GithubRepository
		if err := rows.Scan(&i.Owner, &i.Name); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
