package query

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"maps"
	"regexp"
	"slices"
	"strings"

	"mimi/internal/scraper/logseq"
	"mimi/internal/scraper/logseq/query/sexp"
)

type QueryResult struct {
	Head []string
	Rows [][]string
}

type pageFilter = func(pages logseq.Page) bool

var (
	queryRegex               = regexp.MustCompile(`\{\{query\s?(.*)\}\}`)
	queryOptRegex            = regexp.MustCompile(`\s*([\w\-_]+)::\s*(.*)`)
	queryOptPropertyValue    = regexp.MustCompile(`:([\w\-_]+)\s?`)
	linkRegex                = regexp.MustCompile(`\[\[(.*)\]\]`)
	ErrRedundantPageProperty = fmt.Errorf("redundant 'page-property' statement")
	ErrIncorrectPageProperty = fmt.Errorf("incorrect 'page-property' statement")
	ErrNotSyntaxError        = fmt.Errorf("'not' accepts only one atom")
	ErrIncorrectPageTags     = fmt.Errorf("incorrect 'page-tags' statement")
	ErrIncorrectProperty     = fmt.Errorf("incorrect 'property' statement")
	ErrIncorrectAnd          = fmt.Errorf("incorrect 'and' statement")
)

type QueryOptions struct {
	properties []string
	sortBy     string
	sortDesc   bool
}

func defaultQueryOptions() QueryOptions {
	return QueryOptions{
		properties: []string{"page"},
		sortBy:     "page",
		sortDesc:   true,
	}
}

type Option = func(*QueryOptions)

func WithProperties(props []string) Option {
	return func(opts *QueryOptions) {
		opts.properties = make([]string, len(props))
		for i, prop := range props {
			opts.properties[i] = strings.TrimLeft(prop, ":")
		}
	}
}

func WithSortBy(by string) Option {
	return func(opts *QueryOptions) {
		opts.sortBy = by
	}
}

func WithSortDesc(desc bool) Option {
	return func(opts *QueryOptions) {
		opts.sortDesc = desc
	}
}

type Result struct {
	Pages []logseq.Page
	// First slice is always a header
	// all others are rows
	Table [][]string
}

func Eval(ctx context.Context, g logseq.RegexGraph, q string) (res Result, _ error) {
	// Parse query
	parsed, err := parseQuery(q)
	if err != nil {
		return res, fmt.Errorf("failed to parse query with %w", err)
	}

	// Evaluate state
	filter, err := eval(parsed.s)
	if err != nil {
		return res, fmt.Errorf("failed to evaluate state with %w", err)
	}

	// Filter pages
	pageSet := make(map[string]logseq.Page)
	for page := range g.WalkPages() {
		if !filter(page) {
			continue
		}
		pageSet[page.Title()] = page
	}
	res.Pages = slices.Collect(maps.Values(pageSet))

	// Build table
	res.Table = buildTable(res.Pages, parsed.opts)

	return res, nil
}

func eval(sex sexp.Sexp) (pageFilter, error) {
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
				return evalAnd(sex)
			case "not":
				return evalNot(sex)
			case "page-property":
				return evalPageProperty(sex)
			case "page-tags":
				return evalPageTags(sex)
			case "property":
				return evalProperty(sex)
			default:
				return emptyFilter, fmt.Errorf("unexpected string list entry %s", head)
			}
		default:
			return emptyFilter, fmt.Errorf("unexpected list head type %#v", head)
		}
	case string:
		return evalString(sex)
	}

	return emptyFilter, fmt.Errorf("unexpected sexp format with value %#v", sex)
}

