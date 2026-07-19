package backup

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/the-loon-clan/loon/core"
	"github.com/the-loon-clan/loon/schedule"
)

func init() {
	core.RegisterPlugin("backup", func() core.Plugin { return &Plugin{} })
}

const backupIntervalMin = 7 * 24 * 60 // weekly

// Plugin owns the single backup job. The mutex keeps a manual /admin/jobs
// trigger from racing the scheduled loop — a second concurrent run would
// pg_dump and zip the same assets into a different dated folder, doubling the
// IO and the disk for no benefit.
type Plugin struct {
	job *schedule.JobInfo
	mu  sync.Mutex
}

func (p *Plugin) Metadata() core.Metadata {
	return core.Metadata{
		Name:        "backup",
		Version:     "0.1.0",
		Description: "Weekly backup: zips persistent static-asset directories and dumps the PostgreSQL database, with retention pruning.",
		// Worker-only: no routes, just the scheduled loop.
		Processes: []string{"worker"},
	}
}

func (p *Plugin) Provision(c *core.Core) error {
	if deps == nil || deps.Config == nil {
		return fmt.Errorf("backup: SetDeps not called (Config) before core.Boot — wire it in the worker block")
	}
	if deps.BackupDir == "" {
		return fmt.Errorf("backup: SetDeps missing BackupDir")
	}
	if deps.DB.DBName == "" {
		return fmt.Errorf("backup: SetDeps missing DB connection (DBName empty) — pg_dump has nothing to target")
	}

	p.job = schedule.RegisterJob("Backup",
		"Weekly backup: compresses cover art and dumps the PostgreSQL database")
	p.job.IntervalMin = backupIntervalMin
	// The trigger forces: an operator pressing Run in /admin/jobs means "now",
	// not "now unless a recent backup exists".
	p.job.SetTrigger(func() { go p.runForced(context.Background()) })
	return nil
}

func (p *Plugin) Start(ctx context.Context) error {
	// Bare ServiceLoop: the host installs the off-peak / interval-override /
	// CPU / panic hooks globally.
	//
	// The boot delay replaces the old service's `for { sleep(week); run() }`,
	// which never ran at boot and so slept a full week before its first
	// backup. An hour is late enough not to compete with boot, early enough
	// that a fresh deploy has a backup the same day.
	go schedule.ServiceLoop(ctx, p.job, 1*time.Hour, backupIntervalMin*time.Minute, p.run)
	return nil
}

func (p *Plugin) Stop(ctx context.Context) error { return nil }
