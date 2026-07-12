package scraper

import "github.com/ameNZB/loon-plugins/pluginapi"

// Deps are the host-owned collaborators the scraper needs injected before boot.
// The catalog.Registry itself is NOT here — the scraper looks it up off the Core
// extension registry (the host already publishes it under
// catalog.RegistryExtension). The only injected seam is the write side.
type Deps struct {
	// Sink persists scraped entries into the host's unified catalog_entry table.
	Sink pluginapi.CatalogSink
}

var deps *Deps

// SetDeps stages the host collaborators. Call once, in the worker process,
// before core.Boot.
func SetDeps(d Deps) { deps = &d }
