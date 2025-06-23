package query

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"math"
	"regexp"
	"slices"
	"strings"

	"github.com/aholstenson/logseq-go"
	"github.com/aholstenson/logseq-go/content"

	"mimi/internal/scraper/logseq/logseqext"
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
	refRegex                = regexp.MustCompile(`\[\[(.*)\]\]`)
	ErrRedundantPageProperty = fmt.Errorf("redundant 'page-property' statement")
	ErrIncorrectPageProperty = fmt.Errorf("incorrect 'page-property' statement")
	ErrNotSyntaxError        = fmt.Errorf("'not' accepts only one atom")
	ErrIncorrectPageTags     = fmt.Errorf("incorrect 'page-tags' statement")
	ErrIncorrectProperty     = fmt.Errorf("incorrect 'property' statement")
	ErrIncorrectAnd          = fmt.Errorf("incorrect 'property' statement")
)

func New() *State {
	return &State{}
}

func (s *State) Eval(ctx context.Context, g *logseq.Graph, q string) (res []logseq.Page, _ error) {
	// Parse query
	parsed, err := parseQuery(q)
	if err != nil {
		return res, fmt.Errorf("failed to parse query with %w", err)
	}

	// Evaluate state
	filter, err := s.eval(parsed)
	if err != nil {
		return res, fmt.Errorf("failed to evaluate state with %w", err)
	}

	// Read pages
	pages, err := getAllPages(ctx, g)
	if err != nil {
		return res, fmt.Errorf("failed to get graph pages with %w", err)
	}

	// Filter pages
	pageSet := make(map[string]logseq.Page)
	for _, page := range pages {
		if !filter(page) {
			continue
		}
		pageSet[page.Title()] = page
	}
	res = slices.Collect(maps.Values(pageSet))

	slog.Info("query result", "size", len(res))
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
			case "property":
				return s.evalProperty(sex)
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
		return emptyFilter, ErrIncorrectAnd
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
	if len(l) < 2 || len(l) > 3 {
		return emptyFilter, ErrIncorrectPageProperty
	}

	// Collect params
	cdr := make([]string, len(l)-1)
	for i := 1; i < len(l); i++ {
		atom, ok := l[i].I.(string)
		if !ok {
			return emptyFilter, fmt.Errorf("got unexpected 'page-property' value %#v", atom)
		}
		cdr[i-1] = atom
	}

	// Build filter
	return func(p logseq.Page) bool {
		propName := strings.TrimPrefix(cdr[0], ":")
		nodes := p.Properties().Get(propName)
		if len(cdr) == 1 {
			return len(nodes) > 0
		}
		propValue := cdr[1]
		for _, txt := range nodes.FilterDeep(content.IsOfType[*content.Text]()) {
			if txt.(*content.Text).Value == propValue {
				return true
			}
		}
		return false
	}, nil
}

func (s *State) evalPageTags(l sexp.List) (pageFilter, error) {
	if len(l) == 1 {
		return emptyFilter, ErrIncorrectPageTags
	}

	cdr := make([]string, len(l)-1)
	for i := 1; i < len(l); i++ {
		tag, ok := l[i].I.(string)
		if !ok {
			return emptyFilter, ErrIncorrectPageTags
		}
		cdr[i-1] = tag
	}
	slog.Info("tags to be queried", "value", cdr)

	return func(p logseq.Page) bool {
		tags := p.Properties().Get("tags")
		if len(tags) == 0 {
			return false
		}
		for _, tag := range tags {
			tag, ok := tag.(*content.Text)
			if !ok {
				slog.Error("got unexpected tag type", "value", fmt.Sprintf("%#v", tag))
				continue
			}
			if slices.Contains(cdr, tag.Value) {
				return true
			}
		}
		return false
	}, nil
}

func (s *State) evalProperty(l sexp.List) (pageFilter, error) {
	if len(l) < 2 || len(l) > 3 {
		return emptyFilter, ErrIncorrectProperty
	}

	// Collect params
	cdr := make([]string, len(l)-1)
	for i := 1; i < len(l); i++ {
		switch el := l[i].I.(type) {
		case string:
			cdr[i-1] = el
		case sexp.QString:
			// TODO: May face escaped sequences
			cdr[i-1] = strings.TrimPrefix(strings.TrimSuffix(string(el), `"`), `"`)
		default:
			return emptyFilter, fmt.Errorf("got unexpected 'property' value '%#v', list '%#v'", l[i].I, l)
		}
	}

	slog.Info("eval property", "cdr", cdr)

	// Build filter
	return func(p logseq.Page) bool {
		propName := strings.TrimPrefix(cdr[0], ":")

		// Check page props
		pageProps := p.Properties().Get(propName)
		if len(cdr) == 1 {
			// We need only property existence
			return len(pageProps) > 0
		} else {
			// Should ensure value match
			for _, prop := range pageProps {
				prop := prop.(*content.Text)
				if prop.Value == cdr[1] {
					return true
				}
			}
		}

		// Properties inside of lists are not parsed properly
		// so their match checked by simple string equality
		for prop := range logseqext.WalkPage[*content.Text](p) {
			var val string
			if len(cdr) > 1 {
				// We need only property existence
				val = fmt.Sprintf("%s:: %s", propName, cdr[1])
			} else {
				// Should ensure value match
				val = fmt.Sprintf("%s::", propName)
			}

			if strings.Contains(prop.Value, val) {
				return true
			}
		}

		return false
	}, nil
}

func (s *State) evalString(str string) (pageFilter, error) {
	match := refRegex.FindStringSubmatch(str)

	switch len(match) {
	case 0:
		return emptyFilter, fmt.Errorf("unexpected string atom '%s'", str)
	case 2:
		return func(p logseq.Page) bool {
			to := match[1]
			for ref := range logseqext.WalkPage[content.PageRef](p) {
				if ref.GetTo() == to {
					return true
				}
			}
			return false
		}, nil
	}

	return emptyFilter, fmt.Errorf("unexpected string atom '%s' match '%s'", str, match)
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

func getAllPages(ctx context.Context, g *logseq.Graph) (pages []logseq.Page, err error) {
	res, err := g.SearchPages(ctx, logseq.WithQuery(logseq.All()), logseq.WithMaxHits(math.MaxInt))
	if err != nil {
		return pages, fmt.Errorf("pages search failed with %w", err)
	}

	if res.Size() != res.Count() {
		return pages, fmt.Errorf("res amount differs from total count: %d != %d", res.Size(), res.Count())
	}

	for _, res := range res.Results() {
		page, err := res.Open()
		if err != nil {
			return pages, fmt.Errorf("failed to open page from result with %w", err)
		}
		pages = append(pages, page)
	}

	return pages, nil
}
