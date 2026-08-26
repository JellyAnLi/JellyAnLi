package parser

import (
	"path/filepath"
	"testing"
)

func TestCleanShowName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Kaijuu 8 Gou TV-2 [1080p]mp4 tag RUTRacker", "Kaijuu 8 Gou"},
		{"[SubsPlease] Shingeki no Kyojin - S4E12 (1080p) [1234ABCD]", "Shingeki no Kyojin"},
		{"Клинок рассекающий демонов 3 сезон (ТВ-3) 2023", "Клинок рассекающий демонов 2023"},
		{"Kimetsu no Yaiba - Katanakaji no Sato-hen [BD 1080p HEVC FLAC]", "Kimetsu no Yaiba Katanakaji no Sato hen"},
		{"Mushoku Tensei Isekai Ittara Honki Dasu II", "Mushoku Tensei Isekai Ittara Honki Dasu"},
		{"Mushoku Tensei Isekai Ittara Honki Dasu 2nd Season", "Mushoku Tensei Isekai Ittara Honki Dasu"},
		{"Mushoku Tensei Isekai Ittara Honki Dasu Part 2", "Mushoku Tensei Isekai Ittara Honki Dasu"},
	}

	for _, tt := range tests {
		actual := CleanShowName(tt.input)
		if actual != tt.expected {
			t.Errorf("CleanShowName(%q) = %q; expected %q", tt.input, actual, tt.expected)
		}
	}
}

func TestExtractShowNameFromFile(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"[SubsPlease] Solo Leveling - 13 (1080p) [69C63E88].mkv", "Solo Leveling"},
		{"[SubsPlease] Solo Leveling - S02E01 (1080p) [69C63E88].mkv", "Solo Leveling"},
		{"Jigokuraku - S01E01 [BDRip 1080p HEVC 10bit FLAC].mkv", "Jigokuraku"},
		{"[FortunaTV] Hell's Paradise S2E01 (WEBRip 1920x1080).mkv", "Hell's Paradise"},
		{"Erai-raws.Shangri-La.Frontier.S2E01.1080p.CR.WEB-DL.AVC.AAC.JPN.RUS.mkv", "Shangri La Frontier"},
	}

	for _, tt := range tests {
		actual := ExtractShowNameFromFile(tt.input)
		if actual != tt.expected {
			t.Errorf("ExtractShowNameFromFile(%q) = %q; expected %q", tt.input, actual, tt.expected)
		}
	}
}

func TestExtractSeason(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"Kaijuu 8 Gou TV-2 [1080p]", 2},
		{"Shingeki no Kyojin Season 4", 4},
		{"Клинок рассекающий демонов 3 сезон", 3},
		{"Kimetsu no Yaiba S02", 2},
		{"Some Anime TV-1", 1},
		{"Anime Without Season Number", 1},
		{"Mushoku Tensei Isekai Ittara Honki Dasu II", 2},
		{"Overlord IV", 4},
		{"Solo Leveling 2nd Season", 2},
		{"Spy x Family Part 2", 2},
	}

	for _, tt := range tests {
		actual := ExtractSeason(tt.input)
		if actual != tt.expected {
			t.Errorf("ExtractSeason(%q) = %d; expected %d", tt.input, actual, tt.expected)
		}
	}
}

func TestExtractEpisodeNumber(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"Kaijuu 8 Gou TV-2 - 01 [1080p CR WEB-DL AVC AAC].mkv", 1},
		{"[SubsPlease] Shingeki no Kyojin - 75 (1080p) [123456].mkv", 75},
		{"Show Name S02E05.mkv", 5},
		{"Show Name EP12.mkv", 12},
		{"Some_Anime_10_RUS.mka", 10},
		{"Movie No Episode.mkv", -1},
	}

	for _, tt := range tests {
		actual := ExtractEpisodeNumber(tt.input)
		if actual != tt.expected {
			t.Errorf("ExtractEpisodeNumber(%q) = %d; expected %d", tt.input, actual, tt.expected)
		}
	}
}

