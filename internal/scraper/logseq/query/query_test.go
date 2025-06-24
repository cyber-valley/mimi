package query

import (
	"log/slog"
	"testing"

	"mimi/internal/scraper/logseq"
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
	}
	s := New()
	g := logseq.NewRegexGraph(graphPath)

	for q, expected := range query2expected {
		pages, err := s.Eval(t.Context(), g, q)
		if err != nil {
			t.Errorf("failed to eval query '%s' with %s", q, err)
		}
		if len(pages) != expected {
			t.Errorf("got %d pages instead of %d for '%s'", len(pages), expected, q)
		}
	}
}

func TestEval_Fails(t *testing.T) {
	before2expected := map[string]string{
		`{{query (and [] (page-tags [[species]]) (not (page-tags [[class]])))}}`: `failed to evaluate state with failed to evaluate 'and' with unexpected string atom '[]'`,
	}

	s := New()
	g := logseq.NewRegexGraph(graphPath)

	for before, expected := range before2expected {
		_, err := s.Eval(t.Context(), g, before)
		if err == nil || err.Error() != expected {
			t.Errorf("query '%s' should fail with '%s' but failed with '%s'", before, expected, err)
			t.Fail()
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
	errs := make(map[string]error)
	for _, q := range queries {
		_, err := parseQuery(q)
		if err != nil {
			errs[q] = err
		}
	}
	if len(errs) > 0 {
		for q, err := range errs {
			slog.Error("failed", "query", q, "with", err)
		}
		t.Fail()
	}
}
