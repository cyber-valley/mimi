package logseqext

import (
	"iter"

	"github.com/aholstenson/logseq-go"
	"github.com/aholstenson/logseq-go/content"
)

func WalkPage[T content.Node](p logseq.Page) iter.Seq[T] {
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
