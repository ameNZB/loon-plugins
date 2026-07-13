package dailyreward

import (
	"context"
	"testing"
)

// mockStore is an in-memory Store for testing plugin logic without a database.
type mockStore struct{ byUser map[int64]State }

func newMock() *mockStore { return &mockStore{byUser: map[int64]State{}} }

var _ Store = (*mockStore)(nil)

func (m *mockStore) Get(_ context.Context, userID int64) (State, error) {
	return m.byUser[userID], nil
}

func (m *mockStore) Claim(_ context.Context, userID int64, today, yesterday string) (int, int, bool, error) {
	cur := m.byUser[userID]
	if cur.LastClaim == today {
		return cur.Streak, 0, false, nil
	}
	streak := 1
	if cur.LastClaim == yesterday {
		streak = cur.Streak + 1
	}
	longest := cur.Longest
	if streak > longest {
		longest = streak
	}
	m.byUser[userID] = State{LastClaim: today, Streak: streak, Longest: longest, TotalClaims: cur.TotalClaims + 1}
	return streak, rewardFor(streak), true, nil
}

func TestRewardFor(t *testing.T) {
	cases := map[int]int{0: 10, 1: 10, 2: 15, 7: 40, 30: 40}
	for streak, want := range cases {
		if got := rewardFor(streak); got != want {
			t.Errorf("rewardFor(%d) = %d, want %d", streak, got, want)
		}
	}
}

func TestClaimStreak(t *testing.T) {
	ctx := context.Background()
	m := newMock()

	// day 1: first claim -> streak 1, reward 10
	streak, reward, claimed, _ := m.Claim(ctx, 1, "2026-07-01", "2026-06-30")
	if !claimed || streak != 1 || reward != 10 {
		t.Fatalf("day1 = streak %d reward %d claimed %v", streak, reward, claimed)
	}
	// same day again: no-op, unchanged streak
	if _, _, claimed, _ := m.Claim(ctx, 1, "2026-07-01", "2026-06-30"); claimed {
		t.Fatal("second claim same day should be a no-op")
	}
	// consecutive day: streak 2, reward 15
	streak, reward, _, _ = m.Claim(ctx, 1, "2026-07-02", "2026-07-01")
	if streak != 2 || reward != 15 {
		t.Fatalf("day2 = streak %d reward %d", streak, reward)
	}
	// gap day breaks the streak back to 1
	streak, _, _, _ = m.Claim(ctx, 1, "2026-07-10", "2026-07-09")
	if streak != 1 {
		t.Fatalf("after gap streak = %d, want 1", streak)
	}
	// longest is preserved across the reset
	if st, _ := m.Get(ctx, 1); st.Longest != 2 {
		t.Fatalf("longest = %d, want 2", st.Longest)
	}
}
