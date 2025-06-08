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
	graph, err := logseq.Open(ctx, path)
	if err != nil {
		glog.Errorf("failed to open graph %s with: %s", path, err)
		return err
	}
	glog.Infof("graph: %#v", graph)
	return nil
}
