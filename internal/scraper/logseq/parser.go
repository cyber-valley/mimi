// Parse graph
// Upload it into cozodb
// Do embedding stuff
// Provide search interface with tags & text
package logseq

import (
	"context"
	"github.com/aholstenson/logseq-go"
	"github.com/golang/glog"
)

func SyncGraph(ctx context.Context, path string) error {
	g, err := logseq.Open(ctx, path, logseq.WithInMemoryIndex())
	if err != nil {
		glog.Errorf("failed to open graph %s with: %s", path, err)
		return err
	}
	glog.Infof("graph: %#v", g)
	pages, err := g.SearchPages(ctx)
	if err != nil {
		glog.Errorf("failed to search pages with: %s", err)
		return err
	}
	for _, p := range pages.Results() {
		glog.Infof("page: %s", p.Title())
		p, err := p.Open()
		if err != nil {
			glog.Warning("failed to open page with: %s", err)
		}
		glog.Infof("subpath: %T %#v", p, p)
	}
	return nil
}
