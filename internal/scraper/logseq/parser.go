// Parse graph
// Upload it into cozodb
// Do embedding stuff
// Provide search interface with tags & text
package logseq

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"math"
	"strings"

	"github.com/aholstenson/logseq-go"
	"github.com/aholstenson/logseq-go/content"
	"github.com/golang/glog"

	"mimi/internal/scraper/logseq/db"
)

func SyncGraph(ctx context.Context, q *db.Queries, path string) error {
	g, err := logseq.Open(ctx, path, logseq.WithInMemoryIndex())
	if err != nil {
		glog.Errorf("failed to open graph %s with: %s", path, err)
		return err
	}

	glog.Infof("graph: %#v", g)
	pages, err := g.SearchPages(ctx, logseq.WithQuery(logseq.All()), logseq.WithMaxHits(math.MaxInt))
	if err != nil {
		glog.Errorf("failed to search pages with: %s", err)
		return err
	}

	if pages.Size() != pages.Count() {
		return fmt.Errorf("pages amount differs from total count: %d != %d", pages.Size(), pages.Count())
	}

	glog.Infof("found pages size: %d, count: %d", pages.Size(), pages.Count())
	var errs []error
	for _, p := range pages.Results() {
		props := make(map[string]string)
		var refs []string

		// Get page data
		p, err := p.Open()
		if err != nil {
			glog.Warningf("failed to open page with: %s", err)
			continue
		}

		// Collect properties
		for prop := range walkProps(p) {
			for _, child := range prop.Children() {
				if text, ok := child.(*content.Text); ok {
					if text.Value == "" {
						continue
					}
					if old, ok := props[prop.Name]; ok {
						glog.Warningf("got duplicate property name %s, old '%s', new '%s'", prop.Name, old, text.Value)
					}
					props[prop.Name] = text.Value
				}
			}
		}

		// Collect references
		for ref := range walkRefs(p) {
			refs = append(refs, ref.GetTo())
		}

		glog.Infof("Parsed %d properties and %d refs", len(props), len(refs))
		err = q.SavePage(db.SavePageParams{
			Title:   p.Title(),
			Content: extractText(p),
			Props:   props,
			Refs:    refs,
		})
		if err != nil {
			glog.Errorf("failed to save page with %s", err)
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func walkProps(p logseq.Page) iter.Seq[*content.Property] {
	blocks := p.Blocks()
	return func(yield func(*content.Property) bool) {
		for _, b := range blocks {
			for _, prop := range b.Children().FilterDeep(func(node content.Node) bool {
				_, ok := node.(*content.Property)
				return ok
			}) {
				if !yield(prop.(*content.Property)) {
					return
				}
			}
		}
	}
}

func walkRefs(p logseq.Page) iter.Seq[content.PageRef] {
	blocks := p.Blocks()
	return func(yield func(content.PageRef) bool) {
		for _, b := range blocks {
			for _, ref := range b.Children().FilterDeep(func(node content.Node) bool {
				_, ok := node.(content.PageRef)
				return ok
			}) {
				if !yield(ref.(content.PageRef)) {
					return
				}
			}
		}
	}
}

func extractText(p logseq.Page) string {
	var bob strings.Builder
	for _, block := range p.Blocks() {
		for _, node := range block.Content().FilterDeep(content.IsOfType[*content.Text]()) {
			bob.WriteString(node.(*content.Text).Value)
		}
	}
	return strings.TrimSpace(bob.String())
}
