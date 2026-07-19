package catalog

import (
	"strings"

	"github.com/the-loon-clan/loon-plugins/pluginapi"
)

// taxonomy is the standard Newznab category tree. Top-level ids are the
// thousands; the subcats are the common Newznab set. This is static data — the
// admin doesn't edit the tree, only which top-level categories are enabled.
var taxonomy = []pluginapi.Category{
	{ID: 1000, Name: "Console", Subcats: []pluginapi.Subcategory{
		{1010, "NDS"}, {1020, "PSP"}, {1030, "Wii"}, {1040, "Xbox"}, {1050, "Xbox 360"}, {1080, "PS3"}, {1180, "Other"}}},
	{ID: 2000, Name: "Movies", Subcats: []pluginapi.Subcategory{
		{2010, "Foreign"}, {2030, "SD"}, {2040, "HD"}, {2045, "UHD"}, {2050, "BluRay"}, {2060, "3D"}}},
	{ID: 3000, Name: "Audio", Subcats: []pluginapi.Subcategory{
		{3010, "MP3"}, {3030, "Audiobook"}, {3040, "Lossless"}, {3050, "Other"}}},
	{ID: 4000, Name: "PC", Subcats: []pluginapi.Subcategory{
		{4010, "0day"}, {4020, "ISO"}, {4030, "Mac"}, {4050, "Games"}, {4060, "Mobile-iOS"}, {4070, "Mobile-Android"}}},
	{ID: 5000, Name: "TV", Subcats: []pluginapi.Subcategory{
		{5020, "Foreign"}, {5030, "SD"}, {5040, "HD"}, {5045, "UHD"}, {5060, "Sport"}, {5070, "Anime"}, {5080, "Documentary"}}},
	{ID: 6000, Name: "XXX", Subcats: []pluginapi.Subcategory{
		{6010, "DVD"}, {6040, "x264"}, {6045, "UHD"}, {6050, "Pack"}, {6060, "ImgSet"}, {6070, "Other"}}},
	{ID: 7000, Name: "Books", Subcats: []pluginapi.Subcategory{
		{7010, "Mags"}, {7020, "Ebook"}, {7030, "Comics"}, {7040, "Technical"}}},
	{ID: 8000, Name: "Other", Subcats: []pluginapi.Subcategory{
		{8010, "Misc"}, {8020, "Hashed"}}},
}

// topLevelOf returns the thousands bucket a category id belongs to (5070 → 5000).
func topLevelOf(id int) int { return (id / 1000) * 1000 }

// keyword buckets for Categorize, checked in priority order (first match wins).
var catRules = []struct {
	cat      int
	keywords []string
}{
	{5070, []string{"anime", "subsplease", "erai-raws", "horriblesubs", "vostfr"}},
	{6070, []string{"xxx", "porn", "erotica", "brazzers", "onlyfans", "sex"}},
	{7020, []string{"ebook", "epub", "mobi", ".pdf", " pdf", "azw3"}},
	{7030, []string{"comic", "cbz", "cbr", "manga"}},
	{7010, []string{"magazine"}},
	{3040, []string{"flac", "lossless", "24bit", "dsd"}},
	{3030, []string{"audiobook"}},
	{3010, []string{"mp3", "320kbps", " m4a"}},
	{4010, []string{"crack", "keygen", "0day", "activator", "regged"}},
	{4020, []string{".iso", "installer", "portable", "setup"}},
	{4050, []string{"repack", "fitgirl", "dodi", "-codex", "-plaza", "-flt"}},
	{1000, []string{"nsw", "switch", "ps4", "ps5", "xbox", "-goldberg"}},
	{2050, []string{"bluray", "blu-ray", "remux"}},
	{5040, []string{"season", "hdtv", "pdtv"}},
}

// categorize maps a group + title to a best-fit Newznab category id. The TITLE
// is the primary signal (it's the most specific); the GROUP name is only a
// fallback for titles that keyword-match nothing — otherwise a group like
// "a.b.multimedia.anime" would force every release (even an ebook) to Anime.
func categorize(group, title string) int {
	if cat := categorizeText(strings.ToLower(title)); cat != 8010 {
		return cat
	}
	return groupCategory(strings.ToLower(group))
}

// categorizeText applies the keyword/episode/resolution rules to one string.
func categorizeText(h string) int {
	if hasEpisodePattern(h) {
		if strings.Contains(h, "anime") {
			return 5070
		}
		return 5040
	}
	for _, r := range catRules {
		for _, kw := range r.keywords {
			if strings.Contains(h, kw) {
				return r.cat
			}
		}
	}
	if strings.Contains(h, "1080p") || strings.Contains(h, "2160p") || strings.Contains(h, "720p") {
		return 2040 // resolution with no other signal → Movies/HD
	}
	return 8010
}

// groupCategory infers a category from the newsgroup name alone.
func groupCategory(g string) int {
	switch {
	case strings.Contains(g, "anime"):
		return 5070
	case strings.Contains(g, "erotica"), strings.Contains(g, "xxx"), strings.Contains(g, ".sex"):
		return 6070
	case strings.Contains(g, "ebook"), strings.Contains(g, "e-book"):
		return 7020
	case strings.Contains(g, "sound"), strings.Contains(g, "mp3"), strings.Contains(g, "music"):
		return 3010
	case strings.Contains(g, "console"), strings.Contains(g, "games"):
		return 1000
	case strings.Contains(g, "movie"):
		return 2040
	case strings.Contains(g, ".tv"), strings.Contains(g, "television"):
		return 5040
	}
	return 8010
}

// hasEpisodePattern spots S01E02 / 1x02 style markers.
func hasEpisodePattern(s string) bool {
	for i := 0; i+3 < len(s); i++ {
		// SxxExx
		if s[i] == 's' && isDigit(s[i+1]) && isDigit(s[i+2]) {
			for j := i + 3; j+2 < len(s) && j < i+6; j++ {
				if s[j] == 'e' && isDigit(s[j+1]) {
					return true
				}
			}
		}
	}
	return false
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

// categoryName returns "Parent/Sub" for a category id, or "Other" if unknown.
func categoryName(id int) string {
	top := topLevelOf(id)
	for _, c := range taxonomy {
		if c.ID == top {
			if id == top {
				return c.Name
			}
			for _, sub := range c.Subcats {
				if sub.ID == id {
					return c.Name + "/" + sub.Name
				}
			}
			return c.Name
		}
	}
	return "Other"
}
