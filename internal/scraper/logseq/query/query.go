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
		return res, fmt.Errorf("failed to parse query with %w", err)
	}
	err = execute(parsed)
	if err != nil {
		return res, fmt.Errorf("failed to execute sexp with %w", err)
	}
	return res, nil
}

func execute(s sexp.Sexp) error {
	switch s := s.I.(type) {
	default:
		return fmt.Errorf("unexpected sexp format with value %#v", s)
	case sexp.List:
		if len(s) == 0 {
			slog.Warn("got empty list")
			break
		}
		switch head := s[0].I.(type) {
		case string:
			switch head {
			case "and":
				err := executeAnd(s)
				if err != nil {
					return fmt.Errorf("failed to execute 'and' with %w", err)
				}
			case "page-property":
				err := executePageProperty(s)
				if err != nil {
					return fmt.Errorf("failed to execute 'page-property' with %w", err)
				}
			case "page-tags":
				err := executePageTags(s)
				if err != nil {
					return fmt.Errorf("failed to execute 'page-tags' with %w", err)
				}
			default:
				return fmt.Errorf("unexpected string list entry %s", head)
			}
		default:
			return fmt.Errorf("unexpected list head type %#v", head)
		}
	case string:
		err := executeString(s)
		if err != nil {
			return fmt.Errorf("failed to execute string with %w", err)
		}
	}
	return nil
}

func executeAnd(l sexp.List) error {
	return nil
}

func executePageProperty(l sexp.List) error {
	switch len(l) {
	case 0:
		return fmt.Errorf("got empty page property list")
	case 1:
		slog.Info("should filter all pages with", "property", l[0])
	default:
		prop := l[0]
		for i := 1; i < len(l); i++ {
			value := l[i]
			slog.Info("should filter pages with", "property", prop, "value", value)
		}
	}
	return nil
}

func executePageTags(l sexp.List) error {
	switch len(l) {
	case 0:
		return fmt.Errorf("got empty page tags list")
	case 1:
		slog.Info("should filter all pages with", "tag", l[0])
	default:
		for _, tag := range l {
			slog.Info("should filter pages with", "tag", tag)
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
