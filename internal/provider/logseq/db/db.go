package db

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/cozodb/cozo-lib-go"
)

type Queries struct {
	db cozo.CozoDB
}

func New(db cozo.CozoDB) *Queries {
	return &Queries{
		db: db,
	}
}

func (q *Queries) CreateRelations() error {
	var errs []error

	res, err := q.db.Run("::relations", nil, true)
	if err != nil {
		return fmt.Errorf("failed to get relations list with %w", err)
	}
	relations := make([]string, len(res.Rows))
	for i, row := range res.Rows {
		relations[i] = row[0].(string)
	}

	schema := map[string]string{
		"page": `:create page {
			title: String
			=>
			content: String,
		}
		`,
		"page_ref": `:create page_ref {
			src: String,
			target: String
		}`,
		"page_prop": `:create page_prop {
			page_title: String,
			name: String,
			value: String
		}`,
	}

	for name, query := range schema {
		if slices.Contains(relations, name) {
			slog.Info("relation already exists", "name", name)
			continue
		}
		_, err = q.db.Run(query, nil, false)
		if err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

type SavePageParams struct {
	Title   string
	Content string
	Props   map[string]string
	Refs    []string
}

func (q *Queries) SavePage(p SavePageParams) error {
	var tx []string

	// Save or update page
	tx = append(tx, fmt.Sprintf(
		`?[title, content] <- [[%s,%s]] :put page{title, content}`,
		escape(p.Title),
		escape(p.Content),
	))

	// Save or update page properties
	if len(p.Props) > 0 {
		for name, value := range p.Props {
			tx = append(
				tx,
				fmt.Sprintf(
					`?[page_title, name, value] <- [[%s,%s,%s]] :put page_prop{page_title, name, value}`,
					escape(p.Title),
					escape(name),
					escape(value),
				),
			)
		}
	}

	// Save or update references
	if len(p.Refs) > 0 {
		for _, ref := range p.Refs {
			tx = append(
				tx,
				fmt.Sprintf(
					`?[src, target] <- [[%s,%s]] :put page_ref{src, target}`,
					escape(p.Title),
					escape(ref),
				),
			)
		}
	}

	// Execute queries in transaction
	err := execTx(q.db, tx)
	if err != nil {
		return fmt.Errorf("failed to save or update page '%s' with %w", p.Title, err)
	}
	return nil
}

type FindRelativesRow struct {
	Title   string
	Content string
}

// Page titles that are relative to the given one via ref
func (q *Queries) FindRelatives(pageTitle string, depth int) (rows []FindRelativesRow, err error) {
	query := fmt.Sprintf(
		`
		relatives[target, depth] :=
			*page_ref{src: %s, target},
			depth = 1

		relatives[target, depth] :=
				relatives[new_src, d],
				*page_ref{src: new_src, target},
				depth = d + 1,
				depth <= %d

		?[target, depth] :=
				relatives[target, depth],
				*page{title: target}
		`,
		escape(pageTitle),
		depth,
	)
	res, err := q.db.Run(query, nil, true)
	if err != nil {
		return rows, fmt.Errorf("failed to find relatives for '%s' with %w", pageTitle, err)
	}
	for _, row := range res.Rows {
		rows = append(rows, FindRelativesRow{
			Title: row[0].(string),
		})
	}
	return rows, nil
}

type SimilarPageRow struct {
	Dist    float64
	Title   string
	Content string
}

func (q *Queries) FindSimilarPages(vec []float32) (pages []SimilarPageRow, _ error) {
	query := fmt.Sprintf(
		`
		?[dist, title, content] := 
			~page:embedding_index3{title, content |
				query: q,
				k: 50,
				ef: 20,
				bind_distance: dist
			}, 
			q = vec(%s)
		:order dist
		:limit 20
		`,
		escapeSlice(vec),
	)
	res, err := q.db.Run(query, nil, false)
	if err != nil {
		return pages, fmt.Errorf("failed to find similar pages with %w", err)
	}
	for _, row := range res.Rows {
		dist, ok := row[0].(float64)
		if !ok {
			dist = 0
		}
		pages = append(pages, SimilarPageRow{
			Dist:    dist,
			Title:   row[1].(string),
			Content: row[2].(string),
		})
	}
	return pages, nil
}

func (q *Queries) FindTitles() (titles []string, _ error) {
	res, err := q.db.Run("?[title] := *page{title}", nil, false)
	if err != nil {
		return titles, fmt.Errorf("failed to find similar pages with %w", err)
	}
	slog.Info("retrieved titles", "len", len(res.Rows))
	for _, row := range res.Rows {
		titles = append(titles, row[0].(string))
	}
	return titles, nil
}

func escape(s string) string {
	// Weird escaping happens if double quotes pased into the sequence somehow
	return fmt.Sprintf(`"%s"`, strings.ReplaceAll(s, `"`, "'"))
}

func escapeSlice[T any](s []T) string {
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func execTx(db cozo.CozoDB, queries []string) error {
	wrapped := make([]string, len(queries))
	for i, q := range queries {
		wrapped[i] = fmt.Sprintf("{%s}", q)
		_, err := db.Run(q, nil, false)
		if err != nil {
			return fmt.Errorf("failed to execute transaction query '%s' with %w", q, err)
		}
	}
	return nil
}
