package query

import (
	"log/slog"
	"testing"
)

func TestEval(t *testing.T) {
	s := New()
	queries := []string{
		`{{query (page-property :wood-durability)}}`,
		`{{query [[@master]]}}`,
		`{{query (page-tags [[psycho]])}}`,
		`{{query (and (page-tags [[species]]) (not (page-tags [[class]])))}}`,
	}
	errs := make(map[string]error)
	for _, q := range queries {
		_, err := s.Eval(q)
		if err != nil {
			errs[q] = err
		}
	}
	if len(errs) > 0 {
		for q, err := range errs {
			t.Log("failed", "query", q, "with", err)
		}
		t.Fail()
	}
}

func TestEval_Fails(t *testing.T) {
	s := New()
	before2expected := map[string]string{
		`{{query (and [] (page-tags [[species]]) (not (page-tags [[class]])))}}`: `failed to evaluate state with failed to evaluate 'and' with unexpected string atom '[]'`,
	}
	for before, expected := range before2expected {
		_, err := s.Eval(before)
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
