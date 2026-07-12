package usenet

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	reFileOf = regexp.MustCompile(`\[(\d+)/(\d+)\]`) // [1/12] multi-file header
	rePartOf = regexp.MustCompile(`\((\d+)/(\d+)\)`) // (1/45) yEnc segment marker
	reYenc   = regexp.MustCompile(`(?i)\byenc\b`)
	reExt    = regexp.MustCompile(`(?i)\.(mkv|mp4|avi|mov|ts|nfo|sfv|par2|rar|r\d{2,3}|nzb|zip|7z|mp3|flac|iso|img|srt|ass|jpg|png)\b`)
	reWS     = regexp.MustCompile(`\s+`)
)

// parseSubject parses a Usenet subject into the fields the crawler stages. It
// handles the two dominant yEnc forms:
//
//   - single-file:  Release.Name.ext (n/m) yEnc          -> base = release name
//   - multi-file:   Release Name [i/j] - "file" yEnc (n/m) -> base = release name
//     (the text before [i/j], shared by every file in the release, so they group
//     into ONE NZB)
//
// This is the multi-file-aware version: the base for an [i/j] release is the
// release name, not the per-file name, so completeness + assembly work at the
// release level (per-file segment counts, one <file> per file).
func parseSubject(subject string) (base string, partNum, totalParts, segTotal, fileNum, totalFiles int, fileParts bool) {
	partNum, totalParts = 1, 1

	fileLoc := reFileOf.FindStringSubmatchIndex(subject)
	if fileLoc != nil {
		fileNum = atoi(subject[fileLoc[2]:fileLoc[3]])
		totalFiles = atoi(subject[fileLoc[4]:fileLoc[5]])
		fileParts = totalFiles > 0
	}

	// The segment marker is the last (a/b). For single-file it sits BEFORE yEnc
	// (anything after is a file-count indicator); for [i/j] multi-file it
	// legitimately follows yEnc, so scan the whole subject.
	segScope := subject
	if !fileParts {
		if loc := reYenc.FindStringIndex(subject); loc != nil {
			segScope = subject[:loc[0]]
		}
	}
	if parts := rePartOf.FindAllStringSubmatch(segScope, -1); len(parts) > 0 {
		last := parts[len(parts)-1]
		partNum = atoi(last[1])
		totalParts = atoi(last[2])
	}
	if totalParts < 1 {
		totalParts = 1
	}
	segTotal = totalParts

	if fileParts {
		// Release name = the text before [i/j] (the "Title [i/j] - file" form),
		// shared by every file. Fall back to the per-file name if [i/j] is at the
		// very start.
		if release := cleanBase(subject[:fileLoc[0]]); release != "" {
			base = release
		} else {
			base = cleanBase(stripAllMarkers(subject))
		}
	} else {
		base = cleanBase(stripAllMarkers(subject))
	}
	if base == "" {
		base = strings.TrimSpace(subject)
	}
	return
}

// stripAllMarkers removes segment/file markers, the yEnc keyword, quotes, and a
// trailing extension — used to derive a single-file base from the whole subject.
func stripAllMarkers(s string) string {
	s = reFileOf.ReplaceAllString(s, " ")
	s = rePartOf.ReplaceAllString(s, " ")
	s = reYenc.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, `"`, " ")
	s = reExt.ReplaceAllString(s, "")
	return s
}

// cleanBase collapses whitespace and trims separator punctuation from the ends.
func cleanBase(s string) string {
	s = reWS.ReplaceAllString(s, " ")
	return strings.Trim(s, " -_:.")
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