func TestParseEpisodeFile(t *testing.T) {
	langMap := map[string]string{
		"RUS Sound": "ru",
		"RUS Subs":  "ru",
		"ENG Sound": "en",
		"ENG Subs":  "en",
		"Rus Dub":   "ru",
		"Rus Sub":   "ru",
		"Eng Dub":   "en",
		"Eng Sub":   "en",
	}

	tests := []struct {
		path          string
		defaultSeason int
		expected      *EpisodeFile
	}{
		{
			path:          "Kaijuu 8 Gou TV-2 - 01 [1080p].mkv",
			defaultSeason: 2,
			expected: &EpisodeFile{
				SourcePath: "Kaijuu 8 Gou TV-2 - 01 [1080p].mkv",
				EpisodeNum: 1,
				SeasonNum:  2,
				Type:       TypeVideo,
				Suffix:     "",
				LangCode:   "",
			},
		},
		{
			path:          "RUS Sound/AniLiberty/Kaijuu 8 Gou TV-2 - 01 [1080p].AL.mka",
			defaultSeason: 1,
			expected: &EpisodeFile{
				SourcePath: "RUS Sound/AniLiberty/Kaijuu 8 Gou TV-2 - 01 [1080p].AL.mka",
				EpisodeNum: 1,
				SeasonNum:  2,
				Type:       TypeAudio,
				Suffix:     "AniLiberty.AL",
				LangCode:   "ru",
			},
		},
		{
			path:          "Сезон 2/RUS Sound/StudioBand/Solo Leveling - 01 [1080p].SB.mka",
			defaultSeason: 1,
			expected: &EpisodeFile{
				SourcePath: "Сезон 2/RUS Sound/StudioBand/Solo Leveling - 01 [1080p].SB.mka",
				EpisodeNum: 1,
				SeasonNum:  2,
				Type:       TypeAudio,
				Suffix:     "StudioBand.SB",
				LangCode:   "ru",
			},
		},
		{
			path:          "ENG Sound/Kaijuu 8 Gou TV-2 - 01 [1080p].CR.DUB.eng.mka",
			defaultSeason: 2,
			expected: &EpisodeFile{
				SourcePath: "ENG Sound/Kaijuu 8 Gou TV-2 - 01 [1080p].CR.DUB.eng.mka",
				EpisodeNum: 1,
				SeasonNum:  2,
				Type:       TypeAudio,
				Suffix:     "CR.dub",
				LangCode:   "en",
			},
		},
		{
			path:          "Rus Dub  [СВ-Дубль]/[SOFCJ-Raws] Death Note - 01 (BDRip 1920x1080 x264 VFR 10bit FLAC).mka",
			defaultSeason: 1,
			expected: &EpisodeFile{
				SourcePath: "Rus Dub  [СВ-Дубль]/[SOFCJ-Raws] Death Note - 01 (BDRip 1920x1080 x264 VFR 10bit FLAC).mka",
				EpisodeNum: 1,
				SeasonNum:  1,
				Type:       TypeAudio,
				Suffix:     "СВ-Дубль",
				LangCode:   "ru",
			},
		},
		{
			path:          "Rus Dub  [Мега-Аниме]/[SOFCJ-Raws] Death Note - 01 (BDRip 1920x1080 x264 VFR 10bit FLAC).mka",
			defaultSeason: 1,
			expected: &EpisodeFile{
				SourcePath: "Rus Dub  [Мега-Аниме]/[SOFCJ-Raws] Death Note - 01 (BDRip 1920x1080 x264 VFR 10bit FLAC).mka",
				EpisodeNum: 1,
				SeasonNum:  1,
				Type:       TypeAudio,
				Suffix:     "Мега-Аниме",
				LangCode:   "ru",
			},
		},
		{
			path:          "Rus Sub/Надписи/[SOFCJ-Raws] Death Note - 04 (BDRip 1920x1080 x264 VFR 10bit FLAC).ass",
			defaultSeason: 1,
			expected: &EpisodeFile{
				SourcePath: "Rus Sub/Надписи/[SOFCJ-Raws] Death Note - 04 (BDRip 1920x1080 x264 VFR 10bit FLAC).ass",
				EpisodeNum: 4,
				SeasonNum:  1,
				Type:       TypeSubtitle,
				Suffix:     "Надписи",
				LangCode:   "ru",
			},
		},
	}

	for _, tt := range tests {
		actual := ParseEpisodeFile(tt.path, tt.defaultSeason, 1, langMap)
		if actual == nil {
			t.Errorf("ParseEpisodeFile(%q) returned nil", tt.path)
			continue
		}

		if actual.SourcePath != tt.expected.SourcePath ||
			actual.EpisodeNum != tt.expected.EpisodeNum ||
			actual.SeasonNum != tt.expected.SeasonNum ||
			actual.Type != tt.expected.Type ||
			actual.Suffix != tt.expected.Suffix ||
			actual.LangCode != tt.expected.LangCode {
			t.Errorf("ParseEpisodeFile(%q) =\n%+v\nexpected:\n%+v", tt.path, actual, tt.expected)
		}
	}
}

