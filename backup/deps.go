// Package backup is the weekly-backup plugin: one worker job that zips every
// persistent static-asset directory and pg_dumps the database into a dated
// folder, then prunes old runs to a retention count.
//
// Extracted from the host's pkg/services. The host keeps what only it can know
// — which directories are persistent, where backups land, how to reach
// Postgres — and hands them over as Deps; the scheduling comes from
// loon/schedule.
//
// Notably this plugin needs NO per-plugin cooperation: the database half is a
// pg_dump of the whole cluster, so it captures every plugin's tables without
// knowing any of them exist. Only the asset-directory list is host knowledge,
// and that is a slice of strings.
package backup

import (
	"context"
)

// PGConn is the Postgres connection target the pg_dump CLI is invoked against —
// the same values the host connects with. A struct of five fields rather than
// the host's config type: this module has no business importing the site's
// configuration package to read a hostname.
type PGConn struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
}

// ConfigStore is the admin-tunable behaviour, read fresh on every run so a
// change in the admin UI takes effect on the next tick without a restart.
//
// These stay host settings rather than becoming loon/schedule job-config vars:
// they already exist in the host's admin surface, and moving them would be a
// migration of live operator settings, not an extraction. The host's
// SettingsService satisfies this as-is.
type ConfigStore interface {
	// GetBackupMode returns "db_only" to skip the asset zips, anything else
	// for a full run.
	GetBackupMode(ctx context.Context) string
	// GetBackupKeepCount returns how many dated runs to retain; <= 0 disables
	// pruning entirely.
	GetBackupKeepCount(ctx context.Context) int
}

// Deps are the host-provided seams, staged before core.Boot in the worker.
type Deps struct {
	// DB is the pg_dump connection target.
	DB PGConn
	// Config backs the admin-editable mode + retention.
	Config ConfigStore
	// StaticDirs is the list of directories to zip, one archive each, named
	// after the basename (covers -> covers.zip). The host passes its canonical
	// persistent-dirs list; this plugin has no way to know what is persistent.
	StaticDirs []string
	// BackupDir is where dated run folders are written.
	//
	// It MUST be a bind mount on the host side. Left inside a container's
	// overlay filesystem it is wiped on every recreate, which turns the whole
	// job into theatre: it runs, it logs success, and it protects nothing.
	BackupDir string
}

var deps *Deps

// SetDeps stages the host seams. Call once, in the worker process, before
// core.Boot.
func SetDeps(d Deps) { deps = &d }
