package logseq

import (
	"slices"
	"strings"
	"testing"
)

func TestFindProperties(t *testing.T) {
	page2expected := map[string][]Property{
		`alias:: damiana
tags:: species, research, psycho

- supply:: next-month`: []Property{
			Property{
				Name:   "alias",
				Values: []string{"damiana"},
				Level:  "page",
			},
			Property{
				Name:   "tags",
				Values: []string{"species", "research", "psycho"},
				Level:  "page",
			},
			Property{
				Name:   "supply",
				Values: []string{"next-month"},
				Level:  "block",
			},
		},
	}

	for page, expected := range page2expected {
		properties, err := FindProperties(strings.NewReader(page))
		if err != nil {
			t.Errorf("failed to find properties with %s", err)
		}
		if slices.EqualFunc(properties, expected, func(lhs, rhs Property) bool {
			return lhs.Name == rhs.Name && lhs.Level == rhs.Level && slices.Equal(lhs.Values, rhs.Values)
		}) {
			continue
		}

		t.Errorf("got different properties, expected %#v, got %#v", expected, properties)
	}
}
