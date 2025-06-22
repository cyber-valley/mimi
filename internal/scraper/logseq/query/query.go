package query

import (
	"bytes"
	"fmt"
	"log/slog"
	"regexp"

	"olympos.io/encoding/edn"
)

type QueryResult struct {
	Head []string
	Rows [][]string
}

var (
	queryRegex = regexp.MustCompile(`\{\{query\s?(.*)\}\}`)
)

func Execute(query string) (QueryResult, error) {
	panic("not implemented")
}

func parseQuery(q []byte) (parsed any, err error) {
	slog.Info("parsing", "query", q)
	// Strip {{query <actual query>}} formatting
	matches := queryRegex.FindSubmatch(q)
	switch len(matches) {
	case 2:
		q = matches[1]
	case 0:
		// Raw query or something unexpected that will fail later
		break
	default:
		return parsed, fmt.Errorf("got unexpected query format (%d matches) in '%s'", len(matches), q)
	}

	// Replace mentions (they not allowed by EDN)
	q = bytes.Replace(q, []byte("@"), []byte("atmention_"), -1)

	// Finally deserialize
	err = edn.Unmarshal(q, &parsed)
	if err != nil {
		err = fmt.Errorf("failed to parse query with %w", err)
	} else {
		slog.Info("parsed", "query", fmt.Sprintf("%#v", parsed))
	}
	return
}