func evalAnd(l sexp.List) (pageFilter, error) {
	slog.Info("translating 'and' expression")
	if len(l) == 1 {
		return emptyFilter, ErrIncorrectAnd
	}
	filters := make([]pageFilter, len(l)-1)
	for i := 1; i < len(l); i++ {
		filter, err := eval(l[i])
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

func evalNot(l sexp.List) (pageFilter, error) {
	slog.Info("translating 'not' expression")
	if len(l) != 2 {
		return emptyFilter, ErrNotSyntaxError
	}
	filter, err := eval(l[1])
	if err != nil {
		return emptyFilter, fmt.Errorf("failed to eval 'not' operand with %w", err)
	}
	return func(p logseq.Page) bool {
		return !filter(p)
	}, nil
}

func evalPageProperty(l sexp.List) (pageFilter, error) {
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
		values, ok := p.Info.Get(propName)
		if !ok {
			return false
		}
		if len(cdr) == 1 {
			return len(values) > 0
		}
		propValue := cdr[1]
		for _, value := range values {
			if value == propValue {
				return true
			}
		}
		return false
	}, nil
}

func evalPageTags(l sexp.List) (pageFilter, error) {
	if len(l) == 1 {
		return emptyFilter, ErrIncorrectPageTags
	}

	cdr := make([]string, len(l)-1)
	for i := 1; i < len(l); i++ {
		tag, ok := l[i].I.(string)
		if !ok {
			return emptyFilter, ErrIncorrectPageTags
		}
		tag = logseq.ExtractReference(tag)
		cdr[i-1] = tag
	}
	slog.Info("tags to be queried", "value", cdr)

	return func(p logseq.Page) bool {
		tags, ok := p.Info.PageLevelTags()
		if !ok {
			return false
		}

		for _, tag := range tags {
			if slices.Contains(cdr, tag) {
				return true
			}
		}

		return false
	}, nil
}

func evalProperty(l sexp.List) (pageFilter, error) {
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
		pageProps, ok := p.Info.Get(propName)
		if !ok {
			return false
		}
		if len(cdr) == 1 {
			// We need only property existence
			return len(pageProps) > 0
		} else {
			// Should ensure value match
			if slices.Contains(pageProps, cdr[1]) {
				return true
			}
		}

		return false
	}, nil
}

func evalString(str string) (pageFilter, error) {
	match := linkRegex.FindStringSubmatch(str)

	switch len(match) {
	case 0:
		return emptyFilter, fmt.Errorf("unexpected string atom '%s'", str)
	case 2:
		return func(p logseq.Page) bool {
			tag := logseq.ExtractReference(match[1])
			if p.Title() == tag {
				return true
			}
			for _, prop := range p.Info.Props {
				if slices.Contains(prop.Values, tag) {
					return true
				}
			}
			return slices.Contains(p.Info.Refs, tag)
		}, nil
	}

	return emptyFilter, fmt.Errorf("unexpected string atom '%s' match '%s'", str, match)
}

type Query struct {
	s    sexp.Sexp
	opts QueryOptions
}

func parseQuery(q string) (res Query, err error) {
	slog.Info("parsing", "query", res)
	lines := strings.Split(q, "\n")

	// Strip {{query <actual query>}} formatting
	matches := queryRegex.FindSubmatch([]byte(q))
	switch len(matches) {
	case 2:
		q = string(matches[1])
	case 0:
		// Raw query or something unexpected that will fail later
		break
	default:
		return res, fmt.Errorf("got unexpected query format (%d matches) in '%s'", len(matches), q)
	}

	// Deserialize
	res.s, err = sexp.Parse(q)
	if err != nil {
		err = fmt.Errorf("failed to parse query with %w", err)
	} else {
		slog.Info("parsed", "query", fmt.Sprintf("%#v", res.s))
	}

	// Parse options
	res.opts = defaultQueryOptions()
	if len(lines) > 1 {
		for i := 1; i < len(lines); i++ {
			match := queryOptRegex.FindStringSubmatchIndex(lines[i])
			if match == nil {
				return res, fmt.Errorf("unknwon query option %s", lines[i])
			}
			value := lines[i][match[4]:match[5]]
			switch lines[i][match[2]:match[3]] {
			case "query-properties":
				match := queryOptPropertyValue.FindAllStringSubmatch(value, -1)
				if match == nil {
					return res, fmt.Errorf("unknwon 'query-properties' value %s", value)
				}
				res.opts.properties = make([]string, len(match))
				for i, match := range match {
					res.opts.properties[i] = match[1]
				}
			case "query-sort-by":
				res.opts.sortBy = value
			case "query-sort-desc":
				res.opts.sortDesc = value == "true"
			}
		}
	}

	return
}

func emptyFilter(page logseq.Page) bool {
	return false
}

func buildTable(pages []logseq.Page, queryOpts QueryOptions) [][]string {
	table := [][]string{queryOpts.properties}

	// Collect rows
	rows := make([][]string, len(pages))
	for i, page := range pages {
		rows[i] = make([]string, len(queryOpts.properties))
		for j, prop := range queryOpts.properties {
			switch prop {
			case "page":
				rows[i][j] = page.Title()
			default:
				props, ok := page.Info.PageLevelGet(prop)
				if !ok {
					rows[i][j] = ""
					continue
				}
				rows[i][j] = strings.Join(props, ", ")
			}
		}
	}

	// Sort table
	sortByIdx := slices.Index(queryOpts.properties, queryOpts.sortBy)

	slices.SortFunc(rows, func(lhs, rhs []string) int {
		res := cmp.Compare(lhs[sortByIdx], rhs[sortByIdx])
		if queryOpts.sortDesc {
			return -1 * res
		}
		return res
	})

	table = slices.Concat(table, rows)
	return table
}