func TestAlignEpisodeNumbers(t *testing.T) {
	langMap := map[string]string{
		"RUS Sound": "ru",
		"SUB":       "ru",
	}

	// Сценарий 1: Видео имеет правильную нумерацию (1..3), субтитры — сквозную (13..15)
	files1 := []string{
		"1 сезон/[SubsPlease] Solo Leveling - S01E01 (1080p).mkv",
		"1 сезон/SUB/[SubsPlease] Solo Leveling - 01 (1080p).ass",
		"Сезон 2/[SubsPlease] Solo Leveling - S02E01 (1080p).mkv",
		"Сезон 2/[SubsPlease] Solo Leveling - S02E02 (1080p).mkv",
		"Сезон 2/[SubsPlease] Solo Leveling - S02E03 (1080p).mkv",
		"Сезон 2/SUB/[SubsPlease] Solo Leveling - 13 (1080p).ass",
		"Сезон 2/SUB/[SubsPlease] Solo Leveling - 14 (1080p).ass",
		"Сезон 2/SUB/[SubsPlease] Solo Leveling - 15 (1080p).ass",
	}

	show1 := ParseShowFolder("Solo Leveling", files1, langMap)

	expectedEpisodes1 := map[string]int{
		"Сезон 2/[SubsPlease] Solo Leveling - S02E01 (1080p).mkv": 1,
		"Сезон 2/SUB/[SubsPlease] Solo Leveling - 13 (1080p).ass": 1, // Выровнено с 13 на 1
		"Сезон 2/SUB/[SubsPlease] Solo Leveling - 14 (1080p).ass": 2, // Выровнено с 14 на 2
		"Сезон 2/SUB/[SubsPlease] Solo Leveling - 15 (1080p).ass": 3, // Выровнено с 15 на 3
	}

	for _, f := range show1.Files {
		if expEp, ok := expectedEpisodes1[f.SourcePath]; ok {
			if f.EpisodeNum != expEp {
				t.Errorf("Scenario 1: file %s has episode %d; expected %d", f.SourcePath, f.EpisodeNum, expEp)
			}
		}
	}

	// Сценарий 2: Раздача Solo Leveling TV-2 (в отдельной папке).
	// Видео и сабы ВСЕ пронумерованы сквозным образом (13..15), так как 1 сезон лежит отдельно.
	// Алгоритм должен автоматически сдвинуть весь 2-й сезон на 12 серий назад, чтобы он начинался с 1 серии.
	files2 := []string{
		"[SubsPlease] Solo Leveling - 13 (1080p) [69C63E88].mkv",
		"[SubsPlease] Solo Leveling - 14 (1080p) [2FD84CD9].mkv",
		"[SubsPlease] Solo Leveling - 15 (1080p) [46344676].mkv",
		"RUS Sound/[SubsPlease] Solo Leveling - 13 (1080p) [69C63E88].mka",
		"RUS Sound/[SubsPlease] Solo Leveling - 14 (1080p) [2FD84CD9].mka",
		"SUB/[SubsPlease] Solo Leveling - 13 (1080p) [69C63E88].ass",
		"SUB/[SubsPlease] Solo Leveling - 14 (1080p) [2FD84CD9].ass",
	}

	// Название папки раздачи: "Solo Leveling TV-2"
	show2 := ParseShowFolder("Solo Leveling TV-2", files2, langMap)

	// Имя шоу должно определиться как "Solo Leveling" (из видеофайлов)
	if show2.CleanedName != "Solo Leveling" {
		t.Errorf("expected cleaned name 'Solo Leveling', got '%s'", show2.CleanedName)
	}

	// Серии 13..15 должны сдвинуться в 1..3
	expectedEpisodes2 := map[string]int{
		"[SubsPlease] Solo Leveling - 13 (1080p) [69C63E88].mkv":          1,
		"[SubsPlease] Solo Leveling - 14 (1080p) [2FD84CD9].mkv":          2,
		"RUS Sound/[SubsPlease] Solo Leveling - 13 (1080p) [69C63E88].mka": 1,
		"SUB/[SubsPlease] Solo Leveling - 14 (1080p) [2FD84CD9].ass":       2,
	}

	for _, f := range show2.Files {
		if expEp, ok := expectedEpisodes2[f.SourcePath]; ok {
			if f.EpisodeNum != expEp {
				t.Errorf("Scenario 2: file %s has episode %d; expected %d", f.SourcePath, f.EpisodeNum, expEp)
			}
		}
		// Проверим, что сезон равен 2 (определен из названия папки раздачи)
		if f.SeasonNum != 2 {
			t.Errorf("Scenario 2: file %s has season %d; expected 2", f.SourcePath, f.SeasonNum)
		}
	}
}

