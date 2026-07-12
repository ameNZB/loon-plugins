package pluginapi

import (
	"math"
	"testing"
)

func TestGroupStatCoverage(t *testing.T) {
	const eps = 0.01
	close := func(a, b float64) bool { return math.Abs(a-b) < eps }

	tests := []struct {
		name             string
		g                GroupStat
		wantKnown        bool
		back, have, new_ float64
	}{
		{
			name:      "unknown span (never crawled)",
			g:         GroupStat{ServerLow: 0, ServerHigh: 0},
			wantKnown: false,
		},
		{
			name:      "fresh forward crawl, caught up (200-wide window)",
			g:         GroupStat{ServerLow: 1000, ServerHigh: 2000, HighWatermark: 2000, BackWatermark: 1800},
			wantKnown: true, back: 80, have: 20, new_: 0,
		},
		{
			name:      "mid: some backfill done, some new pending",
			g:         GroupStat{ServerLow: 1000, ServerHigh: 2000, HighWatermark: 1900, BackWatermark: 1400},
			wantKnown: true, back: 40, have: 50, new_: 10,
		},
		{
			name:      "backfill complete to server_low",
			g:         GroupStat{ServerLow: 1000, ServerHigh: 2000, HighWatermark: 2000, BackWatermark: 1000},
			wantKnown: true, back: 0, have: 100, new_: 0,
		},
		{
			name:      "back below server_low is clamped",
			g:         GroupStat{ServerLow: 1000, ServerHigh: 2000, HighWatermark: 2000, BackWatermark: 500},
			wantKnown: true, back: 0, have: 100, new_: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.g.Coverage()
			if c.Known != tt.wantKnown {
				t.Fatalf("Known = %v, want %v", c.Known, tt.wantKnown)
			}
			if !tt.wantKnown {
				return
			}
			if !close(c.BackPct, tt.back) || !close(c.HavePct, tt.have) || !close(c.NewPct, tt.new_) {
				t.Errorf("segments = back %.2f / have %.2f / new %.2f, want %.2f / %.2f / %.2f",
					c.BackPct, c.HavePct, c.NewPct, tt.back, tt.have, tt.new_)
			}
			if sum := c.BackPct + c.HavePct + c.NewPct; !close(sum, 100) {
				t.Errorf("segments sum to %.2f, want 100", sum)
			}
		})
	}
}
