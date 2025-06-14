// Parse graph
// Upload it into cozodb
// Do embedding stuff
// Provide search interface with tags & text
package logseq

import (
	"context"
	"fmt"
	"github.com/aholstenson/logseq-go"
	"github.com/aholstenson/logseq-go/content"
	"github.com/golang/glog"
	"iter"
	"math"
)

func SyncGraph(ctx context.Context, path string) error {
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
	for _, p := range pages.Results() {
		glog.Infof("page: %s", p.Title())
		p, err := p.Open()
		if err != nil {
			glog.Warning("failed to open page with: %s", err)
		}
		glog.Infof("subpath: %#v", p)
		for prop := range walkProps(p) {
			for _, child := range prop.Children() {
				if text, ok := child.(*content.Text); ok {
					glog.Infof("prop %s:%#v", prop.Name, text.Value)
				}
			}
		}

		for ref := range walkRefs(p) {
			glog.Infof("ref %#v", ref)
		}
	}
	return nil
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