func TestParseShowFolder_ShangriLa(t *testing.T) {
	langMap := map[string]string{
		"RUS": "ru",
	}
	files := []string{
		"Erai-raws.Shangri-La.Frontier.S2E01.1080p.CR.WEB-DL.AVC.AAC.JPN.RUS.mkv",
	}
	show := ParseShowFolder("Shangri-La.Frontier.Season.2", files, langMap)

	if show.CleanedName != "Shangri La Frontier" {
		t.Errorf("expected cleaned name 'Shangri La Frontier', got '%s'", show.CleanedName)
	}

	if len(show.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(show.Files))
	}

	f := show.Files[0]
	if f.SeasonNum != 2 {
		t.Errorf("expected season 2, got %d", f.SeasonNum)
	}
	if f.EpisodeNum != 1 {
		t.Errorf("expected episode 1, got %d", f.EpisodeNum)
	}
}

func TestIgnoredBonusFolders(t *testing.T) {
	langMap := map[string]string{}
	bonusFiles := []string{
		"Bonus/BD menu/[Kawaiika-Raws] Kimetsu no Yaiba M01.mkv",
		"Bonus/Commentary/[Kawaiika-Raws] Kimetsu no Yaiba 01.jpn.opus.mka",
		"Bonus/Scans/Vol.01 - 01.webp",
		"Extras/Trailer.mkv",
		"OST/Track01.flac",
	}

	for _, f := range bonusFiles {
		epFile := ParseEpisodeFile(f, 1, 1, langMap)
		if epFile != nil {
			t.Errorf("expected file in bonus folder %s to be ignored (nil), got %+v", f, epFile)
		}
	}
}

func TestOreDakeLevelUpNaKen(t *testing.T) {
	files := []string{
		"01. Im Used to It.mkv",
		"08. This Is Frustrating.mkv",
		"12. Arise.mkv",
		"Amber/01. Im Used to It.mka",
		"Crunchyroll Sub/01. Im Used to It.ass",
	}
	langMap := map[string]string{
		"amber": "ru",
		"sub":   "ru",
	}

	show := ParseShowFolder("[BD-Remux] Ore dake Level Up na Ken", files, langMap)

	if show.CleanedName != "Ore dake Level Up na Ken" {
		t.Errorf("expected show name 'Ore dake Level Up na Ken', got '%s'", show.CleanedName)
	}
	if show.IsMovie {
		t.Errorf("expected show.IsMovie to be false, got true")
	}

	if len(show.Files) != 5 {
		t.Fatalf("expected 5 parsed files, got %d", len(show.Files))
	}

	epMap := make(map[string]int)
	for _, f := range show.Files {
		epMap[filepath.Base(f.SourcePath)] = f.EpisodeNum
	}

	if epMap["01. Im Used to It.mkv"] != 1 {
		t.Errorf("expected episode 1 for '01. Im Used to It.mkv', got %d", epMap["01. Im Used to It.mkv"])
	}
	if epMap["08. This Is Frustrating.mkv"] != 8 {
		t.Errorf("expected episode 8 for '08. This Is Frustrating.mkv', got %d", epMap["08. This Is Frustrating.mkv"])
	}
	if epMap["12. Arise.mkv"] != 12 {
		t.Errorf("expected episode 12 for '12. Arise.mkv', got %d", epMap["12. Arise.mkv"])
	}
}

