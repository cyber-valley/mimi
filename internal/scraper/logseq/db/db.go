package db

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"slices"
	"strings"

	"github.com/cozodb/cozo-lib-go"
)

type Queries struct {
	db cozo.CozoDB
}

func New() *Queries {
	db, err := cozo.New("mem", "", nil)
	if err != nil {
		log.Fatalf("failed to connect to cozo with %s", err)
	}
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
		var props []string
		for name, value := range p.Props {
			props = append(props, fmt.Sprintf(
				`[%s,%s,%s]`,
				escape(p.Title),
				escape(name),
				escape(value),
			))
		}
		tx = append(
			tx,
			fmt.Sprintf(
				`?[page_title, name, value] <- [%s] :put page_prop{page_title, name, value}`,
				strings.Join(props, ", "),
			),
		)
	}

	// Save or update references
	if len(p.Refs) > 0 {
		var refs []string
		for _, ref := range p.Refs {
			refs = append(refs, fmt.Sprintf(
				`[%s,%s]`,
				escape(p.Title),
				escape(ref),
			))
		}
		tx = append(
			tx,
			fmt.Sprintf(
				`?[src, target] <- [%s] :put page_ref{src, target}`,
				strings.Join(refs, ", "),
			),
		)
	}

	// Execute queries in transaction
	err := execTx(q.db, tx)
	if err != nil {
		return fmt.Errorf("failed to save or update page '%s' with %w", p.Title, err)
	}
	return nil
}

// Page titles that are relative to the given one via ref
func (q *Queries) FindRelatives(pageTitle string, depth int) (titles []string, err error) {
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
				relatives[target, depth] 
		`,
		escape(pageTitle),
		depth,
	)
	res, err := q.db.Run(query, nil, true)
	if err != nil {
		return titles, fmt.Errorf("failed to find relatives for '%s' with %w", pageTitle, err)
	}
	for _, row := range res.Rows {
		titles = append(titles, row[0].(string))
	}
	return titles, nil
}

func (q *Queries) FindContentChanged(hash string) (bool, error) {
	query := fmt.Sprintf(
		`
		?[title] := *page{title, hash: %s}
		`,
		escape(hash),
	)
	res, err := q.db.Run(query, nil, false)
	if err != nil {
		return false, fmt.Errorf("failed to check existence of page '%s' with %w", hash, err)
	}
	return len(res.Rows) == 0, nil
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

func (q *Queries) FindContent(titles ...string) (contents []string, _ error) {
	query := fmt.Sprintf(`
		?[content] :=
			*page{title, content},
			title in %s
	`, escapeSlice(titles))
	res, err := q.db.Run(query, nil, false)
	if err != nil {
		return contents, fmt.Errorf("failed to find content with %w", err)
	}
	for _, row := range res.Rows {
		contents = append(contents, row[0].(string))
	}
	return contents, nil
}

func escape(s string) string {
	b, err := json.Marshal(strings.ReplaceAll(s, `"`, `'`))
	if err != nil {
		panic(err)
	}
	return string(b)
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
	}
	_, err := db.Run(strings.Join(wrapped, "\n"), nil, false)
	if err != nil {
		return fmt.Errorf("failed to execute transaction with %w", err)
	}
	return nil
}
