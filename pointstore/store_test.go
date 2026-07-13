package pointstore

import (
	"context"
	"testing"
)

// mockStore is an in-memory Store for testing without a database.
type mockStore struct{ byUser map[int64]string }

func newMock() *mockStore { return &mockStore{byUser: map[int64]string{}} }

var _ Store = (*mockStore)(nil)

func (m *mockStore) Flair(_ context.Context, userID int64) (string, error) {
	return m.byUser[userID], nil
}
func (m *mockStore) SetFlair(_ context.Context, userID int64, flairID string) error {
	m.byUser[userID] = flairID
	return nil
}

func TestFlairCatalog(t *testing.T) {
	if _, ok := flairByID("nope"); ok {
		t.Fatal("unknown flair should not resolve")
	}
	f, ok := flairByID("vip")
	if !ok || f.Cost != 50 {
		t.Fatalf("vip = %+v ok=%v", f, ok)
	}
}

func TestSetAndReplaceFlair(t *testing.T) {
	ctx := context.Background()
	m := newMock()
	if fl, _ := m.Flair(ctx, 1); fl != "" {
		t.Fatal("no flair expected initially")
	}
	_ = m.SetFlair(ctx, 1, "supporter")
	if fl, _ := m.Flair(ctx, 1); fl != "supporter" {
		t.Fatalf("flair = %q", fl)
	}
	// buying another replaces it (one equipped flair per user)
	_ = m.SetFlair(ctx, 1, "legend")
	if fl, _ := m.Flair(ctx, 1); fl != "legend" {
		t.Fatalf("flair after replace = %q", fl)
	}
}
