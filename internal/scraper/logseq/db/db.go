package db

import (
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"strings"

	"github.com/cozodb/cozo-lib-go"
)

type Queries struct {
	db cozo.CozoDB
}

func New() *Queries {
	db, err := cozo.New("sqlite", "cozo.db", nil)
	if err != nil {
		log.Fatalf("failed to connect to cozo with %s", err)
	}
	return &Queries{
		db: db,
	}
}

func (q *Queries) CreateRelations() {
	var err error

	_, err = q.db.Run(":create page { title: String => content: String, embedding: <F32; 1536> }", nil, false)
	if err != nil {
		slog.Warn("failed to create 'relation' table", "with", err)
	}
	_, err = q.db.Run(":create page_ref { src: String, target: String }", nil, false)
	if err != nil {
		slog.Warn("failed to create 'page_ref' relation", "with", err)
	}
	_, err = q.db.Run(":create page_prop { page_title: String, name: String, value: String }", nil, false)
	if err != nil {
		slog.Warn("failed to create 'page_prop' relation", "with", err)
	}
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

func escape(s string) string {
	b, err := json.Marshal(strings.ReplaceAll(s, `"`, `'`))
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