func TestJudasBlackClover(t *testing.T) {
	folder := "[Judas]_Black_Clover_(170ep+Extras)_[BDrip_1080p][HEVC_x265_10bit][RUS+JP+ENG][Multi-Subs]"
	files := []string{
		"Episodes/Black_Clover_-_001_RUS_JP_ENG_Judas_BDrip_HEVC_10bit.mkv",
		"Episodes/Black_Clover_-_064_RUS_JP_ENG_Judas_BDrip_HEVC_10bit.mkv",
		"Episodes/Black_Clover_-_119_RUS_JP_ENG_Judas_BDrip_HEVC_10bit.mkv",
		"Episodes/Black_Clover_-_170_RUS_JP_ENG_Judas_BDrip_HEVC_10bit.mkv",
	}

	show := ParseShowFolder(folder, files, map[string]string{})

	if show.CleanedName != "Black Clover" {
		t.Errorf("expected cleaned show name 'Black Clover', got '%s'", show.CleanedName)
	}

	if len(show.Files) != 4 {
		t.Fatalf("expected 4 parsed files, got %d", len(show.Files))
	}

	expectedEpNums := map[string]int{
		"Episodes/Black_Clover_-_001_RUS_JP_ENG_Judas_BDrip_HEVC_10bit.mkv": 1,
		"Episodes/Black_Clover_-_064_RUS_JP_ENG_Judas_BDrip_HEVC_10bit.mkv": 64,
		"Episodes/Black_Clover_-_119_RUS_JP_ENG_Judas_BDrip_HEVC_10bit.mkv": 119,
		"Episodes/Black_Clover_-_170_RUS_JP_ENG_Judas_BDrip_HEVC_10bit.mkv": 170,
	}

	for _, f := range show.Files {
		expected, ok := expectedEpNums[f.SourcePath]
		if !ok {
			t.Errorf("unexpected file in results: %s", f.SourcePath)
		} else if f.EpisodeNum != expected {
			t.Errorf("expected episode %d for %s, got %d", expected, f.SourcePath, f.EpisodeNum)
		}
	}
}

