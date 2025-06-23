package logseq

import (
	"slices"
	"strings"
	"testing"
)

const (
	page1 = `alias:: damiana
tags:: species, research, psycho

- supply:: next-month`
)

func TestFindProperties(t *testing.T) {
	page2expected := map[string][]Property{
		page1: []Property{
			Property{
				Name:   "alias",
				Values: []string{"damiana"},
				Level:  PageLevel,
			},
			Property{
				Name:   "tags",
				Values: []string{"species", "research", "psycho"},
				Level:  PageLevel,
			},
			Property{
				Name:   "supply",
				Values: []string{"next-month"},
				Level:  BlockLevel,
			},
		},
	}

	for page, expected := range page2expected {
		properties, err := FindProperties(strings.NewReader(page))
		if err != nil {
			t.Errorf("failed to find properties with %s", err)
		}
		if slices.EqualFunc(properties.Result, expected, func(lhs, rhs Property) bool {
			return lhs.Name == rhs.Name && lhs.Level == rhs.Level && slices.Equal(lhs.Values, rhs.Values)
		}) {
			continue
		}

		t.Errorf("got different properties, expected %#v, got %#v", expected, properties)
	}
}

func TestProperties_Tags(t *testing.T) {
	page2expected := map[string][]string{
		page1: []string{"species", "research", "psycho"},
	}

	for page, expected := range page2expected {
		properties, err := FindProperties(strings.NewReader(page))
		if err != nil {
			t.Errorf("failed to find properties with %s", err)
		}

		tags, _ := properties.AllTags()
		if slices.Equal(tags, expected) {
			continue
		}

		t.Errorf("expected %#v, got %#v", expected, tags)
	}
}
