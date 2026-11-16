package parser

import "testing"

func TestParseShowFolder_ShortFolderFullFileName(t *testing.T) {
	files := []string{
		"Re Zero kara Hajimeru Isekai Seikatsu III - 01 [BDRip 1080p HEVC 10bits FLAC].mkv",
		"Re Zero kara Hajimeru Isekai Seikatsu III - 02 [BDRip 1080p HEVC 10bits FLAC].mkv",
	}
	show := ParseShowFolder("Re Zero III", files, nil)
	if show.CleanedName != "Re Zero kara Hajimeru Isekai Seikatsu" {
		t.Fatalf("unexpected name: %q", show.CleanedName)
	}
	if show.Season != 3 {
		t.Fatalf("unexpected season: %d", show.Season)
	}
}
