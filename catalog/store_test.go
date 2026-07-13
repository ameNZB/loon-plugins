package catalog

import (
	"context"
	"testing"

	"github.com/ameNZB/loon/catalog"
)

// mockStore is an in-memory Store for testing without a database.
type mockStore struct {
	entries  []catalog.CatalogEntry
	covers   map[int64]string
	disabled map[int]bool
}

func newMock() *mockStore {
	return &mockStore{covers: map[int64]string{}, disabled: map[int]bool{}}
}

var _ Store = (*mockStore)(nil)

func (m *mockStore) UpsertEntry(_ context.Context, e catalog.CatalogEntry) error {
	m.entries = append(m.entries, e)
	return nil
}
func (m *mockStore) SetReleaseCover(_ context.Context, id int64, url string) error {
	m.covers[id] = url
	return nil
}
func (m *mockStore) ReleaseCover(_ context.Context, id int64) (string, bool, error) {
	u, ok := m.covers[id]
	return u, ok, nil
}
func (m *mockStore) DisabledSet(_ context.Context) (map[int]bool, error) {
	out := map[int]bool{}
	for k, v := range m.disabled {
		if v {
			out[k] = true
		}
	}
	return out, nil
}
func (m *mockStore) SetEnabled(_ context.Context, id int, enabled bool) error {
	if enabled {
		delete(m.disabled, id)
	} else {
		m.disabled[id] = true
	}
	return nil
}

func TestCategoryToggle(t *testing.T) {
	ctx := context.Background()
	m := newMock()

	// disable category 6000, enable it back
	_ = m.SetEnabled(ctx, 6000, false)
	if d, _ := m.DisabledSet(ctx); !d[6000] {
		t.Fatal("6000 should be disabled")
	}
	_ = m.SetEnabled(ctx, 6000, true)
	if d, _ := m.DisabledSet(ctx); d[6000] {
		t.Fatal("6000 should be enabled again")
	}
}

func TestReleaseCoverRoundTrip(t *testing.T) {
	ctx := context.Background()
	m := newMock()
	if _, ok, _ := m.ReleaseCover(ctx, 42); ok {
		t.Fatal("no cover expected yet")
	}
	_ = m.SetReleaseCover(ctx, 42, "http://x/c.jpg")
	if u, ok, _ := m.ReleaseCover(ctx, 42); !ok || u != "http://x/c.jpg" {
		t.Fatalf("cover = %q ok=%v", u, ok)
	}
}
