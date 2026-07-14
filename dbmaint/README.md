# dbmaint plugin

Database-maintenance worker jobs, extracted from the host's `pkg/services`.
Three scheduled jobs keep Postgres lean:

| Job (`/admin/jobs` name) | Cadence | Online? | What it does |
| --- | --- | --- | --- |
| **Repack NZBs (online)** | weekly | yes | `pg_repack -t <tables> -x` — rebuilds tables + indexes with a copy-and-swap, brief lock only. The active reclaim path. |
| **Reindex (online)** | monthly | yes | `REINDEX INDEX CONCURRENTLY` on the largest used indexes not covered by pg_repack's `-x`. |
| **Vacuum NZBs** | weekly | **no** — maintenance window | `VACUUM FULL nzbs`. Takes an AccessExclusiveLock, so the site shows a maintenance page. **Paused by default** — pg_repack replaced it; kept as a manual-trigger fallback for hosts without pg_repack. |

All three are worker-only (`Processes: ["worker"]`), off-peak-gated
(`MarkOffPeak`), and admin-tunable via `DeclareConfig` knobs (tables, disk
safety multiplier, timeouts, index size/count thresholds). Each holds its own
mutex so a manual `/admin/jobs` trigger can't race the scheduled loop.

## Dependencies (`SetDeps`, worker process, before `core.Boot`)

The heavy table ops + disk probe are host seams; the scheduling and config
machinery come from `loon/schedule`. The host injects:

- **`Diag`** — `GetTableTotalSize`, `IsPGExtensionInstalled`, `GetIndexUsage`,
  `ReindexIndexConcurrently`. Three are primitive; only `GetIndexUsage` needs a
  small host-side conversion into the plugin's `IndexUsage` type.
- **`StatCache`** — persists each run's duration for the next run's ETA.
- **`Nzbs`** — `VacuumFullNzbs` (VACUUM FULL only).
- **`Maintenance`** — the host maintenance-mode gate (`Begin`/`End`);
  `middleware.Global` satisfies it. VACUUM FULL only.
- **`ConfigStore`** — `schedule.JobConfigStore` (the host JobRun repo) backing
  the admin knobs.
- **`FreeDisk`** — `func(ctx) (int64, error)` free bytes on the working volume;
  the host wraps gopsutil so this module stays dependency-light. Fail-soft: an
  error skips the disk pre-flight rather than blocking the run.
- **`Repack`** — the pg_repack CLI connection target (host/port/user/pass/db).

The `pg_repack` binary + extension are checked at runtime; if absent the Repack
job logs a clear install hint and skips (no error).

## Notes

- The off-peak / interval-override / CPU / panic hooks are installed **globally
  by the host** in `cmd/main.go`, so the plugin calls the bare
  `schedule.ServiceLoop` (no per-call hooks).
- Owns no tables and ships no migrations — it only reads catalog metadata and
  runs maintenance statements against host-owned tables.
