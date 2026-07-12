// Package usenet is the delivery-axis plugin: a basic Usenet indexer that crawls
// the last few days of a set of newsgroups, assembles complete article sets into
// downloadable NZB files, and serves search / group-list / download through a
// capability the host's pages consume. It owns the "usenet" Postgres schema and
// groups all the indexer's jobs — Crawler, NZB Builder, Prune — in one place.
//
// It is the LEAN subset of the prod crawler: forward-only, ~N-day window, single
// connection, Postgres staging (no Redis), no backfill/coverage machinery. See
// USENET-PLUGIN.md.
package usenet

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"sync"
	"time"

	"github.com/ameNZB/loon/core"

	"github.com/ameNZB/loon-plugins/pluginapi"
)

//go:embed migrations/*.sql
var migrations embed.FS

func init() {
	core.RegisterPlugin("usenet", func() core.Plugin { return &Plugin{} })
}

type Plugin struct {
	core *core.Core
	cfg  Config
	ctx  context.Context
	st   *store
	svc  *service
	tmpl *template.Template // admin-view fragments (views.go)

	crawlJob    core.Job
	backfillJob core.Job
	buildJob    core.Job
	tagJob      core.Job
	pruneJob    core.Job

	// per-job locks: a manual trigger (admin button / /admin/jobs) must not
	// overlap a scheduled run of the same job — they share one NNTP connection
	// and race on watermarks.
	crawlMu    sync.Mutex
	backfillMu sync.Mutex
	buildMu    sync.Mutex
}

func (p *Plugin) Metadata() core.Metadata {
	return core.Metadata{
		Name:        "usenet",
		Version:     "0.1.0",
		Description: "A basic Usenet indexer: crawls recent posts, builds NZB files, and serves search / groups / download.",
		Processes:   []string{"web", "worker"},
		Migrations:  migrations,
	}
}

func (p *Plugin) Provision(c *core.Core) error {
	p.core = c
	p.st = &store{db: c.Storage.SchemaDB("usenet")}
	if err := c.Config.PluginInto("usenet", &p.cfg); err != nil {
		return fmt.Errorf("usenet: config: %w", err)
	}
	p.cfg.applyDefaults()
	p.svc = &service{store: p.st}

	// Contribute indexer totals to the stats snapshot (collected in the worker
	// process; registering everywhere is harmless).
	if err := pluginapi.RegisterStats(c, statHook{store: p.st}); err != nil {
		return err
	}

	// web/all: publish the read + admin capabilities the host pages consume,
	// and register the plugin-owned admin views (setup wizard + crawl status)
	// the host wraps in its own chrome.
	if c.Process == "web" || c.Process == "all" {
		if err := c.Register(pluginapi.UsenetIndexName, p.svc); err != nil {
			return err
		}
		if err := c.Register(pluginapi.UsenetAdminName, p.svc); err != nil {
			return err
		}
		if err := p.registerViews(c); err != nil {
			return err
		}
	}

	// worker/all: register the three grouped jobs. The NZB Builder is a distinct
	// job because "creating the files" is the point of an indexer.
	if c.Process == "worker" || c.Process == "all" {
		p.crawlJob = c.Scheduler.RegisterJob("Usenet Crawler",
			"Fetches recent article overviews from active newsgroups").MarkOffPeak()
		p.backfillJob = c.Scheduler.RegisterJob("Usenet Backfill",
			"Walks each group's history backward to fill the retention window").MarkOffPeak()
		p.buildJob = c.Scheduler.RegisterJob("NZB Builder",
			"Assembles complete article sets into downloadable NZB files").MarkOffPeak()
		p.tagJob = c.Scheduler.RegisterJob("NZB Tag Fill",
			"Re-parses resolution/source/codec/audio/language tags for untagged NZBs")
		p.pruneJob = c.Scheduler.RegisterJob("NZB Prune",
			"Deletes NZBs older than the retention window")
		p.crawlJob.SetTrigger(func() { go p.runCrawl(p.ctx) })
		p.backfillJob.SetTrigger(func() { go p.runBackfill(p.ctx) })
		p.buildJob.SetTrigger(func() { go p.runBuild(p.ctx) })
		p.tagJob.SetTrigger(func() { go p.runTagFill(p.ctx) })
		p.pruneJob.SetTrigger(func() { go p.runPrune(p.ctx) })
		p.svc.triggerCrawl = func() { go p.runCrawl(p.ctx) }
		p.svc.triggerBackfill = func() { go p.runBackfill(p.ctx) }
	}
	return nil
}

