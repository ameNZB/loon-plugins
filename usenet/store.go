package usenet

import (
	"context"
	"database/sql"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/ameNZB/loon/core"

	"github.com/ameNZB/loon-plugins/pluginapi"
)

// store is the usenet-schema data layer. Every method runs through the
// SchemaDB's WithTx, which scopes search_path to "usenet" so unqualified table
// names resolve into the plugin's own schema.
type store struct{ db *core.SchemaDB }

func (s *store) searchNzbs(ctx context.Context, q string, limit int) ([]pluginapi.Release, error) {
	return s.queryReleases(ctx, `title ILIKE '%' || $1 || '%'`, q, limit)
}

func (s *store) browseNzbs(ctx context.Context, group string, limit int) ([]pluginapi.Release, error) {
	return s.queryReleases(ctx, `($1 = '' OR group_name = $1)`, group, limit)
}

// queryReleases lists completed NZBs newest-first. cond is a fixed literal
// referencing $1 (the search term or group name); arg flows through the
// placeholder, so there is no injection despite the concatenation.
func (s *store) queryReleases(ctx context.Context, cond, arg string, limit int) ([]pluginapi.Release, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	type row struct {
		ID         int64        `db:"id"`
		Title      string       `db:"title"`
		Size       int64        `db:"size"`
		Posted     sql.NullTime `db:"posted_at"`
		Group      string       `db:"group_name"`
		Resolution string       `db:"resolution"`
		Source     string       `db:"source"`
		Codec      string       `db:"video_codec"`
		Audio      string       `db:"audio"`
		Language   string       `db:"language"`
	}
	var rows []row
	err := s.db.WithTx(ctx, func(tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &rows,
			`SELECT id, title, size, posted_at, group_name,
			        resolution, source, video_codec, audio, language
			 FROM nzbs
			 WHERE status = 'completed' AND `+cond+`
			 ORDER BY COALESCE(posted_at, created_at) DESC LIMIT $2`, arg, limit)
	})
	if err != nil {
		return nil, err
	}
	out := make([]pluginapi.Release, len(rows))
	for i, r := range rows {
		out[i] = pluginapi.Release{
			ID: r.ID, Title: r.Title, Size: r.Size, Group: r.Group,
			Resolution: r.Resolution, Source: r.Source, Codec: r.Codec,
			Audio: r.Audio, Language: r.Language,
		}
		if r.Posted.Valid {
			out[i].Posted = r.Posted.Time
		}
	}
	return out, nil
}

// stats returns crawl progress: total NZBs, total staged articles, and per
// active-group status (NZBs, staged, last crawl, watermark vs server high).
func (s *store) stats(ctx context.Context) (pluginapi.IndexStats, error) {
	var st pluginapi.IndexStats
	err := s.db.WithTx(ctx, func(tx *sqlx.Tx) error {
		if err := tx.GetContext(ctx, &st.TotalNZBs, `SELECT COUNT(*) FROM nzbs`); err != nil {
			return err
		}
		if err := tx.GetContext(ctx, &st.TotalStaged, `SELECT COUNT(*) FROM articles`); err != nil {
			return err
		}
		type row struct {
			Name       string       `db:"name"`
			NZBs       int          `db:"nzbs"`
			Staged     int          `db:"staged"`
			LastCrawl  sql.NullTime `db:"last_crawl"`
			Watermark  int64        `db:"high_watermark"`
			HWDate     sql.NullTime `db:"high_watermark_date"`
			Back       int64        `db:"back_watermark"`
			BackDate   sql.NullTime `db:"back_watermark_date"`
			ServerLow  int64        `db:"server_low"`
			ServerHigh int64        `db:"server_high"`
			Done       bool         `db:"backfill_done"`
		}
		var rows []row
		if err := tx.SelectContext(ctx, &rows,
			`SELECT g.name, g.high_watermark, g.high_watermark_date,
			        COALESCE(g.back_watermark, g.high_watermark) AS back_watermark,
			        g.back_watermark_date, g.server_low, g.server_high, g.last_crawl, g.backfill_done,
			        (SELECT COUNT(*) FROM nzbs n WHERE n.group_name = g.name) AS nzbs,
			        (SELECT COUNT(*) FROM articles a WHERE a.group_name = g.name) AS staged
			 FROM newsgroups g WHERE g.active = TRUE ORDER BY g.name`); err != nil {
			return err
		}
		for _, r := range rows {
			gs := pluginapi.GroupStat{
				Name: r.Name, NZBs: r.NZBs, Staged: r.Staged,
				HighWatermark: r.Watermark, BackWatermark: r.Back,
				ServerLow: r.ServerLow, ServerHigh: r.ServerHigh, BackfillDone: r.Done,
			}
			if r.LastCrawl.Valid {
				gs.LastCrawl = r.LastCrawl.Time
			}
			if r.HWDate.Valid {
				gs.HighWatermarkDate = r.HWDate.Time
			}
			if r.BackDate.Valid {
				gs.BackWatermarkDate = r.BackDate.Time
			}
			if !r.Done && r.Back > r.ServerLow {
				st.TotalBackfillRemaining += r.Back - r.ServerLow
			}
			st.Groups = append(st.Groups, gs)
		}
		return nil
	})
	return st, err
}

