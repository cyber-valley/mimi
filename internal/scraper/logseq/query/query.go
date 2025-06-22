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

type State struct {
	filter pageFilter
}

type pageFilter struct{}

var (
	queryRegex      = regexp.MustCompile(`\{\{query\s?(.*)\}\}`)
	mentionRegex    = regexp.MustCompile(`\[\[\@(.*)\]\]`)
	ErrRedundantAnd = fmt.Errorf("redundant and statement")
)

func New() *State {
	return &State{}
}

func (s *State) Eval(q string) (res QueryResult, _ error) {
	// Parse query
	parsed, err := parseQuery(q)
	if err != nil {
		return res, fmt.Errorf("failed to parse query with %w", err)
	}

	// Evaluate state
	err = s.eval(parsed)
	if err != nil {
		return res, fmt.Errorf("failed to evaluate state with %w", err)
	}

	// Execute filtering
	res, err = s.filter.execute()
	if err != nil {
		return res, fmt.Errorf("failed to execute page filter with %w", err)
	}

	return res, nil
}

func (s *State) eval(sex sexp.Sexp) error {
	switch sex := sex.I.(type) {
	default:
		return fmt.Errorf("unexpected sexp format with value %#v", sex)
	case sexp.List:
		// Most of the query logic sits inside of a list
		if len(sex) == 0 {
			slog.Warn("got empty list")
			break
		}
		switch head := sex[0].I.(type) {
		case string:
			// Find out filter and execute it
			switch head {
			case "and":
				err := s.evalAnd(sex)
				if err != nil {
					return fmt.Errorf("failed to execute 'and' with %w", err)
				}
			case "page-property":
				err := s.evalPageProperty(sex)
				if err != nil {
					return fmt.Errorf("failed to execute 'page-property' with %w", err)
				}
			case "page-tags":
				err := s.evalPageTags(sex)
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
		err := s.evalString(sex)
		if err != nil {
			return fmt.Errorf("failed to execute string with %w", err)
		}
	}
	return nil
}

func (s *State) evalAnd(l sexp.List) error {
	if len(l) == 1 {
		return ErrRedundantAnd
	}
	return nil
}

func (s *State) evalPageProperty(l sexp.List) error {
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

func (s *State) evalPageTags(l sexp.List) error {
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

func (s *State) evalString(str string) error {
	if !mentionRegex.MatchString(str) {
		return fmt.Errorf("unexpected string atom '%s'", str)
	}
	slog.Info("got mention filter")
	return nil
}

func (f pageFilter) execute() (QueryResult, error) {
	panic("not implemented")
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
