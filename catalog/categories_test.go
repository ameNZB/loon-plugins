package catalog

import "testing"

func TestCategorize(t *testing.T) {
	cases := map[string]int{
		"Rich Dad Poor Dad Ebook pdf":        7020, // Books/Ebook
		"[SubsPlease] Frieren - 12 (1080p)":  5070, // TV/Anime
		"Some.Movie.2024.1080p.BluRay.x264":  2050, // Movies/BluRay
		"Show.S01E05.1080p.WEB":              5040, // TV/HD (episode pattern)
		"Ashampoo Burning Studio Crack 2024": 4010, // PC/0day
		"Great Album - Artist 2023 FLAC":     3040, // Audio/Lossless
		"Some Random Doc 1080p":              2040, // Movies/HD (resolution fallback)
		"Strategic Trading Planner":          8010, // Other/Misc
	}
	for title, want := range cases {
		if got := categorize("", title); got != want {
			t.Errorf("categorize(%q) = %d (%s), want %d (%s)", title, got, categoryName(got), want, categoryName(want))
		}
	}

	// group signal: an anime group categorizes as TV/Anime even for a plain title.
	if got := categorize("a.b.multimedia.anime", "Episode 5"); got != 5070 {
		t.Errorf("anime group = %d, want 5070", got)
	}
}
