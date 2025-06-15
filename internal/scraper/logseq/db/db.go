package db

import (
	"crypto/sha256"
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
	db, err := cozo.New("sqlite", "cozo.db", nil)
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
			hash: String
		}`,
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
	sha := sha256.New()
	sha.Write([]byte(p.Content))
	h := fmt.Sprintf("%x", sha.Sum(nil))

	// Check if content changed
	changed, err := q.findContentChanged(h)
	if err != nil {
		return fmt.Errorf("failed to save page with %w", err)
	}
	if !changed {
		return nil
	}

	slog.Info("page content changed")

	var tx []string

	// Save or update page
	tx = append(tx, fmt.Sprintf(
		`?[title, content, hash] <- [[%s,%s,%s]] :put page{title, content, hash}`,
		escape(p.Title),
		escape(p.Content),
		escape(h),
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
	err = execTx(q.db, tx)
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

func (q *Queries) findContentChanged(hash string) (bool, error) {
	query := fmt.Sprintf(
		`
		?[title] := *page{title, hash: %s}
		`,
		escape(hash),
	)
	res, err := q.db.Run(query, nil, true)
	if err != nil {
		return false, fmt.Errorf("failed to check existence of page '%s' with %w", hash, err)
	}
	return len(res.Rows) == 0, nil
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
