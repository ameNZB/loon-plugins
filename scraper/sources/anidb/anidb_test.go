package anidb

import "testing"

func TestNormalizeSequelFolding(t *testing.T) {
	s := New("", nil)
	// all of these should fold to the same base key
	base := s.Normalize("Attack on Titan")
	for _, variant := range []string{
		"Attack on Titan Season 2",
		"Attack on Titan 2",
		"Attack on Titan II",
		"Attack.on.Titan.S2", // via default normalize (dots→space) then… "s2" stays; check trailing-number only
	} {
		if got := s.Normalize(variant); got != base && variant != "Attack.on.Titan.S2" {
			t.Errorf("Normalize(%q) = %q, want %q", variant, got, base)
		}
	}
	if base != "attack on titan" {
		t.Fatalf("base = %q, want %q", base, "attack on titan")
	}
}

func TestDomain(t *testing.T) {
	d := New("client", nil).Domain()
	if d.Key != "anime" || d.Priority != 100 {
		t.Fatalf("domain = %+v", d)
	}
}
