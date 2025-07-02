package logseq

import (
	"slices"
	"strings"
	"testing"
)

const (
	page1 = `alias:: damiana
tags:: species, research, psycho

- supply:: next-month
[[@master]]
[[foo]] and [[bar]]`
)

func TestFindPageInfo(t *testing.T) {
	page2expected := map[string]PageInfo{
		page1: PageInfo{
			Props: []Property{
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
			Refs: []string{"@master", "foo", "bar"},
		},
	}

	for page, expected := range page2expected {
		info, err := FindPageInfo(strings.NewReader(page))
		if err != nil {
			t.Errorf("failed to find info with %s", err)
		}

		// Test properties
		if !slices.EqualFunc(info.Props, expected.Props, func(lhs, rhs Property) bool {
			return lhs.Name == rhs.Name && lhs.Level == rhs.Level && slices.Equal(lhs.Values, rhs.Values)
		}) {
			t.Errorf("expected %#v, got %#v", expected.Props, info.Props)
		}

		// Test references
		if !slices.Equal(info.Refs, expected.Refs) {
			t.Errorf("expected %#v, got %#v", expected.Refs, info.Refs)
		}
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
