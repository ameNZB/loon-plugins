package scraper

import (
	"context"

	"github.com/ameNZB/loon-plugins/pluginapi"
)

// Deps are the host-owned collaborators the scraper needs injected before boot.
// The catalog.Registry itself is NOT here — the scraper looks it up off the Core
// extension registry (the host publishes it under catalog.RegistryExtension).
type Deps struct {
	// Sink persists scraped entries into the host's unified catalog_entry table.
	Sink pluginapi.CatalogSink

	// Candidates yields releases to enrich — the host wires it over its indexer
	// (usenet.Browse → {id, title, category}). Nil disables the match job.
	Candidates func(ctx context.Context) ([]Candidate, error)

	// Link records a release↔cover match so the host can show the cover on that
	// release's page. Nil means covers aren't linked (entries still upserted).
	Link func(ctx context.Context, releaseID int64, coverURL string) error
}

// Candidate is one release the match job tries to enrich.
type Candidate struct {
	ID       int64
	Title    string
	Category int // Newznab category id — picks the source domain
}

var deps *Deps

// SetDeps stages the host collaborators. Call once, in the worker process,
// before core.Boot.
func SetDeps(d Deps) { deps = &d }
