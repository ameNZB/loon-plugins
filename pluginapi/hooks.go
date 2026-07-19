package pluginapi

import (
	"context"
	"io"
	"strings"

	"github.com/the-loon-clan/loon/core"
)

// This file is the callback/hook mechanism for cross-cutting concerns (backups,
// stats). It needs NO core change: loon's extension registry
// (core.Register/Lookup/ExtensionNames) already IS the hook bus. These helpers
// add a typed convention on top so call sites aren't stringly-typed:
//
//   - A plugin PARTICIPATES by implementing Backupable / StatContributor and
//     calling RegisterBackup(c, self) / RegisterStats(c, self) in Provision.
//   - The backups / stats plugin COLLECTS every participant with Backups(c) /
//     StatContributors(c) and invokes the hook on its job.
//
// Neither side imports the other — they meet at these interfaces, exactly like
// the ranks -> store capability.

// ── Backup hook ────────────────────────────────────────────────────

// Backupable is the hook a plugin implements to contribute to a site backup.
type Backupable interface {
	// BackupName is a stable, unique short label (the plugin name) used as the
	// archive entry name. No slashes.
	BackupName() string
	// Backup streams this plugin's payload to w — a SQL dump of its schema, a
	// tarball of its assets, whatever it owns. Called off-peak on the backup job.
	Backup(ctx context.Context, w io.Writer) error
}

const backupPrefix = "backup:"

// RegisterBackup publishes b so the backups plugin includes it. Call in Provision.
func RegisterBackup(c *core.Core, b Backupable) error {
	return c.Register(backupPrefix+b.BackupName(), b)
}

// Backups collects every registered Backupable (name order — ExtensionNames is
// sorted). Used by the backups plugin's job.
func Backups(c *core.Core) []Backupable {
	var out []Backupable
	for _, name := range c.ExtensionNames() {
		if !strings.HasPrefix(name, backupPrefix) {
			continue
		}
		if v, ok := c.Lookup(name); ok {
			if b, ok := v.(Backupable); ok {
				out = append(out, b)
			}
		}
	}
	return out
}

// ── Stats hook ─────────────────────────────────────────────────────

// Stat is one metric a plugin contributes to the stats page.
type Stat struct {
	Key   string // stable id, namespaced by plugin ("store.purchases")
	Label string // display label ("Store purchases")
	Value int64
}

// StatContributor is the hook a plugin implements to add rows to the stats page.
type StatContributor interface {
	// StatsName is a stable unique label for the contributor (the plugin name).
	StatsName() string
	// Stats returns current metric values. Called on the stats cache job.
	Stats(ctx context.Context) ([]Stat, error)
}

const statsPrefix = "stats:"

// RegisterStats publishes s so the stats plugin collects it. Call in Provision.
func RegisterStats(c *core.Core, s StatContributor) error {
	return c.Register(statsPrefix+s.StatsName(), s)
}

// StatContributors collects every registered StatContributor (name order).
func StatContributors(c *core.Core) []StatContributor {
	var out []StatContributor
	for _, name := range c.ExtensionNames() {
		if !strings.HasPrefix(name, statsPrefix) {
			continue
		}
		if v, ok := c.Lookup(name); ok {
			if s, ok := v.(StatContributor); ok {
				out = append(out, s)
			}
		}
	}
	return out
}

// ── Event bus hook ─────────────────────────────────────────────────

// Emitter is the publish side of a host event bus (loon-baseline/events.Bus
// satisfies it structurally). A plugin publishes domain events through it; the
// host — or another plugin — subscribes on its own bus. Neither imports the
// other; they meet here.
type Emitter interface {
	Emit(ctx context.Context, topic string, payload any)
}

// Well-known event topics plugins publish.
const (
	// EventIngested fires after a batch of new releases lands (payload: the
	// number inserted, an int). Subscribers typically invalidate search caches.
	EventIngested = "usenet.ingested"
)

const eventsCapability = "events"

// RegisterEvents publishes the host's event bus so plugins can Emit through it.
// Call once at wiring time. A host with no bus simply doesn't call this, and
// EmitEvent becomes a no-op everywhere.
func RegisterEvents(c *core.Core, e Emitter) error {
	return c.Register(eventsCapability, e)
}

// EmitEvent publishes a best-effort event through the host bus if one is
// registered — no bus, no subscriber, no-op. Publishing never requires a
// subscriber to exist.
func EmitEvent(c *core.Core, ctx context.Context, topic string, payload any) {
	if v, ok := c.Lookup(eventsCapability); ok {
		if e, ok := v.(Emitter); ok {
			e.Emit(ctx, topic, payload)
		}
	}
}
