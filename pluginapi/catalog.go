package pluginapi

import "context"

// Catalog is the content-taxonomy capability: the standard Newznab category
// tree (Console/Movies/Audio/PC/TV/XXX/Books/Other), which top-level categories
// an admin has enabled ("list everything, pick what to index"), and a heuristic
// to categorize a release by its group + title. Published by the catalog
// plugin; consumed by indexer plugins (usenet) for Newznab caps + filtering.

// Category is a Newznab-standard content category. Top-level ids are thousands
// (1000 Console … 8000 Other); Subcats sit under them (e.g. 5070 TV/Anime).
type Category struct {
	ID      int
	Name    string
	Subcats []Subcategory
}

// Subcategory is a child category (e.g. {5070, "Anime"} under TV).
type Subcategory struct {
	ID   int
	Name string
}

// Catalog is looked up off the extension registry under CatalogName.
type Catalog interface {
	// All returns the full taxonomy (every top-level category + subcats).
	All(ctx context.Context) ([]Category, error)
	// Enabled returns only the admin-enabled top-level categories (+ subcats).
	Enabled(ctx context.Context) ([]Category, error)
	// IsEnabled reports whether a top-level category (or a subcat's parent) is on.
	IsEnabled(ctx context.Context, categoryID int) (bool, error)
	// Categorize maps a release to its best-fit Newznab category id from its
	// group + title. Pure heuristic; does not consult the enabled set.
	Categorize(group, title string) int
	// Name returns a "Parent/Sub" display name for a category id.
	Name(id int) string
}

const CatalogName = "catalog.taxonomy"
