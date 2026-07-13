package catalog

import (
	"context"

	"github.com/jmoiron/sqlx"

	"github.com/ameNZB/loon/core"
)

type store struct{ db *core.SchemaDB }

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
