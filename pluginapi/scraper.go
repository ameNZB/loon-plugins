package pluginapi

import (
	"context"

	"github.com/ameNZB/loon/catalog"
)

// CatalogSink persists a scraped catalog.CatalogEntry into the host's unified
// `catalog_entry` table (kind, id, source, title, external_ids, fields, …). The
// host implements it and injects it via scraper.SetDeps.
//
// The scraper deliberately does NOT own this table: the site's browse, search,
// and card rendering read catalog_entry directly, so it lives in the host
// (public) schema, not a plugin schema. The sink is the one write seam.
type CatalogSink interface {
	Upsert(ctx context.Context, entry catalog.CatalogEntry) error
}

// Fillable is the OPTIONAL capability a catalog.MetadataSource implements to
// tell the generic Metadata-Fill job which of its local ids still need
// enrichment. It is feature-detected by type assertion — a source that doesn't
// implement it is simply skipped by the fill loop (it can still serve Fetch and
// TitleIndex for matching). This keeps a minimal source minimal, the same way
// loon/catalog treats CrossIDResolver / CompletionProvider.
type Fillable interface {
	PendingIDs(ctx context.Context, limit int) ([]int64, error)
}