func (s *store) groups(ctx context.Context) ([]pluginapi.GroupInfo, error) {
	type row struct {
		Name   string `db:"name"`
		Active bool   `db:"active"`
		NZBs   int64  `db:"nzbs"`
	}
	var rows []row
	err := s.db.WithTx(ctx, func(tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &rows,
			`SELECT g.name, g.active, COUNT(n.id) AS nzbs
			 FROM newsgroups g LEFT JOIN nzbs n ON n.group_name = g.name
			 WHERE g.active = TRUE
			 GROUP BY g.name, g.active ORDER BY g.name`)
	})
	if err != nil {
		return nil, err
	}
	out := make([]pluginapi.GroupInfo, len(rows))
	for i, r := range rows {
		out[i] = pluginapi.GroupInfo{Name: r.Name, Active: r.Active, NZBs: r.NZBs}
	}
	return out, nil
}

func (s *store) nzbData(ctx context.Context, id int64) ([]byte, string, error) {
	var data []byte
	var filename string
	err := s.db.WithTx(ctx, func(tx *sqlx.Tx) error {
		return tx.QueryRowContext(ctx,
			`SELECT nzb_data, filename FROM nzbs WHERE id = $1`, id).Scan(&data, &filename)
	})
	if err != nil {
		return nil, "", err
	}
	return data, filename, nil
}

func (s *store) getServer(ctx context.Context) (pluginapi.Server, bool, error) {
	var srv pluginapi.Server
	found := false
	err := s.db.WithTx(ctx, func(tx *sqlx.Tx) error {
		e := tx.QueryRowContext(ctx,
			`SELECT host, port, tls, username, password, enabled FROM servers ORDER BY id LIMIT 1`).
			Scan(&srv.Host, &srv.Port, &srv.TLS, &srv.Username, &srv.Password, &srv.Enabled)
		if e == sql.ErrNoRows {
			return nil
		}
		if e != nil {
			return e
		}
		found = true
		return nil
	})
	return srv, found, err
}

func (s *store) saveServer(ctx context.Context, srv pluginapi.Server) error {
	return s.db.WithTx(ctx, func(tx *sqlx.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM servers`); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx,
			`INSERT INTO servers (host, port, tls, username, password, enabled)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			srv.Host, srv.Port, srv.TLS, srv.Username, srv.Password, srv.Enabled)
		return err
	})
}

// upsertGroups inserts each name as an inactive group, ignoring duplicates.
// Returns how many were newly added.
func (s *store) upsertGroups(ctx context.Context, names []string) (int, error) {
	added := 0
	err := s.db.WithTx(ctx, func(tx *sqlx.Tx) error {
		for _, name := range names {
			res, err := tx.ExecContext(ctx,
				`INSERT INTO newsgroups (name) VALUES ($1) ON CONFLICT (name) DO NOTHING`, name)
			if err != nil {
				return err
			}
			if n, _ := res.RowsAffected(); n > 0 {
				added++
			}
		}
		return nil
	})
	return added, err
}

