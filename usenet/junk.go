package usenet

import "regexp"

// Junk-title detection, ported from the prod site's isJunkTitle
// (indexer-site/pkg/services/nzb_assembler.go). Obfuscated Usenet posts use
// random-token subjects ("Pzz8CzBPoBNsCu8oRPpDYwESRkpq5UU3jGlz…") that would
// otherwise assemble into garbage "releases". We drop them at ingest (before
// staging) and again at build (defensive), and sweep already-staged/built junk
// in the prune job.
//
// This is the size-independent subset of prod's checks — the ones that need no
// assembled-size context. Software/warez and ROT13 patterns (anime-catalog
// specific) are intentionally omitted here.

var (
	// 24+ consecutive alphanumerics anywhere = a hash/token. The workhorse.
	reLongAlnumRun = regexp.MustCompile(`[A-Za-z0-9]{24,}`)
	// canonical UUID.
	reUUID = regexp.MustCompile(`(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b`)
	// leftover poster-template tokens: {total}, {{count}}.
	reTemplateToken = regexp.MustCompile(`\{[A-Za-z0-9_]+\}|\{\{[^}]+\}\}`)
	// dot-separated obfuscated pair: "f329yZ98AaYf2qHd.QPv2" (checked with the
	// upper+lower+digit gate below).
	reDotSepObfuscated = regexp.MustCompile(`^[A-Za-z0-9]{6,}\.[A-Za-z0-9]{1,12}$`)
	// structural gate for the multi-segment random-token check: only
	// alphanumerics, underscores, spaces (real titles carry other punctuation).
	reMultiSegSegmented = regexp.MustCompile(`^[A-Za-z0-9_ ]+$`)
	// trailing release extensions to peel before checks (compound first).
	reReleaseExtDynamic = regexp.MustCompile(`(?i)\.(vol\d+\+\d+\.par2|part\d+\.rar|r\d{2,3}|\d{3})$`)
)

var staticReleaseExts = []string{
	".par2", ".rar", ".7z", ".zip", ".tar", ".gz", ".nzb", ".enc",
	".sfv", ".nfo", ".mkv", ".mp4", ".avi", ".iso", ".bin", ".001",
}

// stripReleaseExts peels trailing release/archive extensions so a title like
// "f2c8b393559540cfb9e33471cfda340c.par2" reduces to the bare hash before the
// pattern checks run. Repeats until stable (handles ".part01.rar" etc.).
func stripReleaseExts(s string) string {
	s = trimSpace(s)
	for {
		n := len(s)
		if n == 0 {
			return s
		}
		if next := reReleaseExtDynamic.ReplaceAllString(s, ""); next != s {
			s = trimSpace(next)
			continue
		}
		stripped := false
		low := toLowerTail(s)
		for _, ext := range staticReleaseExts {
			if hasSuffix(low, ext) {
				s = trimSpace(s[:n-len(ext)])
				stripped = true
				break
			}
		}
		if !stripped {
			return s
		}
	}
}

// isJunkTitle reports whether a parsed release name is machine-generated junk.
func isJunkTitle(title string) bool {
	t := trimSpace(stripReleaseExts(title))
	// strip wrapping decoration posters put around hashes: 'x', {x}, [x], - x
	t = trimCut(t, "'\"{}[]- ")
	if len(t) == 0 {
		return true // empty after stripping
	}

	if reLongAlnumRun.MatchString(t) {
		return true
	}

	// multi-segment random chaos: 2+ segments (split on _ or space) of 5+ chars
	// that each mix upper, lower, AND digit. Real tokens rarely do all three,
	// almost never in two segments.
	if len(t) >= 24 && reMultiSegSegmented.MatchString(t) {
		chaotic := 0
		seg := []rune{}
		flush := func() bool {
			defer func() { seg = seg[:0] }()
			if len(seg) < 5 {
				return false
			}
			var u, l, d bool
			for _, c := range seg {
				switch {
				case c >= 'A' && c <= 'Z':
					u = true
				case c >= 'a' && c <= 'z':
					l = true
				case c >= '0' && c <= '9':
					d = true
				}
			}
			return u && l && d
		}
		for _, c := range t {
			if c == '_' || c == ' ' {
				if flush() {
					chaotic++
				}
				continue
			}
			seg = append(seg, c)
		}
		if flush() {
			chaotic++
		}
		if chaotic >= 2 {
			return true
		}
	}

	if reUUID.MatchString(t) {
		return true
	}
	if reTemplateToken.MatchString(t) {
		return true
	}
	if reDotSepObfuscated.MatchString(t) &&
		containsAny(t, 'A', 'Z') && containsAny(t, 'a', 'z') && containsAny(t, '0', '9') {
		return true
	}
	return false
}

// ── small string helpers (avoid importing strings twice across files) ──

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\n' || s[0] == '\r') {
		s = s[1:]
	}
	for len(s) > 0 {
		c := s[len(s)-1]
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			break
		}
		s = s[:len(s)-1]
	}
	return s
}

func trimCut(s, cutset string) string {
	in := func(b byte) bool {
		for i := 0; i < len(cutset); i++ {
			if cutset[i] == b {
				return true
			}
		}
		return false
	}
	for len(s) > 0 && in(s[0]) {
		s = s[1:]
	}
	for len(s) > 0 && in(s[len(s)-1]) {
		s = s[:len(s)-1]
	}
	return s
}

func toLowerTail(s string) string {
	start := len(s) - 8
	if start < 0 {
		start = 0
	}
	b := []byte(s[start:])
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}
	return string(b)
}

func hasSuffix(s, suf string) bool {
	return len(s) >= len(suf) && s[len(s)-len(suf):] == suf
}

func containsAny(s string, lo, hi byte) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= lo && s[i] <= hi {
			return true
		}
	}
	return false
}
