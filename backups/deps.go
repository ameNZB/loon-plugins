package backups

import (
	"context"
	"io"
)

// Deps are the host-provided storage + DB-dump seams. The hook orchestration
// (collecting every plugin's Backupable) is core-registry-driven and needs no
// injection; only the "where do bytes go" and "how do I dump the DB" edges do.
type Deps struct {
	// OpenEntry returns a writer for one named backup entry. The host decides
	// storage: a file in a dated dir, a tar member, an object-store key.
	OpenEntry func(ctx context.Context, name string) (io.WriteCloser, error)
	// DumpDB writes a logical database dump (pg_dump) to w. Optional — nil skips
	// the DB entry and backs up only the plugin hooks.
	DumpDB func(ctx context.Context, w io.Writer) error
}

var deps *Deps

// SetDeps stages the host seams. Call once, in the worker process, before core.Boot.
func SetDeps(d Deps) { deps = &d }
