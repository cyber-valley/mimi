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
	queryRegex   = regexp.MustCompile(`\{\{query\s?(.*)\}\}`)
	mentionRegex = regexp.MustCompile(`\[\[\@(.*)\]\]`)
)

func Execute(q string) (res QueryResult, _ error) {
	parsed, err := parseQuery(q)
	if err != nil {
		return QueryResult{}, fmt.Errorf("failed to parse query with %w", err)
	}
	switch p := parsed.I.(type) {
	default:
		return res, fmt.Errorf("unexpected sexp format with value %#v", p)
	case sexp.List:
		if len(p) == 0 {
			slog.Warn("got empty list")
			break
		}
		switch head := p[0].I.(type) {
		case string:
			switch head {
			case "page-property":
				err = executePageProperty(p)
				if err != nil {
					return res, fmt.Errorf("failed to execute page property with %w", err)
				}
			default:
				return res, fmt.Errorf("unexpected string list entry %s", head)
			}
		default:
			return res, fmt.Errorf("unexpected list head type %#v", head)
		}
	case string:
		err = executeString(p)
		if err != nil {
			return res, fmt.Errorf("failed to execute string with %w", err)
		}
	}
	return QueryResult{}, nil
}

func executePageProperty(l sexp.List) error {
	switch len(l) {
	case 0:
		return fmt.Errorf("got empty page property list")
	case 1:
		slog.Info("should filter all pages", "tag", l[0])
	default:
		tag := l[0]
		for i := 1; i < len(l); i++ {
			value := l[i]
			slog.Info("should filter all pages", "tag", tag, "value", value)
		}
	}
	return nil
}

func executeString(s string) error {
	if !mentionRegex.MatchString(s) {
		return fmt.Errorf("unexpected string atom '%s'", s)
	}
	slog.Info("got mention filter")
	return nil
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
