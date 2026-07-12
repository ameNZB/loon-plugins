package usenet

import (
	"regexp"
	"strings"
)

// Tags is the quality metadata parsed from a release title.
type Tags struct {
	Resolution string // 2160p / 1080p / 720p / 480p
	Source     string // BluRay / WEB-DL / WEBRip / HDTV / DVD / Remux
	Codec      string // x265 / x264 / AV1 / XviD
	Audio      string // FLAC / AAC / DTS / AC3 / TrueHD / Opus
	Language   string // English / Japanese / Multi / Dual Audio / …
}

// Empty reports whether nothing was parsed.
func (t Tags) Empty() bool {
	return t.Resolution == "" && t.Source == "" && t.Codec == "" && t.Audio == "" && t.Language == ""
}

var (
	reRes    = regexp.MustCompile(`(?i)\b(2160p|1440p|1080p|720p|576p|480p|4k)\b`)
	reSource = regexp.MustCompile(`(?i)\b(blu-?ray|bd(?:rip|mux)?|web-?dl|web-?rip|hdtv|dvd(?:rip)?|remux)\b`)
	reCodec  = regexp.MustCompile(`(?i)\b(x265|x264|h\.?265|h\.?264|hevc|avc|av1|xvid|divx)\b`)
	reAudio  = regexp.MustCompile(`(?i)\b(flac|aac|dts(?:-hd)?|e?-?ac-?3|truehd|opus|mp3|pcm)\b`)
	reLang   = regexp.MustCompile(`(?i)\b(multi|dual[- ]?audio|english|eng|japanese|jpn|spanish|french|german|italian|dubbed|subbed)\b`)
)

// parseTags extracts quality tags from a release title. Best-effort; unmatched
// fields stay empty.
func parseTags(title string) Tags {
	return Tags{
		Resolution: normRes(reRes.FindString(title)),
		Source:     normSource(reSource.FindString(title)),
		Codec:      normCodec(reCodec.FindString(title)),
		Audio:      normAudio(reAudio.FindString(title)),
		Language:   normLang(reLang.FindString(title)),
	}
}

func normRes(s string) string {
	s = strings.ToLower(s)
	if s == "4k" {
		return "2160p"
	}
	return s
}

func normSource(s string) string {
	s = strings.ToLower(strings.ReplaceAll(s, "-", ""))
	switch {
	case strings.HasPrefix(s, "blu"), strings.HasPrefix(s, "bd"):
		return "BluRay"
	case strings.Contains(s, "webdl"):
		return "WEB-DL"
	case strings.Contains(s, "webrip"):
		return "WEBRip"
	case s == "hdtv":
		return "HDTV"
	case strings.HasPrefix(s, "dvd"):
		return "DVD"
	case s == "remux":
		return "Remux"
	}
	return s
}

func normCodec(s string) string {
	s = strings.ToLower(strings.ReplaceAll(s, ".", ""))
	switch s {
	case "x265", "h265", "hevc":
		return "x265"
	case "x264", "h264", "avc":
		return "x264"
	}
	return strings.ToUpper(s)
}

func normAudio(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(strings.ReplaceAll(s, "-", ""))
}

func normLang(s string) string {
	switch strings.ToLower(strings.ReplaceAll(s, " ", "-")) {
	case "eng", "english":
		return "English"
	case "jpn", "japanese":
		return "Japanese"
	case "dual-audio", "dualaudio":
		return "Dual Audio"
	case "":
		return ""
	}
	return capitalize(strings.ToLower(s))
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
