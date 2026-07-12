// Package stats is the site-stats collector: on a cache job it gathers every
// plugin's StatContributor hook into one snapshot and caches it for the stats
// page. Plugins opt in by implementing pluginapi.StatContributor and calling
// pluginapi.RegisterStats(c, self) in Provision — discovered off the core
// extension registry, so adding a stat-contributing plugin needs no change here.
package stats

import (
	"context"
	"fmt"
	"time"

	"github.com/ameNZB/loon/core"

	"github.com/ameNZB/loon-plugins/pluginapi"
)

func init() {
	core.RegisterPlugin("stats", func() core.Plugin { return &Plugin{} })
}

type Plugin struct {
	core *core.Core
	ctx  context.Context
	job  core.Job
}

func (p *Plugin) Metadata() core.Metadata {
	return core.Metadata{
		Name:        "stats",
		Version:     "0.1.0",
		Description: "Collects every plugin's StatContributor hook into a cached site-stats snapshot.",
		Processes:   []string{"worker"},
	}
}

func (p *Plugin) Provision(c *core.Core) error {
	p.core = c
	if deps == nil || deps.Cache == nil {
		return fmt.Errorf("stats: SetDeps not called with a Cache sink before core.Boot")
	}
	p.job = c.Scheduler.RegisterJob("Stats Cache", "Collects plugin StatContributor hooks into the cached stats snapshot")
	p.job.SetTrigger(func() { go p.run(p.ctx) })
	return nil
}

func (p *Plugin) Start(ctx context.Context) error {
	p.ctx = ctx
	p.core.Scheduler.RunLoop(ctx, p.job, time.Minute, time.Hour, p.run)
	return nil
}

func (p *Plugin) Stop(ctx context.Context) error { return nil }

func (p *Plugin) run(ctx context.Context) {
	if ctx == nil {
		return
	}
	p.job.SetRunning()

	var all []pluginapi.Stat
	contributors := pluginapi.StatContributors(p.core)
	for _, sc := range contributors {
		if ctx.Err() != nil {
			return
		}
		stats, err := sc.Stats(ctx)
		if err != nil {
			p.core.Errors.Report(ctx, "stats/collect", fmt.Errorf("%s: %w", sc.StatsName(), err))
			continue
		}
		all = append(all, stats...)
	}

	if err := deps.Cache(ctx, all); err != nil {
		p.job.SetError(err.Error())
		p.core.Errors.Report(ctx, "stats/cache", err)
		return
	}
	p.job.Log("stats cached: %d metrics from %d contributors", len(all), len(contributors))
	p.job.SetIdle(time.Now().Add(time.Hour))
}