// allGroups returns up to limit groups, active first then alphabetical, for the
// admin picker. query filters by name substring so a 100k-group server is
// searchable instead of truncated to the first page.
func (s *store) allGroups(ctx context.Context, query string, limit int) ([]pluginapi.GroupInfo, error) {
	if limit <= 0 || limit > 5000 {
		limit = 500
	}
	type row struct {
		Name   string `db:"name"`
		Active bool   `db:"active"`
		NZBs   int64  `db:"nzbs"`
	}
	var rows []row
	err := s.db.WithTx(ctx, func(tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &rows,
			`SELECT g.name, g.active, COUNT(n.id) AS nzbs
			 FROM newsgroups g LEFT JOIN nzbs n ON n.group_name = g.name
			 WHERE ($1 = '' OR g.name ILIKE '%' || $1 || '%')
			 GROUP BY g.name, g.active
			 ORDER BY g.active DESC, g.name LIMIT $2`, query, limit)
	})
	if err != nil {
		return nil, err
	}
	out := make([]pluginapi.GroupInfo, len(rows))
	for i, r := range rows {
		out[i] = pluginapi.GroupInfo{Name: r.Name, Active: r.Active, NZBs: r.NZBs}
	}
	return out, nil
}

// groupCount returns the total number of fetched groups (so the picker can show
// "showing N of M" and reassure that a big LIST was fully imported).
func (s *store) groupCount(ctx context.Context) (int, error) {
	var n int
	err := s.db.WithTx(ctx, func(tx *sqlx.Tx) error {
		return tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM newsgroups`).Scan(&n)
	})
	return n, err
}

func (s *store) setGroupActive(ctx context.Context, name string, active bool) error {
	return s.db.WithTx(ctx, func(tx *sqlx.Tx) error {
		_, err := tx.ExecContext(ctx, `UPDATE newsgroups SET active = $2 WHERE name = $1`, name, active)
		return err
	})
}

// ── plugin settings (admin-editable knob overrides) ─────────────────

func (s *store) getSettings(ctx context.Context) (map[string]string, error) {
	out := map[string]string{}
	err := s.db.WithTx(ctx, func(tx *sqlx.Tx) error {
		var rows []struct {
			Key   string `db:"key"`
			Value string `db:"value"`
		}
		if err := tx.SelectContext(ctx, &rows, `SELECT key, value FROM settings`); err != nil {
			return err
		}
		for _, r := range rows {
			out[r.Key] = r.Value
		}
		return nil
	})
	return out, err
}

func (s *store) setSetting(ctx context.Context, key, value string) error {
	return s.db.WithTx(ctx, func(tx *sqlx.Tx) error {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO settings (key, value, updated_at) VALUES ($1, $2, now())
			 ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`,
			key, value)
		return err
	})
}

// ── backfill ────────────────────────────────────────────────────────

// backfillRow is one active group that still has history to fetch below its
// back_watermark.
type backfillRow struct {
	Name          string
	BackWatermark int64
	ServerLow     int64
}

// groupsNeedingBackfill lists active groups not yet marked done whose backfill
// pointer is still above the server's oldest article.
func (s *store) groupsNeedingBackfill(ctx context.Context, limit int) ([]backfillRow, error) {
	if limit <= 0 {
		limit = 20
	}
	type row struct {
		Name string `db:"name"`
		Back int64  `db:"back_watermark"`
		Low  int64  `db:"server_low"`
	}
	var rows []row
	err := s.db.WithTx(ctx, func(tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &rows,
			`SELECT name, COALESCE(back_watermark, high_watermark) AS back_watermark, server_low
			 FROM newsgroups
			 WHERE active = TRUE AND NOT backfill_done
			   AND COALESCE(back_watermark, high_watermark) > server_low
			 ORDER BY name LIMIT $1`, limit)
	})
	if err != nil {
		return nil, err
	}
	out := make([]backfillRow, len(rows))
	for i, r := range rows {
		out[i] = backfillRow{Name: r.Name, BackWatermark: r.Back, ServerLow: r.Low}
	}
	return out, nil
}

