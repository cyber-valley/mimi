package query

import (
	"fmt"
	"log/slog"
	"regexp"

	"mimi/internal/scraper/logseq/query/sexp"
)

type QueryResult struct {
	Head []string
	Rows [][]string
}

var (
	queryRegex = regexp.MustCompile(`\{\{query\s?(.*)\}\}`)
)

func Execute(q string) (QueryResult, error) {
	parsed, err := parseQuery(q)
	if err != nil {
		return QueryResult{}, fmt.Errorf("failed to parse query with %w", err)
	}
	switch p := parsed.I.(type) {
	case sexp.List:
		for _, item := range p {
			slog.Info("got parsed item", "value", item)
		}
	}
	return QueryResult{}, nil
}

func parseQuery(q string) (s sexp.Sexp, err error) {
	slog.Info("parsing", "query", q)

	// Strip {{query <actual query>}} formatting
	matches := queryRegex.FindSubmatch([]byte(q))
	switch len(matches) {
	case 2:
		q = string(matches[1])
	case 0:
		// Raw query or something unexpected that will fail later
		break
	default:
		return s, fmt.Errorf("got unexpected query format (%d matches) in '%s'", len(matches), q)
	}

	// Deserialize
	s, err = sexp.Parse(q)
	if err != nil {
		err = fmt.Errorf("failed to parse query with %w", err)
	} else {
		slog.Info("parsed", "query", fmt.Sprintf("%#v", s))
	}

	return
}
