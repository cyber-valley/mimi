package query

import (
	"fmt"
	"log/slog"
	"regexp"

	"github.com/aholstenson/logseq-go"

	"mimi/internal/scraper/logseq/query/sexp"
)

type QueryResult struct {
	Head []string
	Rows [][]string
}

type State struct{}

type pageFilter = func(pages logseq.Page) bool

var (
	queryRegex      = regexp.MustCompile(`\{\{query\s?(.*)\}\}`)
	mentionRegex    = regexp.MustCompile(`\[\[\@(.*)\]\]`)
	ErrRedundantAnd = fmt.Errorf("redundant 'and' statement")
	ErrEmptyNot     = fmt.Errorf("empty 'not' statement")
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
	_, err = s.eval(parsed)
	if err != nil {
		return res, fmt.Errorf("failed to evaluate state with %w", err)
	}

	return res, nil
}

func (s *State) eval(sex sexp.Sexp) (pageFilter, error) {
	switch sex := sex.I.(type) {
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
				return s.evalAnd(sex)
			case "not":
				return s.evalNot(sex)
			case "page-property":
				return s.evalPageProperty(sex)
			case "page-tags":
				return s.evalPageTags(sex)
			default:
				return emptyFilter, fmt.Errorf("unexpected string list entry %s", head)
			}
		default:
			return emptyFilter, fmt.Errorf("unexpected list head type %#v", head)
		}
	case string:
		return s.evalString(sex)
	}

	return emptyFilter, fmt.Errorf("unexpected sexp format with value %#v", sex)
}

func (s *State) evalAnd(l sexp.List) (pageFilter, error) {
	if len(l) == 1 {
		return emptyFilter, ErrRedundantAnd
	}
	filters := make([]pageFilter, len(l)-2)
	for i := 1; i < len(l); i++ {
		filter, err := s.eval(l[i])
		if err != nil {
			return emptyFilter, fmt.Errorf("failed to evaluate 'and' with %w", err)
		}
		filters[i-1] = filter
	}
	return conjuction(filters...), nil
}

func (s *State) evalNot(l sexp.List) (pageFilter, error) {
	if len(l) == 1 {
		return emptyFilter, ErrEmptyNot
	}
	panic("not implemented")
}

func (s *State) evalPageProperty(l sexp.List) (pageFilter, error) {
	switch len(l) {
	case 0:
		return emptyFilter, fmt.Errorf("got empty page property list")
	case 1:
		slog.Info("should filter all pages with", "property", l[0])
	default:
		prop := l[0]
		for i := 1; i < len(l); i++ {
			value := l[i]
			slog.Info("should filter pages with", "property", prop, "value", value)
		}
	}
	panic("not implemented")
}

func (s *State) evalPageTags(l sexp.List) (pageFilter, error) {
	switch len(l) {
	case 0:
		return emptyFilter, fmt.Errorf("got empty page tags list")
	case 1:
		slog.Info("should filter all pages with", "tag", l[0])
	default:
		for _, tag := range l {
			slog.Info("should filter pages with", "tag", tag)
		}
	}
	panic("not implemented")
}

func (s *State) evalString(str string) (pageFilter, error) {
	if !mentionRegex.MatchString(str) {
		return emptyFilter, fmt.Errorf("unexpected string atom '%s'", str)
	}
	slog.Info("got mention filter")
	panic("not implemented")
}

func conjuction(filters ...pageFilter) pageFilter {
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

func emptyFilter(pages logseq.Page) bool {
	return false
}
