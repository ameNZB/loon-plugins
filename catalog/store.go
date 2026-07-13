package catalog

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/jmoiron/sqlx"

	"github.com/ameNZB/loon/catalog"
	"github.com/ameNZB/loon/core"
)

type store struct{ db *core.SchemaDB }

// upsertEntry persists a scraped catalog entry, deduped on (kind, external id).
func (s *store) upsertEntry(ctx context.Context, e catalog.CatalogEntry) error {
	ns, extID := "", ""
	if len(e.External) > 0 {
		ns, extID = e.External[0].Namespace, e.External[0].Value
	}
	fields, _ := json.Marshal(e.Fields)
	return s.db.WithTx(ctx, func(tx *sqlx.Tx) error {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO catalog_entry (kind, ext_namespace, ext_id, title, norm_title, cover_url, year, fields, updated_at)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8, now())
			 ON CONFLICT (kind, ext_namespace, ext_id) DO UPDATE SET
			   title = EXCLUDED.title, norm_title = EXCLUDED.norm_title,
			   cover_url = EXCLUDED.cover_url, year = EXCLUDED.year,
			   fields = EXCLUDED.fields, updated_at = now()`,
			e.Ref.Kind, ns, extID, e.Title, catalog.DefaultNormalize(e.Title), e.CoverURL, e.Year, fields)
		return err
	})
}

func (s *store) setReleaseCover(ctx context.Context, releaseID int64, coverURL string) error {
	return s.db.WithTx(ctx, func(tx *sqlx.Tx) error {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO release_cover (release_id, cover_url, updated_at) VALUES ($1, $2, now())
			 ON CONFLICT (release_id) DO UPDATE SET cover_url = EXCLUDED.cover_url, updated_at = now()`,
			releaseID, coverURL)
		return err
	})
}

func (s *store) releaseCover(ctx context.Context, releaseID int64) (string, bool, error) {
	var url string
	err := s.db.WithTx(ctx, func(tx *sqlx.Tx) error {
		e := tx.QueryRowContext(ctx, `SELECT cover_url FROM release_cover WHERE release_id = $1`, releaseID).Scan(&url)
		if e == sql.ErrNoRows {
			return nil
		}
		return e
	})
	return url, url != "", err
}

// disabledSet returns the top-level category ids an admin has turned off.
func (s *store) disabledSet(ctx context.Context) (map[int]bool, error) {
	out := map[int]bool{}
	err := s.db.WithTx(ctx, func(tx *sqlx.Tx) error {
		var ids []int
		if err := tx.SelectContext(ctx, &ids, `SELECT category_id FROM category_disabled`); err != nil {
			return err
		}
		for _, id := range ids {
			out[id] = true
		}
		return nil
	})
	return out, err
}

// setEnabled turns a top-level category on (delete the disabled row) or off
// (insert one).
func (s *store) setEnabled(ctx context.Context, categoryID int, enabled bool) error {
	return s.db.WithTx(ctx, func(tx *sqlx.Tx) error {
		if enabled {
			_, err := tx.ExecContext(ctx, `DELETE FROM category_disabled WHERE category_id = $1`, categoryID)
			return err
		}
		_, err := tx.ExecContext(ctx,
			`INSERT INTO category_disabled (category_id) VALUES ($1) ON CONFLICT DO NOTHING`, categoryID)
		return err
	})
}
