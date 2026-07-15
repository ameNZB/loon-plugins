package backup

import (
	"archive/zip"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"time"
)

// stampFormat is both the dated folder name and the last-run record. It sorts
// lexically = chronologically, which prune relies on.
const stampFormat = "2006-01-02_150405"

// run is the scheduled path: skip if a recent backup already exists.
func (p *Plugin) run(ctx context.Context) { p.doRun(ctx, false) }

// runForced is the /admin/jobs trigger: always back up. An operator pressing
// the button means "now", not "now unless you'd rather not".
func (p *Plugin) runForced(ctx context.Context) { p.doRun(ctx, true) }

func (p *Plugin) doRun(ctx context.Context, force bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	job := p.job
	job.SetRunning()
	defer job.SetIdle(time.Now().Add(backupIntervalMin * time.Minute))

	// The dated folders ARE the last-run record, so no new state is needed to
	// answer "did we back up recently?".
	//
	// This guard is what makes the boot delay safe, and the boot delay is what
	// makes the job work at all. The host service this replaces ran
	// `for { sleep(1 week); run() }` — it never ran at boot, and every restart
	// reset the week, so on any box redeployed more often than weekly the
	// backup simply never happened. Not failed: never started. A boot delay
	// alone would swing to the opposite failure (a full pg_dump + asset zip an
	// hour after every restart), so pair it with this.
	if !force {
		if age, ok := p.newestBackupAge(); ok && age < backupIntervalMin*time.Minute {
			job.Log("Skipped: last backup was %s ago (interval %s) — nothing due",
				age.Round(time.Minute), (backupIntervalMin * time.Minute).String())
			return
		}
	}

	stamp := time.Now().Format(stampFormat)
	dest := filepath.Join(deps.BackupDir, stamp)
	job.Log("Creating backup directory: %s", dest)
	if err := os.MkdirAll(dest, 0o755); err != nil {
		job.Log("ERROR creating directory: %v", err)
		return
	}

	mode := deps.Config.GetBackupMode(ctx)

	// ── Static assets ────────────────────────────────────────────────────────
	// One zip per source directory, named after the basename (covers ->
	// covers.zip) so a partial restore can target a single asset class. Skipped
	// in db_only mode — the asset dirs are the disk hog and are re-fetchable, so
	// a db_only run is just the DB.
	if mode == "db_only" {
		job.Log("db_only mode: skipping static-asset zips")
	} else {
		for _, src := range deps.StaticDirs {
			base := filepath.Base(src)
			zipOut := filepath.Join(dest, base+".zip")
			job.SetProgress("Zipping %s…", base)
			if err := zipDir(src, zipOut, job.Log); err != nil {
				job.Log("ERROR zipping %s: %v", base, err)
				continue
			}
			fi, _ := os.Stat(zipOut)
			job.Log("%s.zip written (%s)", base, fmtFileSize(fi))
		}
	}

	// ── Database dump ────────────────────────────────────────────────────────
	// pg_dump takes the whole database, so every plugin's tables are captured
	// without this plugin knowing any of them exist.
	job.SetProgress("Dumping database…")
	dbOut := filepath.Join(dest, "database.sql.gz")
	if err := dumpDB(deps.DB, dbOut); err != nil {
		job.Log("ERROR dumping database: %v", err)
	} else {
		fi, _ := os.Stat(dbOut)
		job.Log("database.sql.gz written (%s)", fmtFileSize(fi))
	}

	// Retention: prune the oldest dated folders so backups don't accumulate
	// forever (the original disk-space leak — there was no cleanup).
	p.prune(job.Log, deps.Config.GetBackupKeepCount(ctx))

	job.Log("Backup complete → %s", dest)
}

