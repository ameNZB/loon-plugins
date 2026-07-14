package dbmaint

import "testing"

func TestSplitCSV(t *testing.T) {
	cases := map[string][]string{
		"nzbs,posters":        {"nzbs", "posters"},
		" nzbs , posters ":    {"nzbs", "posters"}, // trimmed
		"nzbs,,posters":       {"nzbs", "posters"}, // empties dropped
		"nzbs":                {"nzbs"},
		"":                    nil,
		"  ":                  nil,
		",":                   nil,
		"nzbs, ,nzb_requests": {"nzbs", "nzb_requests"},
	}
	for in, want := range cases {
		got := splitCSV(in)
		if len(got) != len(want) {
			t.Errorf("splitCSV(%q) = %v, want %v", in, got, want)
			continue
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("splitCSV(%q)[%d] = %q, want %q", in, i, got[i], want[i])
			}
		}
	}
}

func TestHumanBytes(t *testing.T) {
	cases := map[int64]string{
		0:                      "0 B",
		512:                    "512 B",
		2 * 1024:               "2 KB",
		5 * 1024 * 1024:        "5.0 MB",
		3 * 1024 * 1024 * 1024: "3.00 GB",
	}
	for in, want := range cases {
		if got := humanBytes(in); got != want {
			t.Errorf("humanBytes(%d) = %q, want %q", in, got, want)
		}
	}
}