// updateBackWatermark lowers a group's backfill pointer and records the oldest
// posting date reached (kept if the batch had no dated articles).
func (s *store) updateBackWatermark(ctx context.Context, name string, back int64, oldest time.Time) error {
	return s.db.WithTx(ctx, func(tx *sqlx.Tx) error {
		var d sql.NullTime
		if !oldest.IsZero() {
			d = sql.NullTime{Time: oldest, Valid: true}
		}
		_, err := tx.ExecContext(ctx,
			`UPDATE newsgroups
			   SET back_watermark = $2,
			       back_watermark_date = COALESCE($3, back_watermark_date)
			 WHERE name = $1`, name, back, d)
		return err
	})
}

func (s *store) markBackfillDone(ctx context.Context, name string) error {
	return s.db.WithTx(ctx, func(tx *sqlx.Tx) error {
		_, err := tx.ExecContext(ctx, `UPDATE newsgroups SET backfill_done = TRUE WHERE name = $1`, name)
		return err
	})
}

// resetBackfill re-arms a group: backfill restarts just below the forward
// watermark and walks down again (dupes are ignored on insert).
func (s *store) resetBackfill(ctx context.Context, name string) error {
	return s.db.WithTx(ctx, func(tx *sqlx.Tx) error {
		_, err := tx.ExecContext(ctx,
			`UPDATE newsgroups
			   SET back_watermark = GREATEST(high_watermark - 1, server_low),
			       back_watermark_date = NULL, backfill_done = FALSE
			 WHERE name = $1`, name)
		return err
	})
}

// retagUntagged re-parses tags for NZBs that have none set (rows from before a
// parser change, or that genuinely had no tags in the title). Idempotent.
func (s *store) retagUntagged(ctx context.Context, limit int) (int, error) {
	if limit <= 0 {
		limit = 500
	}
	updated := 0
	err := s.db.WithTx(ctx, func(tx *sqlx.Tx) error {
		var rows []struct {
			ID    int64  `db:"id"`
			Title string `db:"title"`
		}
		if err := tx.SelectContext(ctx, &rows,
			`SELECT id, title FROM nzbs
			 WHERE resolution = '' AND source = '' AND video_codec = '' AND audio = '' AND language = ''
			 LIMIT $1`, limit); err != nil {
			return err
		}
		for _, r := range rows {
			t := parseTags(r.Title)
			if t.Empty() {
				continue
			}
			if _, err := tx.ExecContext(ctx,
				`UPDATE nzbs SET resolution=$2, source=$3, video_codec=$4, audio=$5, language=$6 WHERE id=$1`,
				r.ID, t.Resolution, t.Source, t.Codec, t.Audio, t.Language); err != nil {
				return err
			}
			updated++
		}
		return nil
	})
	return updated, err
}

func (s *store) pruneNzbs(ctx context.Context, days int) (int64, error) {
	var n int64
	err := s.db.WithTx(ctx, func(tx *sqlx.Tx) error {
		res, err := tx.ExecContext(ctx,
			`DELETE FROM nzbs WHERE COALESCE(posted_at, created_at) < now() - make_interval(days => $1)`, days)
		if err != nil {
			return err
		}
		n, _ = res.RowsAffected()
		return nil
	})
	return n, err
}

func (s *store) pruneStaging(ctx context.Context) (int64, error) {
	var n int64
	err := s.db.WithTx(ctx, func(tx *sqlx.Tx) error {
		res, err := tx.ExecContext(ctx, `DELETE FROM articles WHERE added_at < now() - INTERVAL '6 hours'`)
		if err != nil {
			return err
		}
		n, _ = res.RowsAffected()
		return nil
	})
	return n, err
}
