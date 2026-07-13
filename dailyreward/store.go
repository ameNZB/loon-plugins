package dailyreward

import (
	"context"
	"database/sql"

	"github.com/jmoiron/sqlx"

	"github.com/ameNZB/loon/core"
)

type store struct{ db *core.SchemaDB }

// state is a user's current reward standing. LastClaim is a civil date string
// ("YYYY-MM-DD") or "" if never claimed — kept as a string so streak arithmetic
// never depends on the process/DB timezone.
type state struct {
	LastClaim   string
	Streak      int
	Longest     int
	TotalClaims int
}

// Every query goes through SchemaDB.WithTx, which sets search_path to the
// plugin's schema so unqualified table names resolve there (the raw .DB() pool
// runs under the default search_path and would not find daily_rewards).

func (s *store) get(ctx context.Context, userID int64) (state, error) {
	var st state
	err := s.db.WithTx(ctx, func(tx *sqlx.Tx) error {
		var lc sql.NullString
		e := tx.QueryRowContext(ctx,
			`SELECT to_char(last_claim,'YYYY-MM-DD'), streak, longest, total_claims
			   FROM daily_rewards WHERE user_id = $1`, userID).
			Scan(&lc, &st.Streak, &st.Longest, &st.TotalClaims)
		if e == sql.ErrNoRows {
			return nil // no row yet: leave st zero
		}
		st.LastClaim = lc.String
		return e
	})
	return st, err
}

// claim records today's claim if not already done, atomically (read + write in
// one scoped tx). today/yesterday are civil date strings. Returns the new
// streak, the reward granted, and claimed=false (unchanged streak) if the user
// already claimed today.
func (s *store) claim(ctx context.Context, userID int64, today, yesterday string) (streak, reward int, claimed bool, err error) {
	err = s.db.WithTx(ctx, func(tx *sqlx.Tx) error {
		var lc sql.NullString
		var cur state
		e := tx.QueryRowContext(ctx,
			`SELECT to_char(last_claim,'YYYY-MM-DD'), streak, longest, total_claims
			   FROM daily_rewards WHERE user_id = $1 FOR UPDATE`, userID).
			Scan(&lc, &cur.Streak, &cur.Longest, &cur.TotalClaims)
		if e != nil && e != sql.ErrNoRows {
			return e
		}
		cur.LastClaim = lc.String

		if cur.LastClaim == today {
			streak, claimed = cur.Streak, false
			return nil
		}
		streak = 1
		if cur.LastClaim == yesterday {
			streak = cur.Streak + 1
		}
		longest := cur.Longest
		if streak > longest {
			longest = streak
		}
		if _, e := tx.ExecContext(ctx,
			`INSERT INTO daily_rewards (user_id, last_claim, streak, longest, total_claims)
			 VALUES ($1, $2::date, $3, $4, $5)
			 ON CONFLICT (user_id) DO UPDATE
			   SET last_claim = $2::date, streak = $3, longest = $4, total_claims = $5`,
			userID, today, streak, longest, cur.TotalClaims+1); e != nil {
			return e
		}
		reward, claimed = rewardFor(streak), true
		return nil
	})
	return streak, reward, claimed, err
}

// rewardFor is the points granted for a claim at the given streak length:
// 10 on day 1, +5 per consecutive day, capped at 40 (day 7+).
func rewardFor(streak int) int {
	bonus := streak - 1
	if bonus < 0 {
		bonus = 0
	}
	if bonus > 6 {
		bonus = 6
	}
	return 10 + bonus*5
}
