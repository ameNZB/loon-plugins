package pluginapi

import (
	"context"
	"time"
)

// Usenet capability contracts. The usenet plugin publishes these on the core
// extension registry (UsenetIndexName / UsenetAdminName); the host's pages look
// them up so the site queries the indexer without importing the plugin.

// Release is one assembled NZB in search/listing results.
type Release struct {
	ID         int64
	Title      string
	Size       int64
	Posted     time.Time
	Group      string
	Resolution string
	Source     string
	Codec      string
	Audio      string
	Language   string
}

// GroupInfo is one watched newsgroup + how many NZBs it has produced.
type GroupInfo struct {
	Name   string
	Active bool
	NZBs   int64
}

// GroupStat is the crawl status of one active group.
type GroupStat struct {
	Name              string
	NZBs              int
	Staged            int // articles waiting in staging (not yet assembled)
	LastCrawl         time.Time
	HighWatermark     int64     // highest article number crawled (forward position)
	HighWatermarkDate time.Time // posting date at the forward position
	BackWatermark     int64     // backfill position; walks down toward ServerLow
	BackWatermarkDate time.Time // posting date reached by backfill
	ServerLow         int64     // server's oldest retained article number
	ServerHigh        int64     // server's highest article number
	BackfillDone      bool      // backfill reached ServerLow or the retention horizon
}

// CoverageBar is the 3-segment coverage of a group over [ServerLow..ServerHigh],
// as percentages that sum to ~100 when Known. Back = still-to-backfill (below
// back_watermark), Have = indexed span, New = not-yet-fetched-forward (above
// high_watermark).
type CoverageBar struct {
	BackPct float64
	HavePct float64
	NewPct  float64
	Known   bool // false when the server span is unknown (never crawled)
}

// Coverage derives the coverage bar from the watermarks. Pure; the host page
// renders BackPct/HavePct/NewPct as a stacked bar.
func (g GroupStat) Coverage() CoverageBar {
	span := float64(g.ServerHigh - g.ServerLow)
	if span <= 0 {
		return CoverageBar{Known: false}
	}
	back := g.BackWatermark
	if back < g.ServerLow {
		back = g.ServerLow
	}
	if back > g.HighWatermark {
		back = g.HighWatermark
	}
	pct := func(n int64) float64 {
		if n < 0 {
			n = 0
		}
		return float64(n) / span * 100
	}
	return CoverageBar{
		BackPct: pct(back - g.ServerLow),
		HavePct: pct(g.HighWatermark - back),
		NewPct:  pct(g.ServerHigh - g.HighWatermark),
		Known:   true,
	}
}

// IndexStats summarizes what the indexer has pulled so far.
type IndexStats struct {
	TotalNZBs              int
	TotalStaged            int
	TotalBackfillRemaining int64       // sum of (back_watermark - server_low) over active groups still backfilling
	Groups                 []GroupStat // active groups
}

// Server is the NNTP server configuration the setup wizard edits.
type Server struct {
	Host     string
	Port     int
	TLS      bool
	Username string
	Password string
	Enabled  bool
}

// UsenetIndex is the public read surface — registered in the web/all process.
type UsenetIndex interface {
	Search(ctx context.Context, query string, limit int) ([]Release, error)
	// Browse lists recent releases, optionally filtered to one group (empty = all).
	Browse(ctx context.Context, group string, limit int) ([]Release, error)
	Groups(ctx context.Context) ([]GroupInfo, error)
	// NZB returns the decompressed .nzb bytes + a suggested download filename.
	NZB(ctx context.Context, id int64) (data []byte, filename string, err error)
}

// UsenetAdmin is the setup-wizard surface — registered in the web/all process.
type UsenetAdmin interface {
	Server(ctx context.Context) (Server, error)
	SetServer(ctx context.Context, s Server) error
	TestConnect(ctx context.Context, s Server) error        // Dial + Auth + Quit
	FetchGroups(ctx context.Context) (added int, err error) // NNTP LIST -> insert inactive
	// AllGroups returns groups for the picker, active first. query filters by
	// name substring (case-insensitive); empty query returns the first `limit`.
	AllGroups(ctx context.Context, query string, limit int) ([]GroupInfo, error)
	GroupCount(ctx context.Context) (int, error)   // total groups fetched (for "showing N of M")
	Stats(ctx context.Context) (IndexStats, error) // crawl progress: totals + per-active-group status
	SetGroupActive(ctx context.Context, name string, active bool) error
	TriggerCrawl()    // fire the crawler job now
	TriggerBackfill() // fire the backfill job now
	// ResetBackfill re-arms a group's backfill from its high watermark downward.
	ResetBackfill(ctx context.Context, name string) error
}

const (
	UsenetIndexName = "usenet.index"
	UsenetAdminName = "usenet.admin"
)
