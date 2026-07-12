package usenet

import (
	"context"
	"database/sql"

	"github.com/jmoiron/sqlx"

	"github.com/ameNZB/loon/core"

	"github.com/ameNZB/loon-plugins/pluginapi"
)

// store is the usenet-schema data layer. Every method runs through the
// SchemaDB's WithTx, which scopes search_path to "usenet" so unqualified table
// names resolve into the plugin's own schema.
type store struct{ db *core.SchemaDB }

func (s *store) searchNzbs(ctx context.Context, q string, limit int) ([]pluginapi.Release, error) {
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
			 WHERE status = 'completed' AND title ILIKE '%' || $1 || '%'
			 ORDER BY COALESCE(posted_at, created_at) DESC LIMIT $2`, q, limit)
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
