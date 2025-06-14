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

func New() *Queries {
	db, err := cozo.New("mem", "", nil)
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
	_, err = q.db.Run(":create page_ref { page_title: String, ref: String }", nil, false)
	if err != nil {
		return
	}
	_, err = q.db.Run(":create page_prop { page_title: String, name: String, value: String }", nil, false)
	if err != nil {
		return
	}
	return
}

type SavePageParams struct {
	Title   string
	Content string
	Props   map[string]string
	Refs    []string
}

func (q *Queries) SavePageParams(p SavePageParams) error {
	var tx []string

	// Save or update page
	tx = append(tx, fmt.Sprintf(
		`?[title, content] <- [["%s","%s"]] :put page{title, content}`,
		escape(p.Title),
		escape(p.Content),
	))

	// Save or update page properties
	if len(p.Props) > 0 {
		var props []string
		for name, value := range p.Props {
			props = append(props, fmt.Sprintf(
				`["%s","%s","%s"]`,
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
				`["%s","%s"]`,
				escape(p.Title),
				escape(ref),
			))
		}
		tx = append(
			tx,
			fmt.Sprintf(
				`?[page_title, ref] <- [%s] :put page_ref{page_title, ref}`,
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

func escape(s string) string {
	return strings.ReplaceAll(s, `"`, `\"`)
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
