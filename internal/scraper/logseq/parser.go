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
	"log/slog"
	"math"
	"strings"

	"github.com/aholstenson/logseq-go"
	"github.com/aholstenson/logseq-go/content"
	"github.com/golang/glog"

	"mimi/internal/scraper/logseq/db"
)

type Graph struct {
	g   *logseq.Graph
	q   *db.Queries
	ctx context.Context
}

func New(ctx context.Context, q *db.Queries, path string) (Graph, error) {
	g, err := logseq.Open(ctx, path, logseq.WithInMemoryIndex())
	if err != nil {
		glog.Errorf("failed to open graph %s with: %s", path, err)
		return Graph{}, err
	}
	return Graph{
		g:   g,
		q:   q,
		ctx: ctx,
	}, nil
}

func (g Graph) Sync() error {
	pages, err := g.getAllPages()
	if err != nil {
		return fmt.Errorf("reading all pages failed with %w", err)
	}

	var errs []error
	for _, p := range pages {
		props := make(map[string]string)
		var refs []string

		// Get page data
		p, err := p.Open()
		if err != nil {
			glog.Warningf("failed to open page with: %s", err)
			continue
		}

		// Collect properties
		for prop := range walkPage[*content.Property](p) {
			for _, child := range prop.Children() {
				if text, ok := child.(*content.Text); ok {
					if text.Value == "" {
						continue
					}
					if old, ok := props[prop.Name]; ok && old != text.Value {
						glog.Warningf("got duplicate property name %s, old '%s', new '%s'", prop.Name, old, text.Value)
					}
					props[prop.Name] = text.Value
				}
			}
		}

		// Collect references
		for ref := range walkPage[content.PageRef](p) {
			refs = append(refs, ref.GetTo())
		}

		// Persist page
		glog.Infof("Parsed %d properties and %d refs from '%s'", len(props), len(refs), p.Title())
		err = g.q.SavePage(db.SavePageParams{
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

// Get relative text to the given page title
func (g Graph) Retrieve(title string) (contents []string, err error) {
	rels, err := g.q.FindRelatives("genesis", 2)
	if err != nil {
		return contents, fmt.Errorf("failed to find relatives with %w", err)
	}
	slog.Info("found relatives", "amount", len(rels))
	pages, err := g.getAllPages()
	if err != nil {
		return
	}
	var errs []error
	for _, page := range pages {
		p, err := page.Open()
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to open page with %w", err))
			continue
		}
		contents = append(contents, extractText(p))
	}
	return contents, errors.Join(errs...)
}

func (g Graph) getAllPages() (pages []logseq.PageResult, err error) {
	res, err := g.g.SearchPages(g.ctx, logseq.WithQuery(logseq.All()), logseq.WithMaxHits(math.MaxInt))
	if err != nil {
		return pages, fmt.Errorf("pages search failed with %w", err)
	}

	if res.Size() != res.Count() {
		return pages, fmt.Errorf("res amount differs from total count: %d != %d", res.Size(), res.Count())
	}

	return res.Results(), nil
}

func walkPage[T content.Node](p logseq.Page) iter.Seq[T] {
	blocks := p.Blocks()
	return func(yield func(T) bool) {
		for _, b := range blocks {
			for _, ref := range b.Children().FilterDeep(content.IsOfType[T]()) {
				if !yield(ref.(T)) {
					return
				}
			}
		}
	}
}

func extractText(p logseq.Page) string {
	var bob strings.Builder
	for text := range walkPage[*content.Text](p) {
		bob.WriteString(text.Value)
	}
	return strings.TrimSpace(bob.String())
}
