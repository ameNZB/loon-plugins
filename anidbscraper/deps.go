package anidbscraper

import "github.com/the-loon-clan/loon-plugins/pluginapi"

// Deps are the host-owned collaborators the scraper needs injected before boot.
// The host builds thin adapters over its existing repositories and calls
// SetDeps in the worker block of cmd/main.go, BEFORE core.Boot — exactly the
// pattern plugins/offers already uses (offers.SetJobDeps).
//
// Everything here is either a CLEAN dependency the plugin would ideally own
// (Catalog) or an ENTANGLED one that must stay host-side (Nzbs, Covers, Matcher
// — see pluginapi/anidb.go and JOBS-AS-PLUGINS.md for why).
type Deps struct {
	// Catalog is the anime_metadata / anime_aliases store (scraper-owned data).
	Catalog pluginapi.AnimeCatalog
	// Nzbs is the write-back port into the host `nzbs` table (the deep seam).
	Nzbs pluginapi.NzbTagSink
	// Matcher is the shared title matcher, rebuilt after each titles refresh.
	Matcher pluginapi.TitleMatcher
	// Covers maps to web/static/covers/{aid}.jpg.
	Covers pluginapi.CoverStore
}

// deps is package-scoped because RegisterPlugin captures a zero-value factory at
// init() time; the host fills the collaborators in later, before Boot. Mirrors
// offers.deps / offers.jobDeps.
var deps *Deps

// SetDeps stages the host collaborators. Call exactly once, in the worker
// process, before core.Boot. A nil field is caught in Provision (fail-fast)
// rather than nil-panicking mid-scan.
func SetDeps(d Deps) { deps = &d }
