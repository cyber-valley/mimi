package query

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/aholstenson/logseq-go"
	"github.com/aholstenson/logseq-go/content"

	"mimi/internal/scraper/logseq/query/sexp"
)

type QueryResult struct {
	Head []string
	Rows [][]string
}

type State struct{}

type pageFilter = func(pages logseq.Page) bool

var (
	queryRegex               = regexp.MustCompile(`\{\{query\s?(.*)\}\}`)
	mentionRegex             = regexp.MustCompile(`\[\[\@(.*)\]\]`)
	ErrRedundantAnd          = fmt.Errorf("redundant 'and' statement")
	ErrRedundantPageProperty = fmt.Errorf("redundant 'page-property' statement")
	ErrIncorrectPageProperty = fmt.Errorf("incorrect 'page-property' statement")
	ErrNotSyntaxError        = fmt.Errorf("'not' accepts only one atom")
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
	slog.Info("translating 'and' expression")
	if len(l) == 1 {
		return emptyFilter, ErrRedundantAnd
	}
	filters := make([]pageFilter, len(l)-1)
	for i := 1; i < len(l); i++ {
		filter, err := s.eval(l[i])
		if err != nil {
			return emptyFilter, fmt.Errorf("failed to evaluate 'and' with %w", err)
		}
		filters[i-1] = filter
	}
	return func(p logseq.Page) bool {
		for _, filter := range filters {
			if !(filter(p)) {
				return false
			}
		}
		return true
	}, nil
}

func (s *State) evalNot(l sexp.List) (pageFilter, error) {
	slog.Info("translating 'not' expression")
	if len(l) != 2 {
		return emptyFilter, ErrNotSyntaxError
	}
	filter, err := s.eval(l[1])
	if err != nil {
		return emptyFilter, fmt.Errorf("failed to eval 'not' operand with %w", err)
	}
	return func(p logseq.Page) bool {
		return !filter(p)
	}, nil
}

func (s *State) evalPageProperty(l sexp.List) (pageFilter, error) {
	slog.Info("translating 'page-property' expression")
	if len(l) == 1 {
		return emptyFilter, ErrRedundantPageProperty
	}

	if len(l) > 3 {
		return emptyFilter, ErrIncorrectPageProperty
	}

	// Collect params
	cdr := make([]string, len(l)-1)
	for i := 1; i < len(l); i++ {
		atom, ok := l[i].I.(string)
		if !ok {
			return emptyFilter, fmt.Errorf("got unexpected 'page-property' value %#v", atom)
		}
		cdr[i] = atom
	}

	// Build filter
	return func(p logseq.Page) bool {
		propName = strings.TrimPrefix(cdr[0], ":")
		nodes := props.Get(propName)
		if len(cdr) == 1 {
			return len(nodes) > 0
		}
		propValue = cdr[1]
		for _, txt := range nodes.FilterDeep(content.IsOfType[*content.Text]()) {
			if txt.(*content.Text).Value == propValue {
				return true
			}
		}
		return false
	}, nil
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