func (p *Plugin) Start(ctx context.Context) error {
	p.ctx = ctx
	if p.crawlJob == nil {
		return nil // web-only process: capability is registered, no jobs run here
	}
	p.seedServer(ctx)
	interval := time.Duration(p.cfg.CrawlIntervalMin) * time.Minute
	backfillInterval := time.Duration(p.cfg.BackfillIntervalMin) * time.Minute
	p.core.Scheduler.RunLoop(ctx, p.crawlJob, time.Minute, interval, p.runCrawl)
	// Boot backfill after the forward crawl has had a pass to seed watermarks.
	p.core.Scheduler.RunLoop(ctx, p.backfillJob, 3*time.Minute, backfillInterval, p.runBackfill)
	p.core.Scheduler.RunLoop(ctx, p.buildJob, 90*time.Second, interval, p.runBuild)
	p.core.Scheduler.RunLoop(ctx, p.tagJob, 5*time.Minute, 6*time.Hour, p.runTagFill)
	p.core.Scheduler.RunLoop(ctx, p.pruneJob, 10*time.Minute, 24*time.Hour, p.runPrune)
	return nil
}

// runTagFill retrofits quality tags onto NZBs missing them (build-time tagging
// covers new rows; this catches rows from before a parser change).
func (p *Plugin) runTagFill(ctx context.Context) {
	if ctx == nil {
		return
	}
	p.tagJob.SetRunning()
	n, err := p.st.retagUntagged(ctx, 500)
	if err != nil {
		p.tagJob.SetError(err.Error())
		p.core.Errors.Report(ctx, "usenet/tag-fill", err)
		return
	}
	p.tagJob.Log("re-tagged %d NZB(s)", n)
	p.tagJob.SetIdle(time.Now().Add(6 * time.Hour))
}

func (p *Plugin) Stop(ctx context.Context) error { return nil }

// seedServer inserts the config server if the table is empty (best-effort). Runs
// in Start (not Provision — no I/O there) so the crawler has a server to use.
func (p *Plugin) seedServer(ctx context.Context) {
	if p.cfg.Server.Host == "" {
		return
	}
	if _, ok, _ := p.st.getServer(ctx); ok {
		return
	}
	err := p.st.saveServer(ctx, pluginapi.Server{
		Host: p.cfg.Server.Host, Port: p.cfg.Server.Port, TLS: p.cfg.Server.TLS,
		Username: p.cfg.Server.Username, Password: p.cfg.Server.Password, Enabled: true,
	})
	if err != nil {
		p.core.Errors.Report(ctx, "usenet/seed-server", err)
	}
}

// effective returns the config with any admin-edited settings overlaid (the
// /admin/settings knobs). Jobs call this at run start so edits apply on the
// next run without a restart. Falls back to the boot config on read error.
func (p *Plugin) effective(ctx context.Context) Config {
	s, err := p.st.getSettings(ctx)
	if err != nil {
		p.core.Errors.Report(ctx, "usenet/settings", err)
		return p.cfg
	}
	return p.cfg.withOverrides(s)
}

// runPrune deletes NZBs past the retention window + stale staged articles.
func (p *Plugin) runPrune(ctx context.Context) {
	if ctx == nil {
		return
	}
	p.pruneJob.SetRunning()
	cfg := p.effective(ctx)
	n, err := p.st.pruneNzbs(ctx, cfg.RetentionDays)
	if err != nil {
		p.pruneJob.SetError(err.Error())
		p.core.Errors.Report(ctx, "usenet/prune", err)
		return
	}
	staged, _ := p.st.pruneStaging(ctx)
	// Sweep junk left over from before ingest filtering (obfuscated random-token
	// titles that assembled into garbage releases / clog staging).
	junkNzbs, err := p.st.deleteJunkNzbs(ctx)
	if err != nil {
		p.core.Errors.Report(ctx, "usenet/prune-junk-nzbs", err)
	}
	junkStaged, err := p.st.deleteJunkStaged(ctx)
	if err != nil {
		p.core.Errors.Report(ctx, "usenet/prune-junk-staged", err)
	}
	p.pruneJob.Log("pruned %d NZBs (older than %dd) + %d stale staged; swept %d junk NZBs + %d junk staged",
		n, cfg.RetentionDays, staged, junkNzbs, junkStaged)
	p.pruneJob.SetIdle(time.Now().Add(24 * time.Hour))
}

// runCrawl (crawl.go) and runBuild (assemble.go) implement the crawl + assembly.