func TestFiveExamplesFromInputLog(t *testing.T) {
	// Example 1: AniLibria bracketed episode numbers
	ex1Folder := "Fantasy Bishoujo Juniku Ojisan to - AniLibria.TV [WEBRip 1080p]"
	ex1Files := []string{
		"Fantasy_Bishoujo_Juniku_Ojisan_to_[01]_[AniLibria_TV]_[WEBRip_1080p].mkv",
		"Fantasy_Bishoujo_Juniku_Ojisan_to_[02]_[AniLibria_TV]_[WEBRip_1080p].mkv",
		"Fantasy_Bishoujo_Juniku_Ojisan_to_[12]_[AniLibria_TV]_[WEBRip_1080p].mkv",
	}
	show1 := ParseShowFolder(ex1Folder, ex1Files, nil)
	if show1.CleanedName != "Fantasy Bishoujo Juniku Ojisan to" {
		t.Errorf("Ex1: expected 'Fantasy Bishoujo Juniku Ojisan to', got '%s'", show1.CleanedName)
	}
	if len(show1.Files) != 3 {
		t.Fatalf("Ex1: expected 3 files, got %d", len(show1.Files))
	}
	if show1.Files[0].EpisodeNum != 1 || show1.Files[1].EpisodeNum != 2 || show1.Files[2].EpisodeNum != 12 {
		t.Errorf("Ex1: unexpected episode numbers: %d, %d, %d", show1.Files[0].EpisodeNum, show1.Files[1].EpisodeNum, show1.Files[2].EpisodeNum)
	}

	// Example 2: Katainaka no Ossan with Extra, NC, and OST folders
	ex2Folder := "Katainaka no Ossan, Kensei ni Naru BDRip 1080p [HEVC, FLAC]"
	ex2Files := []string{
		"Extra/[VCB-Studio] Katainaka no Ossan, Kensei ni Naru [Character PV 01][Ma10p_1080p][x265_flac].mkv",
		"Extra/[VCB-Studio] Katainaka no Ossan, Kensei ni Naru [CM01][Ma10p_1080p][x265_flac].mkv",
		"Extra/[VCB-Studio] Katainaka no Ossan, Kensei ni Naru [PV01][Ma10p_1080p][x265_flac].mkv",
		"Katainaka no Ossan, Kensei ni Naru - 01.mkv",
		"Katainaka no Ossan, Kensei ni Naru - 12.mkv",
		"NC/[VCB-Studio] Katainaka no Ossan, Kensei ni Naru [NCED][Ma10p_1080p][x265_flac].mkv",
		"NC/[VCB-Studio] Katainaka no Ossan, Kensei ni Naru [NCOP][Ma10p_1080p][x265_flac].mkv",
		"OST/[250521] HEROES (flac+webp)/01. HEROES.flac",
	}
	show2 := ParseShowFolder(ex2Folder, ex2Files, nil)
	if show2.CleanedName != "Katainaka no Ossan, Kensei ni Naru" {
		t.Errorf("Ex2: expected 'Katainaka no Ossan, Kensei ni Naru', got '%s'", show2.CleanedName)
	}
	if len(show2.Files) != 2 {
		t.Fatalf("Ex2: expected 2 episodes (extras, NC and OST ignored), got %d", len(show2.Files))
	}
	if show2.Files[0].EpisodeNum != 1 || show2.Files[1].EpisodeNum != 12 {
		t.Errorf("Ex2: unexpected episode numbers: %d, %d", show2.Files[0].EpisodeNum, show2.Files[1].EpisodeNum)
	}

	// Example 3: Dan Da Dan Season 2
	ex3Folder := "[VARYG] Dan Da Dan 2 (2025) [WEB-DL 1080p x264 AAC]"
	ex3Files := []string{
		"[VARYG] Dan Da Dan 2 (2025) - 01 [WEB-DL 1080p x264 AAC].mkv",
		"[VARYG] Dan Da Dan 2 (2025) - 12 [WEB-DL 1080p x264 AAC].mkv",
	}
	show3 := ParseShowFolder(ex3Folder, ex3Files, nil)
	if show3.CleanedName != "Dan Da Dan" {
		t.Errorf("Ex3: expected 'Dan Da Dan', got '%s'", show3.CleanedName)
	}
	if show3.Season != 2 {
		t.Errorf("Ex3: expected Season 2, got %d", show3.Season)
	}
	if len(show3.Files) != 2 {
		t.Fatalf("Ex3: expected 2 files, got %d", len(show3.Files))
	}
	if show3.Files[0].SeasonNum != 2 || show3.Files[0].EpisodeNum != 1 {
		t.Errorf("Ex3: expected S02E01, got S%02dE%02d", show3.Files[0].SeasonNum, show3.Files[0].EpisodeNum)
	}

	// Example 4: Yuusha-kei ni Shosu [TV-1]
	ex4Folder := "Yuusha-kei ni Shosu - Choubatsu Yuusha 9004 Tai Keimu Kiroku [TV-1] [2026] [WEBRip] [1080p] [RUS + JAP]"
	ex4Files := []string{
		"Yuusha-kei ni Shosu - 01 (WEBRip 1920x1080 x264 AAC Rus + Jap).mkv",
		"Yuusha-kei ni Shosu - 12 (WEBRip 1920x1080 x264 AAC Rus + Jap).mkv",
	}
	show4 := ParseShowFolder(ex4Folder, ex4Files, nil)
	if show4.CleanedName != "Yuusha kei ni Shosu" {
		t.Errorf("Ex4: expected 'Yuusha kei ni Shosu', got '%s'", show4.CleanedName)
	}
	if show4.Season != 1 {
		t.Errorf("Ex4: expected Season 1, got %d", show4.Season)
	}
	if len(show4.Files) != 2 {
		t.Fatalf("Ex4: expected 2 files, got %d", len(show4.Files))
	}

	// Example 5: Elfen Lied with Special 1
	ex5Folder := "Elfen Lied (BD 1920x1080 AVC FLAC)"
	ex5Files := []string{
		"Elfen Lied - 01 (BD 1920x1080 AVC FLAC).mkv",
		"Elfen Lied - 13 (BD 1920x1080 AVC FLAC).mkv",
		"Elfen Lied - Special 1 (BD 1920x1080 AVC FLAC).mkv",
	}
	show5 := ParseShowFolder(ex5Folder, ex5Files, nil)
	if show5.CleanedName != "Elfen Lied" {
		t.Errorf("Ex5: expected 'Elfen Lied', got '%s'", show5.CleanedName)
	}
	if len(show5.Files) != 3 {
		t.Fatalf("Ex5: expected 3 files, got %d", len(show5.Files))
	}
	if show5.Files[0].SeasonNum != 1 || show5.Files[0].EpisodeNum != 1 {
		t.Errorf("Ex5: file 0 expected S01E01, got S%02dE%02d", show5.Files[0].SeasonNum, show5.Files[0].EpisodeNum)
	}
	if show5.Files[1].SeasonNum != 1 || show5.Files[1].EpisodeNum != 13 {
		t.Errorf("Ex5: file 1 expected S01E13, got S%02dE%02d", show5.Files[1].SeasonNum, show5.Files[1].EpisodeNum)
	}
	if show5.Files[2].SeasonNum != 0 || show5.Files[2].EpisodeNum != 1 {
		t.Errorf("Ex5: special file expected S00E01, got S%02dE%02d", show5.Files[2].SeasonNum, show5.Files[2].EpisodeNum)
	}
}

