package usenet

import "testing"

func TestParseTags(t *testing.T) {
	tests := []struct {
		title string
		want  Tags
	}{
		{
			"Some.Show.S01E01.1080p.BluRay.x265.FLAC-GROUP",
			Tags{Resolution: "1080p", Source: "BluRay", Codec: "x265", Audio: "FLAC"},
		},
		{
			"Movie.2024.2160p.WEB-DL.DDP5.1.H.264-XYZ",
			Tags{Resolution: "2160p", Source: "WEB-DL", Codec: "x264"},
		},
		{
			"Anime Title - 05 [720p][HEVC][Dual Audio]",
			Tags{Resolution: "720p", Codec: "x265", Language: "Dual Audio"},
		},
		{
			"just a plain title with no tags",
			Tags{},
		},
	}
	for _, tc := range tests {
		got := parseTags(tc.title)
		if got != tc.want {
			t.Errorf("parseTags(%q)\n  got  %+v\n  want %+v", tc.title, got, tc.want)
		}
	}
}