// newestBackupAge reports how long ago the most recent dated run folder was
// written. ok=false means there are none (or the dir is unreadable), which must
// be treated as "never backed up" — i.e. do run.
func (p *Plugin) newestBackupAge() (time.Duration, bool) {
	entries, err := os.ReadDir(deps.BackupDir)
	if err != nil {
		return 0, false
	}
	var newest time.Time
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		// ParseInLocation, not Parse: stampFormat carries no zone, so Format
		// writes local time while Parse would read it back as UTC. The original
		// code never noticed because it only parsed to validate the name shape
		// and then sorted lexically — this is the first code to compare the
		// stamp against now, and a plain Parse skews the age by the local
		// offset (measured: 9h where 2h was true, at UTC-7). East of Greenwich
		// the sign flips and a due backup gets skipped.
		t, perr := time.ParseInLocation(stampFormat, e.Name(), time.Local)
		if perr != nil {
			continue // not one of ours
		}
		if t.After(newest) {
			newest = t
		}
	}
	if newest.IsZero() {
		return 0, false
	}
	return time.Since(newest), true
}

// prune deletes the oldest dated backup folders, keeping the most recent
// `keep` (0 = keep all / pruning disabled). Only folders whose name parses as
// the backup stamp are touched, so unrelated files in BackupDir are safe.
func (p *Plugin) prune(logf func(string, ...any), keep int) {
	if keep <= 0 {
		return
	}
	entries, err := os.ReadDir(deps.BackupDir)
	if err != nil {
		logf("prune: read backup dir: %v", err)
		return
	}
	var dirs []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, perr := time.Parse(stampFormat, e.Name()); perr == nil {
			dirs = append(dirs, e.Name())
		}
	}
	if len(dirs) <= keep {
		return
	}
	sort.Strings(dirs) // stamp sorts lexically = chronologically; oldest first
	for _, d := range dirs[:len(dirs)-keep] {
		if err := os.RemoveAll(filepath.Join(deps.BackupDir, d)); err != nil {
			logf("prune: failed to remove %s: %v", d, err)
		} else {
			logf("Pruned old backup %s", d)
		}
	}
}

// zipDir walks src and writes every file into a zip archive at dest.
func zipDir(src, dest string, logf func(string, ...any)) error {
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return fmt.Errorf("source directory does not exist: %s", src)
	}

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	count := 0
	err = filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		w, err := zw.Create(rel)
		if err != nil {
			return err
		}
		r, err := os.Open(path)
		if err != nil {
			return err
		}
		defer r.Close()
		_, err = io.Copy(w, r)
		count++
		return err
	})
	if err != nil {
		return err
	}
	logf("Zipped %d files from %s", count, src)
	return nil
}

// dumpDB runs pg_dump and pipes the output through gzip into dest.
func dumpDB(conn PGConn, dest string) error {
	args := []string{
		"-h", conn.Host,
		"-p", fmt.Sprintf("%d", conn.Port),
		"-U", conn.User,
		"-d", conn.DBName,
		"--no-password",
	}
	cmd := exec.Command("pg_dump", args...)
	// PGPASSWORD rather than an argv flag: argv is world-readable via /proc.
	cmd.Env = append(os.Environ(), "PGPASSWORD="+conn.Password)

	pr, pw, err := os.Pipe()
	if err != nil {
		return err
	}
	cmd.Stdout = pw

	f, err := os.Create(dest)
	if err != nil {
		pw.Close()
		pr.Close()
		return err
	}

	gz := gzip.NewWriter(f)

	// Copy pg_dump stdout → gzip → file in a goroutine.
	done := make(chan error, 1)
	go func() {
		_, copyErr := io.Copy(gz, pr)
		pr.Close()
		gz.Close()
		f.Close()
		done <- copyErr
	}()

	if err := cmd.Start(); err != nil {
		pw.Close()
		pr.Close()
		return fmt.Errorf("pg_dump not found or failed to start: %w", err)
	}

	cmdErr := cmd.Wait()
	pw.Close() // signal EOF to the copy goroutine
	copyErr := <-done

	if cmdErr != nil {
		return fmt.Errorf("pg_dump: %w", cmdErr)
	}
	return copyErr
}

func fmtFileSize(fi os.FileInfo) string {
	if fi == nil {
		return "?"
	}
	b := fi.Size()
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.2f GB", float64(b)/float64(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(b)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
