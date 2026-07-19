package stats

import (
	"context"

	"github.com/the-loon-clan/loon-plugins/pluginapi"
)

// Deps is the one host seam: where the collected snapshot is cached. The stats
// PAGE (a web route reading this cache) lives in the host or a web-process
// surface; this plugin is the worker-side collector only.
type Deps struct {
	// Cache persists the collected snapshot. The host reads it on the stats page.
	Cache func(ctx context.Context, stats []pluginapi.Stat) error
}

var deps *Deps

// SetDeps stages the host seam. Call once, in the worker process, before core.Boot.
func SetDeps(d Deps) { deps = &d }
