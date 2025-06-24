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

func TestFindPageInfo(t *testing.T) {
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
		info, err := FindPageInfo(strings.NewReader(page))
		if err != nil {
			t.Errorf("failed to find info with %s", err)
		}
		if slices.EqualFunc(info.Props, expected, func(lhs, rhs Property) bool {
			return lhs.Name == rhs.Name && lhs.Level == rhs.Level && slices.Equal(lhs.Values, rhs.Values)
		}) {
			continue
		}

		t.Errorf("got different info, expected %#v, got %#v", expected, info)
	}
}

func TestPageInfo_Tags(t *testing.T) {
	page2expected := map[string][]string{
		page1: []string{"species", "research", "psycho"},
	}

	for page, expected := range page2expected {
		info, err := FindPageInfo(strings.NewReader(page))
		if err != nil {
			t.Errorf("failed to find info with %s", err)
		}

		tags, _ := info.AllTags()
		if slices.Equal(tags, expected) {
			continue
		}

		t.Errorf("expected %#v, got %#v", expected, tags)
	}
}
