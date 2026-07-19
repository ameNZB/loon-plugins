package backup

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/the-loon-clan/loon/schedule"
	"time"
)

func withBackupDir(t *testing.T, stamps ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, s := range stamps {
		if err := os.MkdirAll(filepath.Join(dir, s), 0o755); err != nil {
			t.Fatalf("seed %s: %v", s, err)
		}
	}
	deps = &Deps{BackupDir: dir}
	t.Cleanup(func() { deps = nil })
	return dir
}

// The guard that makes the boot delay safe. Without it, pairing a 1h boot delay
// with a weekly job means a full pg_dump + asset zip an hour after every
// restart; without the boot delay, the job never runs at all on a box that
// redeploys more often than weekly (the bug this replaced).
func TestNewestBackupAge(t *testing.T) {
	t.Run("no backups reads as never, not as recent", func(t *testing.T) {
		withBackupDir(t)
		if _, ok := (&Plugin{}).newestBackupAge(); ok {
			t.Error("ok=true for an empty dir — a fresh install must back up, not skip")
		}
	})

	t.Run("unreadable dir reads as never", func(t *testing.T) {
		deps = &Deps{BackupDir: filepath.Join(t.TempDir(), "does-not-exist")}
		t.Cleanup(func() { deps = nil })
		if _, ok := (&Plugin{}).newestBackupAge(); ok {
			t.Error("ok=true for a missing dir — must fail toward backing up")
		}
	})

	t.Run("picks the newest, not the last read", func(t *testing.T) {
		old := time.Now().Add(-30 * 24 * time.Hour).Format(stampFormat)
		recent := time.Now().Add(-2 * time.Hour).Format(stampFormat)
		// Seed oldest last so a naive "last entry wins" would pick wrong.
		withBackupDir(t, recent, old)

		age, ok := (&Plugin{}).newestBackupAge()
		if !ok {
			t.Fatal("ok=false with backups present")
		}
		if age > 3*time.Hour {
			t.Errorf("age = %s, want ~2h — it found an older folder than the newest", age)
		}
	})

	t.Run("ignores folders that are not ours", func(t *testing.T) {
		withBackupDir(t, "notes", "restore-me", "README")
		if _, ok := (&Plugin{}).newestBackupAge(); ok {
			t.Error("ok=true — unrelated folders must not read as a backup")
		}
	})
}

// The pre-flight is the guard between "no backup today" and "the site is down".
// This job stages a full copy locally, so on a volume without room it fills the
// disk and the site starts erroring — an outage caused by the backup, which
// protects nothing and breaks what it was protecting.
func TestPreflight(t *testing.T) {
	const gb = int64(1) << 30

	newPlugin := func(t *testing.T, free, dbSize int64, mode string) *Plugin {
		t.Helper()
		assets := t.TempDir()
		// ~1 MB of "covers".
		if err := os.WriteFile(filepath.Join(assets, "cover.jpg"), make([]byte, 1<<20), 0o644); err != nil {
			t.Fatal(err)
		}
		deps = &Deps{
			BackupDir:  t.TempDir(),
			StaticDirs: []string{assets},
			Config:     stubConfig{mode: mode},
			FreeDisk:   func(context.Context) (int64, error) { return free, nil },
			DBSize:     func(context.Context) (int64, error) { return dbSize, nil },
		}
		t.Cleanup(func() { deps = nil })
		return &Plugin{job: schedule.RegisterJob("Backup test "+t.Name(), "")}
	}

	t.Run("refuses when the backup would not fit", func(t *testing.T) {
		p := newPlugin(t, 1*gb, 10*gb, "full") // 10GB DB, 1GB free
		if p.preflightOK(context.Background(), false) {
			t.Error("pre-flight passed with 1GB free for a 10GB database — this is the disk-full outage")
		}
	})

	t.Run("allows when there is ample room", func(t *testing.T) {
		p := newPlugin(t, 100*gb, 1*gb, "full")
		if !p.preflightOK(context.Background(), false) {
			t.Error("pre-flight refused with 100GB free for a 1GB database")
		}
	})

	t.Run("a manual trigger does not override it", func(t *testing.T) {
		p := newPlugin(t, 1*gb, 10*gb, "full")
		if p.preflightOK(context.Background(), true) {
			t.Error("force=true bypassed the disk pre-flight — an operator pressing Run is not asking for an outage")
		}
	})

	t.Run("db_only ignores asset size but still checks the dump", func(t *testing.T) {
		p := newPlugin(t, 1*gb, 10*gb, "db_only")
		if p.preflightOK(context.Background(), false) {
			t.Error("db_only passed with 1GB free for a 10GB dump")
		}
	})

	t.Run("an unreadable free-disk probe skips rather than guesses", func(t *testing.T) {
		p := newPlugin(t, 100*gb, 1*gb, "full")
		deps.FreeDisk = func(context.Context) (int64, error) { return 0, errors.New("boom") }
		if p.preflightOK(context.Background(), false) {
			t.Error("pre-flight proceeded despite not knowing free space — must fail toward not backing up")
		}
	})

	t.Run("an unreadable db-size probe skips rather than guesses", func(t *testing.T) {
		p := newPlugin(t, 100*gb, 1*gb, "full")
		deps.DBSize = func(context.Context) (int64, error) { return 0, errors.New("boom") }
		if p.preflightOK(context.Background(), false) {
			t.Error("pre-flight proceeded despite not knowing the dump size")
		}
	})
}

