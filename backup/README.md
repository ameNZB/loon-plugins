# backup

Weekly backup: zips every persistent static-asset directory and `pg_dump`s the
database into `<BackupDir>/YYYY-MM-DD_HHMMSS/`, then prunes old runs to a
retention count.

Worker-only. One job, **"Backup"** — the same name the host service used, so
`/admin/jobs` history and any interval override carry over.

## Why this extracts cleanly

It needs **no per-plugin cooperation**. The database half is a `pg_dump` of the
whole cluster, so every plugin's tables are captured without this plugin knowing
they exist. The only host knowledge is *which directories are persistent*, and
that is a `[]string`.

## Wiring

```go
backup.SetDeps(backup.Deps{
    DB:         backup.PGConn{Host: …, Port: …, User: …, Password: …, DBName: …},
    Config:     settingsService,          // GetBackupMode + GetBackupKeepCount
    StaticDirs: services.PersistentDirs,
    BackupDir:  "backups",
})
```

Call once in the worker process **before `core.Boot`**.

`Config` stays a host seam rather than becoming `loon/schedule` job-config vars:
the knobs already live in the host's admin surface, and moving them would
migrate live operator settings rather than extract a job.

## BackupDir must be a bind mount

Left inside a container's overlay filesystem it is wiped on every recreate,
which turns the whole job into theatre — it runs, logs success, and protects
nothing. In compose: `./backups:/app/backups`.

## Two behaviours worth knowing

**It runs an hour after boot, not a week.** The host service ran
`for { sleep(1 week); run() }`: it never ran at boot, and each restart reset the
week, so on any box redeployed more often than weekly the backup never happened
— not failed, never started. This runs on a 1h boot delay instead.

**A recent backup is skipped.** The boot delay alone would swing to the opposite
failure: a full `pg_dump` + asset zip an hour after every restart. So the
scheduled path skips when a run folder younger than the interval already exists.
The dated folders are the last-run record — no extra state. A manual
`/admin/jobs` trigger always forces: pressing Run means now.

## Tests

Cover the last-run guard (empty dir and unreadable dir read as *never*, so a
fresh install backs up; newest-not-last selection; foreign folders ignored) and
prune (keeps newest N, `keep<=0` disables, never touches non-backup files — the
dir is a bind mount an operator may keep other things in).

The suite runs under `TZ=UTC`, `America/Los_Angeles` and `Asia/Tokyo`, because
`stampFormat` carries no zone: `Format` writes local time, so the guard must
`ParseInLocation`. A plain `Parse` reads it back as UTC and skews the age by the
local offset — west of Greenwich it backs up too eagerly, east it skips a due
backup. The host code never hit this because it only parsed to validate the name
and sorted lexically.
