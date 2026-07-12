package usenet

import "testing"

func TestIsJunkTitle(t *testing.T) {
	junk := []string{
		"Pzz8CzBPoBNsCu8oRPpDYwESRkpq5UU3jGlzo8f7poeLWCLmU596hqnS0SA6eGPW", // the reported one
		"f2c8b393559540cfb9e33471cfda340c.par2",                            // hash + ext
		"550e8400-e29b-41d4-a716-446655440000",                             // UUID
		"OCazHDgoZua22m9UAFIHwxyz.part01.rar",                              // 24-char token + compound ext
		"n9pmKSuLKSyOP5wcDMLmnv_66qv9uJneqQusjTH4NZx_EY89VIWnGO_33zhz",     // underscore chaos
		"'qsptYFQA73GXgLh9IabcdEFGH12345678'",                              // quote-wrapped hash
		"season pack {total} files",                                        // template token mid-string
		"",                                                                 // empty
	}
	for _, s := range junk {
		if !isJunkTitle(s) {
			t.Errorf("isJunkTitle(%q) = false, want true", s)
		}
	}

	legit := []string{
		"[SubsPlease] Frieren - 12 (1080p) [ABCD1234].mkv",
		"Spy x Family S02E05 1080p WEB H264-Group",
		"Kaguya-sama.Love.is.War.S03.1080p.BluRay.x265-RARBG",
		"One Piece 1085 [720p]",
		"My Hero Academia - 138 VOSTFR",
	}
	for _, s := range legit {
		if isJunkTitle(s) {
			t.Errorf("isJunkTitle(%q) = true, want false (legit release)", s)
		}
	}
}