type stubConfig struct {
	mode string
	keep int
}

func (s stubConfig) GetBackupMode(context.Context) string   { return s.mode }
func (s stubConfig) GetBackupKeepCount(context.Context) int { return s.keep }

// prune must only ever touch its own dated folders: BackupDir is a bind mount
// an operator may keep other things in.
func TestPrune(t *testing.T) {
	logf := func(string, ...any) {}

	t.Run("keeps the newest N and deletes the rest", func(t *testing.T) {
		stamps := []string{
			"2026-01-01_000000", "2026-02-01_000000",
			"2026-03-01_000000", "2026-04-01_000000",
		}
		dir := withBackupDir(t, stamps...)

		(&Plugin{}).prune(logf, 2)

		for _, gone := range stamps[:2] {
			if _, err := os.Stat(filepath.Join(dir, gone)); !os.IsNotExist(err) {
				t.Errorf("%s survived, want pruned", gone)
			}
		}
		for _, kept := range stamps[2:] {
			if _, err := os.Stat(filepath.Join(dir, kept)); err != nil {
				t.Errorf("%s was pruned, want kept (it is one of the newest 2)", kept)
			}
		}
	})

	t.Run("keep<=0 disables pruning", func(t *testing.T) {
		dir := withBackupDir(t, "2026-01-01_000000", "2026-02-01_000000")
		(&Plugin{}).prune(logf, 0)
		entries, _ := os.ReadDir(dir)
		if len(entries) != 2 {
			t.Errorf("%d folders left, want 2 — keep=0 means retain everything", len(entries))
		}
	})

	t.Run("never touches foreign files", func(t *testing.T) {
		dir := withBackupDir(t, "2026-01-01_000000", "2026-02-01_000000", "2026-03-01_000000")
		foreign := filepath.Join(dir, "DO-NOT-DELETE.txt")
		if err := os.WriteFile(foreign, []byte("operator's"), 0o644); err != nil {
			t.Fatal(err)
		}
		foreignDir := filepath.Join(dir, "manual-restore")
		if err := os.MkdirAll(foreignDir, 0o755); err != nil {
			t.Fatal(err)
		}

		(&Plugin{}).prune(logf, 1)

		if _, err := os.Stat(foreign); err != nil {
			t.Error("prune deleted a non-backup file — BackupDir is a bind mount, not ours alone")
		}
		if _, err := os.Stat(foreignDir); err != nil {
			t.Error("prune deleted a non-backup directory")
		}
	})

	t.Run("fewer than keep is a no-op", func(t *testing.T) {
		dir := withBackupDir(t, "2026-01-01_000000")
		(&Plugin{}).prune(logf, 5)
		if _, err := os.Stat(filepath.Join(dir, "2026-01-01_000000")); err != nil {
			t.Error("pruned the only backup when keep=5")
		}
	})
}
