package scraper

import (
	"context"

	"github.com/ameNZB/loon/catalog"
)

// Searcher is the optional capability for API-search catalog sources that have
// NO local id space (ThePornDB, StashDB, a TMDB-by-query source). Instead of
// the id-centric TitleIndex + Fetch(id) flow, they identify an entry from a
// free-text query — the emp-pipeline metadata.Provider shape. ok=false with a
// nil error means "no match", not a failure.
//
// It lives here (not loon/catalog) so a source stays a plain
// catalog.MetadataSource to the framework and only the scraper feature-detects
// this by type assertion. A Searcher source still satisfies MetadataSource
// degenerately: empty TitleIndex, Fetch returning ErrNoLocalID.
type Searcher interface {
	Search(ctx context.Context, query string) (entry catalog.CatalogEntry, ok bool, err error)
}
