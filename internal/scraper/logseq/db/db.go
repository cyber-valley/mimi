package db

import (
	"fmt"
	"github.com/cozodb/cozo-lib-go"
	"log"
	"strings"
)

type Queries struct {
	db cozo.CozoDB
}

type Page struct {
	Title   string
	Content string
	Props   map[string]string
	Refs    []string
}

func New() *Queries {
	db, err := cozo.New("sqlite", "test.db", nil)
	if err != nil {
		log.Fatalf("failed to connect to cozo with %s", err)
	}
	return &Queries{
		db: db,
	}
}

func (q *Queries) CreateRelations() (err error) {
	_, err = q.db.Run(":create page { title: String => content: String }", nil, false)
	if err != nil {
		return
	}
	_, err = q.db.Run(":create page_ref { page_title: String, page_ref: String }", nil, false)
	if err != nil {
		return
	}
	_, err = q.db.Run(":create page_prop { page_title: String, name: String, value: String }", nil, false)
	if err != nil {
		return
	}
	return
}

func (q *Queries) SavePage(p Page) error {
	var queries []string
	// Save or update page
	queries = append(queries, fmt.Sprintf(
		`:put page {title: "%s", content: "%s"}`,
		escape(p.Title),
		escape(p.Content),
	))

	// Save or update page properties
	if len(p.Props) > 0 {
		var props []string
		for name, value := range p.Props {
			props = append(props, fmt.Sprintf(
				`{page_title: "%s", name: "%s", value: "%s"}`,
				escape(p.Title),
				escape(name),
				escape(value),
			))
		}
		queries = append(queries, fmt.Sprintf(`:put page_prop %s`, strings.Join(props, ", ")))
	}

	// Save or update references
	if len(p.Refs) > 0 {
		var refs []string
		for _, ref := range p.Refs {
			refs = append(refs, fmt.Sprintf(
				`{page_title: "%s", page_ref: "%s"}`,
				escape(p.Title),
				escape(ref),
			))
		}
		queries = append(queries, fmt.Sprintf(`:put page_ref %s`, strings.Join(refs, ", ")))
	}

	// Execute queries in transaction
	err := execTx(q.db, queries)
	if err != nil {
		return fmt.Errorf("failed to save or update page '%s' with %w", p.Title, err)
	}
	return nil
}

func escape(s string) string {
	return strings.ReplaceAll(s, `"`, `\"`)
}

func execTx(db cozo.CozoDB, queries []string) error {
	wrapped := make([]string, len(queries))
	for i, query := range queries {
		wrapped[i] = fmt.Sprintf("{%s}", query)
	}
	_, err := db.Run(strings.Join(wrapped, "\n"), nil, false)
	return err
}
