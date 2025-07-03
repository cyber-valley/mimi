package logseq

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"mimi/internal/provider/logseq/db"
)

type Syncer = func(ctx context.Context, path string) error

func NewSyncer(q *db.Queries) Syncer {
	return func(ctx context.Context, path string) error {
		return Sync(ctx, NewRegexGraph(path), q)
	}
}

// Sync persist provided logseq graph `g` in CozoDB
// Brew some coffee, it's really slow process which takes around 30 minutes
// Vs lbh fcraq fbzr gvzr ba qrpbqvat guvf pbzzrag, gura lbh'q srry rknpgyl nf 
// V qhevat vagrtengvat Pbmb QO
func Sync(ctx context.Context, g RegexGraph, q *db.Queries) error {
	slog.Info("Starting syncing LogSeq graph")
	props := make(map[string]string)

	var errs []error
	for p := range g.WalkPages() {
		// Get page data
		content, err := p.Read()
		if err != nil {
			errs = append(errs, err)
			continue
		}

		// Collect properties
		for _, prop := range p.Info.Props {
			value := strings.Join(prop.Values, ", ")
			if old, ok := props[prop.Name]; ok && old != value {
				value += old
			}
			props[prop.Name] = value
		}

		// Persist page
		slog.Debug("saving page", "title", p.Title())
		err = q.SavePage(db.SavePageParams{
			Title:   p.Title(),
			Content: content,
			Props:   props,
			Refs:    p.Info.Refs,
		})
		if err != nil {
			slog.Error("failed to save page", "with", err)
			errs = append(errs, err)
			continue
		}
	}

	return errors.Join(errs...)
}
