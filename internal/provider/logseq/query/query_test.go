package query

import (
	"slices"
	"testing"

	"mimi/internal/provider/logseq"
)

const (
	// FIXME: Simplify graph and store in the project
	graphPath = "/home/user/code/clone/cvland"
)

func TestEval(t *testing.T) {
	query2expected := map[string]int{
		`{{query (property :supply "next-month")}}`:  76,
		`{{query (page-property :wood-durability)}}`: 35,
		`{{query [[@master]]}}`:                      7,
		`{{query (page-tags [[psycho]])}}`:           5,
		`{{query (and (page-tags [[species]]) (not (page-tags [[class]])) (and (page-tags [[supply]])))}}`: 2,
		`{{query (and (page-tags [[genus]]))}}`:             364,
		`{{query (and [[fern]] (page-tags [[species]] ))}}`: 14,
	}
	g := logseq.NewRegexGraph(graphPath)

	for q, expected := range query2expected {
		res, err := Eval(t.Context(), g, q)
		if err != nil {
			t.Errorf("failed to eval query '%s' with %s", q, err)
		}
		if len(res.Pages) != expected {
			titles := make([]string, len(res.Pages))
			for i, page := range res.Pages {
				titles[i] = page.Title()
			}
			t.Errorf("got %d pages instead of %d for '%s', titles: %#v", len(res.Pages), expected, q, titles)
		}
	}
}

func TestEval_Fails(t *testing.T) {
	before2expected := map[string]string{
		`{{query (and [] (page-tags [[species]]) (not (page-tags [[class]])))}}`: `failed to evaluate state with failed to evaluate 'and' with unexpected string atom '[]'`,
	}

	g := logseq.NewRegexGraph(graphPath)

	for before, expected := range before2expected {
		_, err := Eval(t.Context(), g, before)
		if err == nil || err.Error() != expected {
			t.Errorf("query '%s' should fail with '%s' but failed with '%s'", before, expected, err)
		}
	}
}

func TestParsing_RawEDN(t *testing.T) {
	queries := []string{
		`{{query (page-property :wood-durability)}}`,
		`{{query [[@master]]}}`,
		`{{query (page-tags [[psycho]])}}`,
		`{{query (and [] (page-tags [[species]]) (not (page-tags [[class]])))}}`,
		`{{query (and (property :supply "next-month") (property :project "edible oils"))}}`,
		`{{query (and [[conifer]] (and) (page-tags [[genus]]))}}`,
		`{{query (and (page-tags [[genus]]) (not (page-tags [[class]])) (not (page-tags [[research]])) (not (page-tags [[prohibited]])))}}`,
		`{{query (page-property :type "sector")}}`,
	}
	for _, q := range queries {
		_, err := parseQuery(q)
		if err != nil {
			t.Errorf("failed query %s with %s", q, err)
		}
	}
}

func TestParsing_Options(t *testing.T) {
	query2expected := map[string]QueryOptions{
		`{{query (page-tags [[super]])}}
  query-properties:: [:page :tags :alias]
  query-sort-by:: page
	query-sort-desc:: true`: QueryOptions{
			properties: []string{"page", "tags", "alias"},
			sortBy:     "page",
			sortDesc:   true,
		},
	}

	for query, expected := range query2expected {
		got, err := parseQuery(query)
		if err != nil {
			t.Errorf("failed to parse query %s with %s", query, err)
		}
		if got.opts.sortBy != expected.sortBy {
			goto testFailed
		}

		if got.opts.sortDesc != expected.sortDesc {
			goto testFailed
		}

		if !slices.Equal(got.opts.properties, expected.properties) {
			goto testFailed
		}

		continue

	testFailed:
		t.Errorf("got %#v, expected %#v", got, expected)
	}
}

func TestBuildTable(t *testing.T) {
	pages := [][]logseq.Page{
		[]logseq.Page{
			logseq.Page{
				Path: "foo.md",
				Info: logseq.PageInfo{
					Props: []logseq.Property{
						logseq.Property{
							Name:   "tags",
							Values: []string{"1", "2", "3"},
							Level:  logseq.PageLevel,
						},
						logseq.Property{
							Name:   "alias",
							Values: []string{"bar"},
							Level:  logseq.PageLevel,
						},
					},
					Refs: []string{},
				},
			},
			logseq.Page{
				Path: "bar.md",
				Info: logseq.PageInfo{
					Props: []logseq.Property{
						logseq.Property{
							Name:   "tags",
							Values: []string{"baz", "huz"},
							Level:  logseq.PageLevel,
						},
					},
					Refs: []string{},
				},
			},
		},
		[]logseq.Page{
			logseq.Page{Path: "foo.md"},
			logseq.Page{Path: "bar.md"},
			logseq.Page{Path: "buz.md"},
		},
	}
	opts := []QueryOptions{
		QueryOptions{
			properties: []string{"page", "tags", "alias"},
			sortBy:     "alias",
			sortDesc:   true,
		},
		QueryOptions{
			properties: []string{"page"},
			sortBy:     "page",
			sortDesc:   false,
		},
	}
	expected := [][][]string{
		[][]string{
			[]string{"page", "tags", "alias"},
			[]string{"foo", "1, 2, 3", "bar"},
			[]string{"bar", "baz, huz", ""},
		},
		[][]string{
			[]string{"page"},
			[]string{"bar"},
			[]string{"buz"},
			[]string{"foo"},
		},
	}

	if len(pages) != len(opts) || len(opts) != len(expected) {
		t.Errorf("wrong test setup")
	}

	for i := 0; i < len(pages); i++ {
		t.Logf("Checking %#v", opts[i])
		table := buildTable(pages[i], opts[i])
		if slices.EqualFunc(table, expected[i], slices.Equal) {
			continue
		}
		t.Errorf("expected %#v, got %#v", expected[i], table)
	}
}
