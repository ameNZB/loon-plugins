package pointstore

import (
	"context"
	"database/sql"

	"github.com/jmoiron/sqlx"

	"github.com/ameNZB/loon/core"
)

// Store is pointstore's persistence: which flair a user has equipped. The plugin
// depends on the interface, not a concrete DB (PGStore is the Postgres impl; the
// mock lives in store_test.go).
type Store interface {
	// Flair returns the user's equipped flair id, or "" if none.
	Flair(ctx context.Context, userID int64) (string, error)
	// SetFlair equips a flair for the user (upsert — replaces any current one).
	SetFlair(ctx context.Context, userID int64, flairID string) error
}

// PGStore is the Postgres implementation of Store (schema-scoped via SchemaDB;
// queries run through WithTx so search_path resolves the plugin schema).
type PGStore struct{ db *core.SchemaDB }

func NewPGStore(db *core.SchemaDB) *PGStore { return &PGStore{db: db} }

var _ Store = (*PGStore)(nil)

func (s *PGStore) Flair(ctx context.Context, userID int64) (string, error) {
	var id string
	err := s.db.WithTx(ctx, func(tx *sqlx.Tx) error {
		e := tx.QueryRowContext(ctx, `SELECT flair_id FROM user_flair WHERE user_id = $1`, userID).Scan(&id)
		if e == sql.ErrNoRows {
			return nil
		}
		return e
	})
	return id, err
}

func (s *PGStore) SetFlair(ctx context.Context, userID int64, flairID string) error {
	return s.db.WithTx(ctx, func(tx *sqlx.Tx) error {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO user_flair (user_id, flair_id, bought_at) VALUES ($1, $2, now())
			 ON CONFLICT (user_id) DO UPDATE SET flair_id = $2, bought_at = now()`,
			userID, flairID)
		return err
	})
}
