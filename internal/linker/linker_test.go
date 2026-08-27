package linker

import (
	"fmt"
	"os"
	"strings"
	"path/filepath"
	"testing"

	"jelly-an-li/internal/config"
	"jelly-an-li/internal/parser"
	"jelly-an-li/internal/providers"
)

func TestRelativeSymlinkCalc(t *testing.T) {
	targetDir := "/DATA/JellyFin/Аниме/Jigokuraku/Season 01"
	absSource := "/DATA/Аниме/Адский рай/1 сезон/Jigokuraku - S01E01 [BDRip 1080p HEVC 10bit FLAC].mkv"

	rel, err := filepath.Rel(targetDir, absSource)
	if err != nil {
		t.Fatalf("filepath.Rel failed: %v", err)
	}
	expected := "../../../../Аниме/Адский рай/1 сезон/Jigokuraku - S01E01 [BDRip 1080p HEVC 10bit FLAC].mkv"
	if rel != expected {
		t.Errorf("expected relative path:\n%s\ngot:\n%s", expected, rel)
	}
}

func TestUpgradeAbsoluteToRelativeSymlink(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "jelly-an-li-upgrade-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	sourceFile := filepath.Join(tmpDir, "Torrents", "Show", "EP01.mkv")
	os.MkdirAll(filepath.Dir(sourceFile), 0755)
	os.WriteFile(sourceFile, []byte("test"), 0644)

	targetFile := filepath.Join(tmpDir, "Library", "Show", "Season 01", "Show S01E01.mkv")
	os.MkdirAll(filepath.Dir(targetFile), 0755)

	// Создаем сначала абсолютную ссылку на диске
	if err := os.Symlink(sourceFile, targetFile); err != nil {
		t.Fatalf("failed to create initial absolute symlink: %v", err)
	}

	plan := []*LinkOperation{
		{
			SourcePath: sourceFile,
			TargetPath: targetFile,
			Status:     StatusPending,
		},
	}

	cfg := &config.Config{
		UseRelativeSymlinks: true,
	}

	statePath := filepath.Join(tmpDir, "state.json")
	if err := ApplyPlan(plan, statePath, cfg); err != nil {
		t.Fatalf("ApplyPlan failed: %v", err)
	}

	// Проверяем, что ссылка пересоздана и теперь она относительная (не абсолютная)
	linkTarget, err := os.Readlink(targetFile)
	if err != nil {
		t.Fatalf("failed to readlink: %v", err)
	}

	if filepath.IsAbs(linkTarget) {
		t.Errorf("expected relative symlink target, but got absolute target: %s", linkTarget)
	}
}

func TestLinkerWorkflow(t *testing.T) {
	// Создаем временную папку для тестов
	tmpDir, err := os.MkdirTemp("", "jelly-an-li-linker-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Инициализируем папки
	torrentsDir := filepath.Join(tmpDir, "Torrents")
	jellyLibraryDir := filepath.Join(tmpDir, "Jellyfin")

	os.MkdirAll(torrentsDir, 0755)
	os.MkdirAll(jellyLibraryDir, 0755)

	// Создаем фейковую раздачу аниме
	showFolder := filepath.Join(torrentsDir, "Kaijuu 8 Gou TV-2 [1080p]")
	os.MkdirAll(filepath.Join(showFolder, "RUS Sound", "AniLiberty"), 0755)

	// Файлы
	videoFile := filepath.Join(showFolder, "Kaijuu 8 Gou TV-2 - 01 [1080p].mkv")
	audioFile := filepath.Join(showFolder, "RUS Sound", "AniLiberty", "Kaijuu 8 Gou TV-2 - 01 [1080p].AL.mka")

	os.WriteFile(videoFile, []byte("video content"), 0644)
	os.WriteFile(audioFile, []byte("audio content"), 0644)

	// Настройки
	cfg := &config.Config{
		TorrentDirs:     []string{torrentsDir},
		LibraryDir:      jellyLibraryDir,
		LanguageMapping: map[string]string{
			"RUS Sound": "ru",
		},
	}

	// 1. Сканирование
	shows, err := Scan(cfg)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if len(shows) != 1 {
		t.Fatalf("expected 1 show, got %d", len(shows))
	}

	show := shows[0]
	if show.CleanedName != "Kaijuu 8 Gou" || show.Season != 2 {
		t.Errorf("incorrect show parse: name=%s, season=%d", show.CleanedName, show.Season)
	}

	// 2. Генерация плана
	plan := GeneratePlan(shows, cfg)
	// Должно быть 2 операции: видео, аудио
	if len(plan) != 2 {
		t.Fatalf("expected 2 link operations, got %d", len(plan))
	}

	// Проверяем пути
	expectedPaths := map[string]string{
		videoFile: filepath.Join(jellyLibraryDir, "Kaijuu 8 Gou", "Season 02", "Kaijuu 8 Gou S02E01.mkv"),
		audioFile: filepath.Join(jellyLibraryDir, "Kaijuu 8 Gou", "Season 02", "Kaijuu 8 Gou S02E01.AniLiberty.AL.ru.mka"),
	}

	for _, op := range plan {
		expectedTarget, ok := expectedPaths[op.SourcePath]
		if !ok {
			t.Errorf("unexpected source path in plan: %s", op.SourcePath)
		}
		if op.TargetPath != expectedTarget {
			t.Errorf("expected target path for %s to be %s, got %s", op.SourcePath, expectedTarget, op.TargetPath)
		}
		if op.Status != StatusPending {
			t.Errorf("expected status 'pending', got '%s'", op.Status)
		}
	}

	// 3. Применение плана
	err = ApplyPlan(plan, filepath.Join(tmpDir, "state.json"))
	if err != nil {
		t.Fatalf("ApplyPlan failed: %v", err)
	}

	// Проверяем статус операций
	for _, op := range plan {
		if op.Status != StatusSuccess {
			t.Errorf("expected operation status 'success', got '%s' (message: %s)", op.Status, op.Message)
		}
		// Проверяем физическое существование симлинка
		info, err := os.Lstat(op.TargetPath)
		if err != nil {
			t.Errorf("link not created: %s", op.TargetPath)
			continue
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Errorf("path is not a symlink: %s", op.TargetPath)
		}
	}

	// 4. Повторное применение плана (должно быть пропущено/skipped)
	plan2 := GeneratePlan(shows, cfg)
	err = ApplyPlan(plan2, filepath.Join(tmpDir, "state.json"))
	if err != nil {
		t.Fatalf("second ApplyPlan failed: %v", err)
	}

	for _, op := range plan2 {
		if op.Status != StatusSkipped {
			t.Errorf("expected status 'skipped' on repeat run, got '%s'", op.Status)
		}
	}

	// 5. Очистка нерабочих ссылок
	// Удалим исходные файлы из торрентов
	os.RemoveAll(showFolder)

	// Запустим очистку
	cleaned, err := CleanBrokenLinks(cfg, "")
	if err != nil {
		t.Fatalf("CleanBrokenLinks failed: %v", err)
	}

	// Должно быть удалено как минимум 2 симлинка и папки
	if len(cleaned) < 2 {
		t.Errorf("expected at least 2 items cleaned, got %d: %v", len(cleaned), cleaned)
	}

	// Проверим, что папки в Jellyfin пустые и симлинки удалены
	for _, op := range plan {
		if _, err := os.Lstat(op.TargetPath); !os.IsNotExist(err) {
			t.Errorf("link was not cleaned: %s", op.TargetPath)
		}
	}
}

func TestLinkerDeathNoteMultipleVoiceovers(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "jelly-an-li-linker-test-multi")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	torrentsDir := filepath.Join(tmpDir, "Torrents")
	jellyLibraryDir := filepath.Join(tmpDir, "Jellyfin")

	os.MkdirAll(torrentsDir, 0755)
	os.MkdirAll(jellyLibraryDir, 0755)

	// Создаем структуру папок
	showFolder := filepath.Join(torrentsDir, "[SOFCJ-Raws] Death Note (BDRip 1920x1080 x264 10bit VFR FLAC)")
	os.MkdirAll(filepath.Join(showFolder, "Rus Dub  [СВ-Дубль]"), 0755)
	os.MkdirAll(filepath.Join(showFolder, "Rus Dub  [Мега-Аниме]"), 0755)
	os.MkdirAll(filepath.Join(showFolder, "Rus Sub"), 0755)
	os.MkdirAll(filepath.Join(showFolder, "Rus Sub", "Надписи"), 0755)

	// Создаем файлы
	videoFile := filepath.Join(showFolder, "[SOFCJ-Raws] Death Note - 01 (BDRip 1920x1080 x264 VFR 10bit FLAC).mkv")
	dub1File := filepath.Join(showFolder, "Rus Dub  [СВ-Дубль]", "[SOFCJ-Raws] Death Note - 01 (BDRip 1920x1080 x264 VFR 10bit FLAC).mka")
	dub2File := filepath.Join(showFolder, "Rus Dub  [Мега-Аниме]", "[SOFCJ-Raws] Death Note - 01 (BDRip 1920x1080 x264 VFR 10bit FLAC).mka")
	subFile := filepath.Join(showFolder, "Rus Sub", "[SOFCJ-Raws] Death Note - 01 (BDRip 1920x1080 x264 VFR 10bit FLAC).ass")
	subSignsFile := filepath.Join(showFolder, "Rus Sub", "Надписи", "[SOFCJ-Raws] Death Note - 01 (BDRip 1920x1080 x264 VFR 10bit FLAC).ass")

	os.WriteFile(videoFile, []byte("mkv"), 0644)
	os.WriteFile(dub1File, []byte("mka1"), 0644)
	os.WriteFile(dub2File, []byte("mka2"), 0644)
	os.WriteFile(subFile, []byte("ass1"), 0644)
	os.WriteFile(subSignsFile, []byte("ass2"), 0644)

	cfg := &config.Config{
		TorrentDirs:  []string{torrentsDir},
		LibraryDir:   jellyLibraryDir,
		UseShikimori: false, // Отключаем, чтобы не делать внешние сетевые запросы
		LanguageMapping: map[string]string{
			"RUS Sound": "ru",
			"RUS Subs":  "ru",
			"ENG Sound": "en",
			"ENG Subs":  "en",
			"Rus Dub":   "ru",
			"Rus Sub":   "ru",
		},
	}

	shows, err := Scan(cfg)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if len(shows) != 1 {
		t.Fatalf("expected 1 show, got %d", len(shows))
	}

	show := shows[0]
	if show.CleanedName != "Death Note" {
		t.Errorf("expected cleaned name 'Death Note', got '%s'", show.CleanedName)
	}

	plan := GeneratePlan(shows, cfg)
	if len(plan) != 5 {
		t.Fatalf("expected 5 operations in plan, got %d", len(plan))
	}

	expectedTargets := map[string]string{
		videoFile:    filepath.Join(jellyLibraryDir, "Death Note", "Season 01", "Death Note S01E01.mkv"),
		dub1File:     filepath.Join(jellyLibraryDir, "Death Note", "Season 01", "Death Note S01E01.СВ-Дубль.ru.mka"),
		dub2File:     filepath.Join(jellyLibraryDir, "Death Note", "Season 01", "Death Note S01E01.Мега-Аниме.ru.mka"),
		subFile:      filepath.Join(jellyLibraryDir, "Death Note", "Season 01", "Death Note S01E01.ru.ass"),
		subSignsFile: filepath.Join(jellyLibraryDir, "Death Note", "Season 01", "Death Note S01E01.Надписи.ru.ass"),
	}

	for _, op := range plan {
		expected, ok := expectedTargets[op.SourcePath]
		if !ok {
			t.Errorf("unexpected source path in plan: %s", op.SourcePath)
			continue
		}
		if op.TargetPath != expected {
			t.Errorf("expected target path for %s to be:\n%s\ngot:\n%s", op.SourcePath, expected, op.TargetPath)
		}
	}

	// Попробуем применить план
	err = ApplyPlan(plan, filepath.Join(tmpDir, "state.json"))
	if err != nil {
		t.Fatalf("ApplyPlan failed: %v", err)
	}

	// Проверим, что все 5 файлов созданы как симлинки
	for _, op := range plan {
		info, err := os.Lstat(op.TargetPath)
		if err != nil {
			t.Errorf("target file not found: %s", op.TargetPath)
			continue
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Errorf("target path is not a symlink: %s", op.TargetPath)
		}
	}
}

func TestCleanBrokenLinksWithMetadataAndEmptyDirs(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "jelly-an-li-linker-test-clean")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	torrentsDir := filepath.Join(tmpDir, "Torrents")
	jellyLibraryDir := filepath.Join(tmpDir, "Jellyfin")

	os.MkdirAll(torrentsDir, 0755)
	os.MkdirAll(jellyLibraryDir, 0755)

	// Исходная раздача
	showFolder := filepath.Join(torrentsDir, "Frieren [1080p]")
	os.MkdirAll(showFolder, 0755)
	videoFile := filepath.Join(showFolder, "Frieren - 01.mkv")
	os.WriteFile(videoFile, []byte("frieren episode 1"), 0644)

	cfg := &config.Config{
		TorrentDirs: []string{torrentsDir},
		LibraryDir:  jellyLibraryDir,
	}

	shows, err := Scan(cfg)
	if err != nil || len(shows) != 1 {
		t.Fatalf("Scan failed or expected 1 show: %v", err)
	}

	statePath := filepath.Join(tmpDir, "state.json")
	plan := GeneratePlan(shows, cfg)
	if err := ApplyPlan(plan, statePath); err != nil {
		t.Fatalf("ApplyPlan failed: %v", err)
	}

	// Имитируем создание файлов метаданных Jellyfin и системных мусорных файлов
	showTargetDir := filepath.Join(jellyLibraryDir, "Frieren")
	seasonTargetDir := filepath.Join(showTargetDir, "Season 01")

	os.WriteFile(filepath.Join(showTargetDir, "tvshow.nfo"), []byte("<xml>nfo</xml>"), 0644)
	os.WriteFile(filepath.Join(showTargetDir, "poster.jpg"), []byte("jpg data"), 0644)
	os.WriteFile(filepath.Join(showTargetDir, ".DS_Store"), []byte("ds store"), 0644)
	os.WriteFile(filepath.Join(seasonTargetDir, "season.nfo"), []byte("<xml>season</xml>"), 0644)

	// Теперь имитируем удаление аниме из торрентов
	os.RemoveAll(showFolder)

	// Запускаем очистку
	cleaned, err := CleanBrokenLinks(cfg, statePath)
	if err != nil {
		t.Fatalf("CleanBrokenLinks failed: %v", err)
	}

	if len(cleaned) == 0 {
		t.Errorf("expected cleaned items, got 0")
	}

	// Проверяем, что папка аниме Frieren полностью удалена из библиотеки Jellyfin
	if _, err := os.Stat(showTargetDir); !os.IsNotExist(err) {
		t.Errorf("expected show directory %s to be completely removed, but it still exists", showTargetDir)
	}
}

func TestMultiSeasonResolution(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "jelly-an-li-multi-season-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	torrentsDir := filepath.Join(tmpDir, "Torrents")
	jellyLibraryDir := filepath.Join(tmpDir, "Jellyfin")
	os.MkdirAll(torrentsDir, 0755)
	os.MkdirAll(jellyLibraryDir, 0755)

	folder1 := filepath.Join(torrentsDir, "[VCB-Studio] Mushoku Tensei - Isekai Ittara Honki Dasu [BDRip 1080p x264 FLAC]")
	folder2 := filepath.Join(torrentsDir, "Mushoku Tensei Isekai Ittara Honki Dasu II")
	os.MkdirAll(folder1, 0755)
	os.MkdirAll(folder2, 0755)

	os.WriteFile(filepath.Join(folder1, "Mushoku Tensei - 01.mkv"), []byte("s1 ep1"), 0644)
	os.WriteFile(filepath.Join(folder2, "Mushoku Tensei - 01.mkv"), []byte("s2 ep1"), 0644)

	cfg := &config.Config{
		TorrentDirs: []string{torrentsDir},
		LibraryDir:  jellyLibraryDir,
	}

	shows, err := Scan(cfg)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if len(shows) != 2 {
		t.Fatalf("expected 2 shows, got %d", len(shows))
	}

	plan := GeneratePlan(shows, cfg)
	if len(plan) != 2 {
		t.Fatalf("expected 2 link operations, got %d", len(plan))
	}

	target1 := plan[0].TargetPath
	target2 := plan[1].TargetPath

	// Оба должны быть в одной общей папке аниме
	dir1 := filepath.Dir(filepath.Dir(target1))
	dir2 := filepath.Dir(filepath.Dir(target2))

	if dir1 != dir2 {
		t.Errorf("expected parent show folders to match, got %s and %s", dir1, dir2)
	}

	// Один должен быть в Season 01, второй в Season 02
	seasonDir1 := filepath.Base(filepath.Dir(target1))
	seasonDir2 := filepath.Base(filepath.Dir(target2))

	if !(seasonDir1 == "Season 01" && seasonDir2 == "Season 02") && !(seasonDir1 == "Season 02" && seasonDir2 == "Season 01") {
		t.Errorf("expected Season 01 and Season 02 directories, got %s and %s", seasonDir1, seasonDir2)
	}
}

func TestMushokuTenseiSeason2(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "jelly-an-li-mushoku-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Изолируем тест от сети: инициализируем кэш Shikimori
	restore := providers.SetShikimoriCacheForTest(map[string]providers.CachedShikimoriInfo{
		"Mushoku Tensei": {
			Russian: "Реинкарнация безработного",
			Season:  1,
		},
		"Mushoku Tensei II": {
			Russian: "Реинкарнация безработного",
			Season:  2,
		},
	})
	defer restore()

	torrentsDir := filepath.Join(tmpDir, "Torrents")
	jellyLibraryDir := filepath.Join(tmpDir, "Jellyfin")
	folder1 := filepath.Join(torrentsDir, "Mushoku Tensei Isekai Ittara Honki Dasu")
	folder2 := filepath.Join(torrentsDir, "Mushoku Tensei Isekai Ittara Honki Dasu II")
	os.MkdirAll(folder1, 0755)
	os.MkdirAll(folder2, 0755)

	os.WriteFile(filepath.Join(folder1, "Mushoku Tensei - 01.mkv"), []byte("s1ep1"), 0644)
	os.WriteFile(filepath.Join(folder2, "Mushoku Tensei II - 01.mkv"), []byte("s2ep1"), 0644)

	cfg := &config.Config{
		TorrentDirs:  []string{torrentsDir},
		LibraryDir:   jellyLibraryDir,
		UseShikimori: true,
	}

	shows, err := Scan(cfg)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	plan := GeneratePlan(shows, cfg)
	if len(plan) != 2 {
		t.Fatalf("expected 2 link operations, got %d", len(plan))
	}

	seasons := make(map[string]string)
	for _, op := range plan {
		srcFolder := filepath.Base(filepath.Dir(op.SourcePath))
		targetSeason := filepath.Base(filepath.Dir(op.TargetPath))
		seasons[srcFolder] = targetSeason
	}

	if seasons["Mushoku Tensei Isekai Ittara Honki Dasu"] != "Season 01" {
		t.Errorf("expected Season 01 for folder1, got %s", seasons["Mushoku Tensei Isekai Ittara Honki Dasu"])
	}
	if seasons["Mushoku Tensei Isekai Ittara Honki Dasu II"] != "Season 02" {
		t.Errorf("expected Season 02 for folder2, got %s", seasons["Mushoku Tensei Isekai Ittara Honki Dasu II"])
	}
}

func TestSlimeSeason2Part2(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "jelly-an-li-slime-part2-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	torrentsDir := filepath.Join(tmpDir, "Torrents")
	jellyLibraryDir := filepath.Join(tmpDir, "Jellyfin")
	os.MkdirAll(torrentsDir, 0755)
	os.MkdirAll(jellyLibraryDir, 0755)

	folder1 := filepath.Join(torrentsDir, "[SOFCJ-Raws] Tensei Shitara Slime Datta Ken - S02 [WEB-DL KP 1080p]")
	folder2 := filepath.Join(torrentsDir, "[SOFCJ-Raws] Tensei Shitara Slime Datta Ken - S02 Part 2 [WEB-DL KP 1080p]")
	os.MkdirAll(folder1, 0755)
	os.MkdirAll(folder2, 0755)

	for i := 1; i <= 12; i++ {
		f1 := filepath.Join(folder1, fmt.Sprintf("[SOFCJ-Raws] Tensei Shitara Slime Datta Ken - %02d (1080p).mkv", i))
		f2 := filepath.Join(folder2, fmt.Sprintf("[SOFCJ-Raws] Tensei Shitara Slime Datta Ken - %02d (1080p).mkv", i))
		os.WriteFile(f1, []byte(fmt.Sprintf("part1 ep%d", i)), 0644)
		os.WriteFile(f2, []byte(fmt.Sprintf("part2 ep%d", i)), 0644)
	}

	cfg := &config.Config{
		TorrentDirs: []string{torrentsDir},
		LibraryDir:  jellyLibraryDir,
	}

	shows, err := Scan(cfg)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if len(shows) != 2 {
		t.Fatalf("expected 2 shows, got %d", len(shows))
	}

	plan := GeneratePlan(shows, cfg)
	if len(plan) != 24 {
		t.Fatalf("expected 24 link operations in plan, got %d", len(plan))
	}

	for _, op := range plan {
		base := filepath.Base(op.SourcePath)
		seasonDir := filepath.Base(filepath.Dir(op.TargetPath))
		if seasonDir != "Season 02" {
			t.Errorf("expected target season dir Season 02, got %s", seasonDir)
		}

		targetFile := filepath.Base(op.TargetPath)
		var epNum int
		fmt.Sscanf(targetFile, "Tensei Shitara Slime Datta Ken S02E%d.mkv", &epNum)

		if strings.Contains(op.SourcePath, "S02 Part 2") {
			if epNum < 13 || epNum > 24 {
				t.Errorf("Part 2 file %s mapped to invalid episode %d (target: %s)", base, epNum, targetFile)
			}
		} else {
			if epNum < 1 || epNum > 12 {
				t.Errorf("Part 1 file %s mapped to invalid episode %d (target: %s)", base, epNum, targetFile)
			}
		}
	}

	err = ApplyPlan(plan, filepath.Join(tmpDir, "state.json"))
	if err != nil {
		t.Fatalf("ApplyPlan failed: %v", err)
	}

	season02Dir := filepath.Join(jellyLibraryDir, "Tensei Shitara Slime Datta Ken", "Season 02")
	entries, err := os.ReadDir(season02Dir)
	if err != nil {
		t.Fatalf("failed to read Season 02 dir: %v", err)
	}

	if len(entries) != 24 {
		t.Errorf("expected 24 files in Season 02, got %d", len(entries))
	}
}

func TestKimetsuNoYaibaSeasons(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "jelly-an-li-kimetsu-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Изолируем тест от сети: инициализируем кэш Shikimori
	restore := providers.SetShikimoriCacheForTest(map[string]providers.CachedShikimoriInfo{
		"Kimetsu no Yaiba": {
			Russian: "Клинок, рассекающий демонов",
			Season:  1,
		},
		"Kimetsu no Yaiba Mugen Ressha Hen": {
			Russian: "Клинок, рассекающий демонов",
			Season:  2,
		},
		"Kimetsu no Yaiba Yuukaku hen": {
			Russian: "Клинок, рассекающий демонов",
			Season:  3,
		},
		"Kimetsu no Yaiba Katanakaji no Sato hen": {
			Russian: "Клинок, рассекающий демонов",
			Season:  4,
		},
		"Kimetsu no Yaiba Hashira Geiko hen": {
			Russian: "Клинок, рассекающий демонов",
			Season:  5,
		},
	})
	defer restore()

	torrentsDir := filepath.Join(tmpDir, "Torrents")
	jellyLibraryDir := filepath.Join(tmpDir, "Jellyfin")
	os.MkdirAll(torrentsDir, 0755)
	os.MkdirAll(jellyLibraryDir, 0755)

	folders := map[string][]string{
		"[Kawaiika-Raws] (2019) Kimetsu no Yaiba [BDRip 1920x1080 HEVC FLAC]": {
			"[Kawaiika-Raws] Kimetsu no Yaiba 01 [BDRip 1920x1080 HEVC FLAC].mkv",
		},
		"[VCB-Studio] Kimetsu no Yaiba - Mugen Ressha-Hen [BDRip 1080p x265 FLAC]": {
			"[VCB-Studio] Kimetsu no Yaiba - Mugen Ressha-Hen - 01 [BDRip 1080p x265 FLAC].mkv",
		},
		"[VCB-Studio] Kimetsu no Yaiba - Yuukaku-hen [BDRip 1080p x265 FLAC]": {
			"[VCB-Studio] Kimetsu no Yaiba - Yuukaku-hen - 01 [BDRip 1080p x265 FLAC].mkv",
		},
		"[Moozzi2] Kimetsu no Yaiba - Katanakaji no Sato-hen [BDRip 1080p x265 FLAC]": {
			"[Moozzi2] Kimetsu no Yaiba - Katanakaji no Sato-hen - 01 [BDRip 1080p x265 FLAC].mkv",
		},
		"[Moozzi2] Kimetsu no Yaiba - Hashira Geiko-hen [BDRip 1080p x265 FLAC]": {
			"[Moozzi2] Kimetsu no Yaiba - Hashira Geiko-hen - 01 [BDRip 1080p x265 FLAC].mkv",
		},
	}

	for fName, files := range folders {
		fPath := filepath.Join(torrentsDir, fName)
		os.MkdirAll(fPath, 0755)
		for _, file := range files {
			os.WriteFile(filepath.Join(fPath, file), []byte("test content"), 0644)
		}
	}

	cfg := &config.Config{
		TorrentDirs:  []string{torrentsDir},
		LibraryDir:   jellyLibraryDir,
		UseShikimori: true,
	}

	shows, err := Scan(cfg)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if len(shows) != 5 {
		t.Fatalf("expected 5 shows scanned, got %d", len(shows))
	}

	plan := GeneratePlan(shows, cfg)
	if len(plan) != 5 {
		t.Fatalf("expected 5 link operations, got %d", len(plan))
	}

	expectedSeasons := map[string]string{
		"[Kawaiika-Raws] (2019) Kimetsu no Yaiba [BDRip 1920x1080 HEVC FLAC]":          "Season 01",
		"[VCB-Studio] Kimetsu no Yaiba - Mugen Ressha-Hen [BDRip 1080p x265 FLAC]":     "Season 02",
		"[VCB-Studio] Kimetsu no Yaiba - Yuukaku-hen [BDRip 1080p x265 FLAC]":         "Season 03",
		"[Moozzi2] Kimetsu no Yaiba - Katanakaji no Sato-hen [BDRip 1080p x265 FLAC]": "Season 04",
		"[Moozzi2] Kimetsu no Yaiba - Hashira Geiko-hen [BDRip 1080p x265 FLAC]":      "Season 05",
	}

	for _, op := range plan {
		parentFolder := filepath.Base(filepath.Dir(op.SourcePath))
		expectedSeason := expectedSeasons[parentFolder]
		actualSeason := filepath.Base(filepath.Dir(op.TargetPath))
		if actualSeason != expectedSeason {
			t.Errorf("for folder %s expected %s, got %s (target path: %s)", parentFolder, expectedSeason, actualSeason, op.TargetPath)
		}
		targetFile := filepath.Base(op.TargetPath)
		if !strings.HasPrefix(targetFile, "Kimetsu no Yaiba S") {
			t.Errorf("expected English target filename prefix 'Kimetsu no Yaiba S', got %s", targetFile)
		}
		showDir := filepath.Base(filepath.Dir(filepath.Dir(op.TargetPath)))
		if showDir != "Клинок, рассекающий демонов" {
			t.Errorf("expected Russian show folder 'Клинок, рассекающий демонов', got %s", showDir)
		}
	}
}

func TestKimetsuNoYaibaEndingConflict(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "jelly-an-li-conflict-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	torrentsDir := filepath.Join(tmpDir, "Torrents")
	jellyLibraryDir := filepath.Join(tmpDir, "Jellyfin")
	os.MkdirAll(torrentsDir, 0755)
	os.MkdirAll(jellyLibraryDir, 0755)

	fPath := filepath.Join(torrentsDir, "[Kawaiika-Raws] (2019) Kimetsu no Yaiba [BDRip 1920x1080 HEVC FLAC]")
	os.MkdirAll(fPath, 0755)

	ep19File := filepath.Join(fPath, "[Kawaiika-Raws] Kimetsu no Yaiba 19 [BDRip 1920x1080 HEVC FLAC].mkv")
	ed19File := filepath.Join(fPath, "[Kawaiika-Raws] Kimetsu no Yaiba ED_e19 [BDRip 1920x1080 HEVC FLAC].mkv")

	os.WriteFile(ep19File, []byte("ep 19 video"), 0644)
	os.WriteFile(ed19File, []byte("ed 19 video"), 0644)

	cfg := &config.Config{
		TorrentDirs:  []string{torrentsDir},
		LibraryDir:   jellyLibraryDir,
		UseShikimori: false,
	}

	shows, err := Scan(cfg)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	plan := GeneratePlan(shows, cfg)
	if len(plan) != 1 {
		t.Fatalf("expected 1 link operation (ED file ignored), got %d", len(plan))
	}

	if plan[0].SourcePath != ep19File || !strings.HasSuffix(plan[0].TargetPath, "S01E19.mkv") {
		t.Errorf("expected ep19 to be mapped to S01E19.mkv, got: %s", plan[0].TargetPath)
	}
}

func TestLongSeriesEpisodePadding(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "jelly-an-li-long-series-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	torrentsDir := filepath.Join(tmpDir, "Torrents")
	jellyLibraryDir := filepath.Join(tmpDir, "Jellyfin")
	os.MkdirAll(torrentsDir, 0755)
	os.MkdirAll(jellyLibraryDir, 0755)

	fPath := filepath.Join(torrentsDir, "[Judas] Black Clover (170ep)")
	os.MkdirAll(fPath, 0755)

	ep1 := filepath.Join(fPath, "Black_Clover_-_001.mkv")
	ep100 := filepath.Join(fPath, "Black_Clover_-_100.mkv")
	ep170 := filepath.Join(fPath, "Black_Clover_-_170.mkv")

	os.WriteFile(ep1, []byte("ep 1"), 0644)
	os.WriteFile(ep100, []byte("ep 100"), 0644)
	os.WriteFile(ep170, []byte("ep 170"), 0644)

	cfg := &config.Config{
		TorrentDirs:  []string{torrentsDir},
		LibraryDir:   jellyLibraryDir,
		UseShikimori: false,
	}

	shows, err := Scan(cfg)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	plan := GeneratePlan(shows, cfg)
	if len(plan) != 3 {
		t.Fatalf("expected 3 episodes in plan, got %d", len(plan))
	}

	for _, op := range plan {
		if op.SourcePath == ep1 && !strings.HasSuffix(op.TargetPath, "S01E001.mkv") {
			t.Errorf("expected S01E001.mkv for ep1, got: %s", op.TargetPath)
		}
		if op.SourcePath == ep100 && !strings.HasSuffix(op.TargetPath, "S01E100.mkv") {
			t.Errorf("expected S01E100.mkv for ep100, got: %s", op.TargetPath)
		}
		if op.SourcePath == ep170 && !strings.HasSuffix(op.TargetPath, "S01E170.mkv") {
			t.Errorf("expected S01E170.mkv for ep170, got: %s", op.TargetPath)
		}
	}
}

func TestCleanObsoleteLinksNotInPlan(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "jelly-an-li-obsolete-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	torrentsDir := filepath.Join(tmpDir, "Torrents")
	jellyLibraryDir := filepath.Join(tmpDir, "Jellyfin")
	os.MkdirAll(torrentsDir, 0755)
	os.MkdirAll(jellyLibraryDir, 0755)

	// Торрент
	showTorrent := filepath.Join(torrentsDir, "Anime Show")
	os.MkdirAll(showTorrent, 0755)
	sourceValid := filepath.Join(showTorrent, "Show - 01.mkv")
	sourceObsolete := filepath.Join(showTorrent, "Show - OP 01.mkv")
	os.WriteFile(sourceValid, []byte("ep1"), 0644)
	os.WriteFile(sourceObsolete, []byte("op1"), 0644)

	// В библиотеке создаем ссылки
	showLib := filepath.Join(jellyLibraryDir, "Anime Show", "Season 01")
	os.MkdirAll(showLib, 0755)
	targetValid := filepath.Join(showLib, "Anime Show S01E01.mkv")
	targetObsolete := filepath.Join(showLib, "Anime Show - OP 01.mkv")
	os.Symlink(sourceValid, targetValid)
	os.Symlink(sourceObsolete, targetObsolete)

	cfg := &config.Config{
		TorrentDirs:  []string{torrentsDir},
		LibraryDir:   jellyLibraryDir,
		UseShikimori: false,
	}

	shows, err := Scan(cfg)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	plan := GeneratePlan(shows, cfg)
	// В плане должна быть только серия 01 (опенинг отфильтрован)
	if len(plan) != 1 {
		t.Fatalf("expected 1 episode in plan, got %d", len(plan))
	}

	cleaned, err := CleanBrokenLinks(cfg, "", plan)
	if err != nil {
		t.Fatalf("CleanBrokenLinks failed: %v", err)
	}

	if len(cleaned) != 1 {
		t.Fatalf("expected 1 obsolete link cleaned, got %d: %v", len(cleaned), cleaned)
	}

	// Проверим, что файл опенинга удален, а валидная серия осталась
	if _, err := os.Lstat(targetObsolete); !os.IsNotExist(err) {
		t.Errorf("obsolete link was not removed: %s", targetObsolete)
	}
	if _, err := os.Lstat(targetValid); err != nil {
		t.Errorf("valid link was removed: %v", err)
	}
}

func TestSousouNoFrierenSeason2(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "jelly-an-li-frieren-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	torrentsDir := filepath.Join(tmpDir, "Torrents")
	jellyLibraryDir := filepath.Join(tmpDir, "Jellyfin")
	os.MkdirAll(torrentsDir, 0755)
	os.MkdirAll(jellyLibraryDir, 0755)

	folder1 := filepath.Join(torrentsDir, "Sousou no Frieren")
	folder2 := filepath.Join(torrentsDir, "Sousou no Frieren TV-2 [anidb2-18886]")
	os.MkdirAll(folder1, 0755)
	os.MkdirAll(folder2, 0755)

	os.WriteFile(filepath.Join(folder1, "Sousou no Frieren 01.mkv"), []byte("s1 ep1"), 0644)
	os.WriteFile(filepath.Join(folder2, "Sousou no Frieren TV-2 01.mkv"), []byte("s2 ep1"), 0644)
	os.WriteFile(filepath.Join(folder2, "Sousou no Frieren TV-2 10.mkv"), []byte("s2 ep10"), 0644)

	// Инициализируем изолированный кэш Shikimori
	restore := providers.SetShikimoriCacheForTest(map[string]providers.CachedShikimoriInfo{
		"Sousou no Frieren": {
			Russian: "Провожающая в последний путь Фрирен",
			Season:  1,
		},
		"Sousou no Frieren 2": {
			Russian: "Провожающая в последний путь Фрирен",
			Season:  2,
		},
	})
	defer restore()

	cfg := &config.Config{
		TorrentDirs:  []string{torrentsDir},
		LibraryDir:   jellyLibraryDir,
		UseShikimori: true,
	}

	shows, err := Scan(cfg)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if len(shows) != 2 {
		t.Fatalf("expected 2 shows, got %d", len(shows))
	}

	for _, show := range shows {
		if show.OriginalName == "Sousou no Frieren TV-2 [anidb2-18886]" {
			if show.Season != 2 {
				t.Errorf("expected Season 2 for TV-2 folder, got %d", show.Season)
			}
			if show.RussianName != "Провожающая в последний путь Фрирен" {
				t.Errorf("expected Russian name 'Провожающая в последний путь Фрирен', got '%s'", show.RussianName)
			}
		}
	}

	plan := GeneratePlan(shows, cfg)
	if len(plan) != 3 {
		t.Fatalf("expected 3 link operations in plan, got %d", len(plan))
	}

	for _, op := range plan {
		targetFile := filepath.Base(op.TargetPath)
		seasonDir := filepath.Base(filepath.Dir(op.TargetPath))
		showDir := filepath.Base(filepath.Dir(filepath.Dir(op.TargetPath)))

		if showDir != "Провожающая в последний путь Фрирен" {
			t.Errorf("expected show directory 'Провожающая в последний путь Фрирен', got '%s'", showDir)
		}

		if strings.Contains(op.SourcePath, "TV-2") {
			if seasonDir != "Season 02" {
				t.Errorf("expected Season 02 for TV-2 file, got %s (target: %s)", seasonDir, op.TargetPath)
			}
			if !strings.HasPrefix(targetFile, "Sousou no Frieren S02E") {
				t.Errorf("expected target filename to start with 'Sousou no Frieren S02E', got %s", targetFile)
			}
		} else {
			if seasonDir != "Season 01" {
				t.Errorf("expected Season 01 for S1 file, got %s (target: %s)", seasonDir, op.TargetPath)
			}
			if !strings.HasPrefix(targetFile, "Sousou no Frieren S01E") {
				t.Errorf("expected target filename to start with 'Sousou no Frieren S01E', got %s", targetFile)
			}
		}
	}
}

func TestFolderNamingModes(t *testing.T) {
	shows := []*parser.AnimeShow{
		{
			CleanedName: "Sousou no Frieren",
			RussianName: "Провожающая в последний путь Фрирен",
			RomajiName:  "Sousou no Frieren",
			Season:      1,
			Files: []*parser.EpisodeFile{
				{
					SourcePath: "/torrents/Sousou no Frieren 01.mkv",
					SeasonNum:  1,
					EpisodeNum: 1,
					Type:       parser.TypeVideo,
				},
			},
		},
	}

	// 1. Russian (default)
	cfgRussian := &config.Config{
		LibraryDir:       "/media",
		FolderNamingMode: "russian",
	}
	planRu := GeneratePlan(shows, cfgRussian)
	if len(planRu) != 1 {
		t.Fatalf("expected 1 plan item, got %d", len(planRu))
	}
	if filepath.Base(filepath.Dir(filepath.Dir(planRu[0].TargetPath))) != "Провожающая в последний путь Фрирен" {
		t.Errorf("expected Russian folder name, got '%s'", filepath.Dir(filepath.Dir(planRu[0].TargetPath)))
	}

	// 2. Romaji / English
	cfgRomaji := &config.Config{
		LibraryDir:       "/media",
		FolderNamingMode: "romaji",
	}
	planRomaji := GeneratePlan(shows, cfgRomaji)
	if len(planRomaji) != 1 {
		t.Fatalf("expected 1 plan item, got %d", len(planRomaji))
	}
	if filepath.Base(filepath.Dir(filepath.Dir(planRomaji[0].TargetPath))) != "Sousou no Frieren" {
		t.Errorf("expected Romaji folder name, got '%s'", filepath.Dir(filepath.Dir(planRomaji[0].TargetPath)))
	}

	// 3. Original
	showsOrig := []*parser.AnimeShow{
		{
			CleanedName: "Custom Raw Release Name",
			RussianName: "Русское Имя",
			RomajiName:  "Official Romaji Name",
			Season:      1,
			Files: []*parser.EpisodeFile{
				{
					SourcePath: "/torrents/ep01.mkv",
					SeasonNum:  1,
					EpisodeNum: 1,
					Type:       parser.TypeVideo,
				},
			},
		},
	}
	cfgOrig := &config.Config{
		LibraryDir:       "/media",
		FolderNamingMode: "original",
	}
	planOrig := GeneratePlan(showsOrig, cfgOrig)
	if len(planOrig) != 1 {
		t.Fatalf("expected 1 plan item, got %d", len(planOrig))
	}
	if filepath.Base(filepath.Dir(filepath.Dir(planOrig[0].TargetPath))) != "Custom Raw Release Name" {
		t.Errorf("expected Original folder name, got '%s'", filepath.Dir(filepath.Dir(planOrig[0].TargetPath)))
	}
}

func TestProvidersFallbackChain(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "jelly-an-li-fallback-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	torrentsDir := filepath.Join(tmpDir, "Torrents")
	jellyLibraryDir := filepath.Join(tmpDir, "Jellyfin")
	os.MkdirAll(torrentsDir, 0755)
	os.MkdirAll(jellyLibraryDir, 0755)

	folder := filepath.Join(torrentsDir, "Solo Leveling - 01 [1080p]")
	os.MkdirAll(folder, 0755)
	os.WriteFile(filepath.Join(folder, "Solo Leveling - 01.mkv"), []byte("video"), 0644)

	// Настраиваем цепочку: сначала anilist (который вернет ромадзи), затем shikimori
	cfg := &config.Config{
		TorrentDirs:       []string{torrentsDir},
		LibraryDir:        jellyLibraryDir,
		MetadataProviders: []string{"anilist", "shikimori"},
		FolderNamingMode:  "romaji",
	}

	shows, err := Scan(cfg)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(shows) != 1 {
		t.Fatalf("expected 1 show, got %d", len(shows))
	}

	plan := GeneratePlan(shows, cfg)
	if len(plan) != 1 {
		t.Fatalf("expected 1 link operation, got %d", len(plan))
	}
}

func TestBlackCloverMovieWithSeries(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "jelly-an-li-black-clover-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	restore := providers.SetShikimoriCacheForTest(map[string]providers.CachedShikimoriInfo{
		"Black Clover": {
			Russian:   "Чёрный клевер",
			Romaji:    "Black Clover",
			Season:    1,
			IsMovie:   false,
			IsSpecial: false,
		},
		"Black Clover Mahoutei no Ken": {
			Russian:   "Чёрный клевер: Меч короля магов",
			Romaji:    "Black Clover: Mahou Tei no Ken",
			Season:    1,
			IsMovie:   true,
			IsSpecial: false,
		},
	})
	defer restore()

	torrentsDir := filepath.Join(tmpDir, "Torrents")
	jellyLibraryDir := filepath.Join(tmpDir, "Jellyfin")
	os.MkdirAll(torrentsDir, 0755)
	os.MkdirAll(jellyLibraryDir, 0755)

	// 1. Папка с сериалом Чёрный клевер
	seriesFolder := filepath.Join(torrentsDir, "[Judas]_Black_Clover_(170ep+Extras)_[BDrip_1080p][HEVC_x265_10bit][RUS+JP+ENG][Multi-Subs]")
	os.MkdirAll(seriesFolder, 0755)
	ep1 := filepath.Join(seriesFolder, "Black_Clover_-_001.mkv")
	ep170 := filepath.Join(seriesFolder, "Black_Clover_-_170.mkv")
	os.WriteFile(ep1, []byte("ep 1"), 0644)
	os.WriteFile(ep170, []byte("ep 170"), 0644)

	// 2. Одиночный файл фильма прямо в корне /Torrents
	movieFile := filepath.Join(torrentsDir, "Eiga Black Clover Mahoutei no Ken (BD 1920x1080 x265-10Bit DTS Flac).mkv")
	os.WriteFile(movieFile, []byte("movie content"), 0644)

	cfg := &config.Config{
		TorrentDirs:  []string{torrentsDir},
		LibraryDir:   jellyLibraryDir,
		UseShikimori: true,
	}

	shows, err := Scan(cfg)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if len(shows) != 2 {
		t.Fatalf("expected 2 shows scanned (1 series + 1 standalone movie), got %d", len(shows))
	}

	plan := GeneratePlan(shows, cfg)
	if len(plan) != 3 {
		t.Fatalf("expected 3 operations in plan, got %d", len(plan))
	}

	for _, op := range plan {
		if op.SourcePath == movieFile {
			// Должен быть в Чёрный клевер / Season 00 / ...
			expectedDir := filepath.Join(jellyLibraryDir, "Чёрный клевер", "Season 00")
			actualDir := filepath.Dir(op.TargetPath)
			if actualDir != expectedDir {
				t.Errorf("expected movie dir '%s', got '%s'", expectedDir, actualDir)
			}
			expectedFile := "Black Clover S00E01 - Black Clover - Mahou Tei no Ken.mkv"
			actualFile := filepath.Base(op.TargetPath)
			if actualFile != expectedFile {
				t.Errorf("expected movie file '%s', got '%s'", expectedFile, actualFile)
			}
		} else {
			expectedDir := filepath.Join(jellyLibraryDir, "Чёрный клевер", "Season 01")
			actualDir := filepath.Dir(op.TargetPath)
			if actualDir != expectedDir {
				t.Errorf("expected series dir '%s', got '%s'", expectedDir, actualDir)
			}
		}
	}

	err = ApplyPlan(plan, filepath.Join(tmpDir, "state.json"))
	if err != nil {
		t.Fatalf("ApplyPlan failed: %v", err)
	}

	// Проверяем, что файл фильма создан в Season 00/
	movieTarget := filepath.Join(jellyLibraryDir, "Чёрный клевер", "Season 00", "Black Clover S00E01 - Black Clover - Mahou Tei no Ken.mkv")
	if info, err := os.Lstat(movieTarget); err != nil {
		t.Errorf("movie link not found at %s: %v", movieTarget, err)
	} else if info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("movie target is not a symlink: %s", movieTarget)
	}
}

func TestKimetsuMugenResshaMovieStandalone(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "jelly-an-li-kimetsu-movie-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	restore := providers.SetShikimoriCacheForTest(map[string]providers.CachedShikimoriInfo{
		"Kimetsu no Yaiba": {
			Russian:   "Клинок, рассекающий демонов",
			Romaji:    "Kimetsu no Yaiba",
			Season:    1,
			IsMovie:   false,
			IsSpecial: false,
		},
		"Kimetsu no Yaiba Mugen Ressha Hen": {
			Russian:   "Клинок, рассекающий демонов: Поезд «Бесконечный»",
			Romaji:    "Kimetsu no Yaiba: Mugen Ressha-hen",
			Season:    1,
			IsMovie:   true,
			IsSpecial: false,
		},
	})
	defer restore()

	torrentsDir := filepath.Join(tmpDir, "Torrents")
	jellyLibraryDir := filepath.Join(tmpDir, "Jellyfin")
	os.MkdirAll(torrentsDir, 0755)
	os.MkdirAll(jellyLibraryDir, 0755)

	// 1. Папка с 1 сезоном сериала Клинок
	seriesFolder := filepath.Join(torrentsDir, "[Kawaiika-Raws] (2019) Kimetsu no Yaiba [BDRip 1920x1080 HEVC FLAC]")
	os.MkdirAll(seriesFolder, 0755)
	ep1 := filepath.Join(seriesFolder, "[Kawaiika-Raws] Kimetsu no Yaiba 01.mkv")
	os.WriteFile(ep1, []byte("ep 1"), 0644)

	// 2. Одиночный файл фильма в корне /Torrents
	movieFile := filepath.Join(torrentsDir, "Gekijouban.Kimetsu.no.Yaiba.Mugen.Ressha.Hen.2020.BDRip.1080p.JanNYy.mkv")
	os.WriteFile(movieFile, []byte("movie content"), 0644)

	cfg := &config.Config{
		TorrentDirs:  []string{torrentsDir},
		LibraryDir:   jellyLibraryDir,
		UseShikimori: true,
	}

	shows, err := Scan(cfg)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if len(shows) != 2 {
		t.Fatalf("expected 2 shows scanned (1 series + 1 standalone movie), got %d", len(shows))
	}

	plan := GeneratePlan(shows, cfg)
	if len(plan) != 2 {
		t.Fatalf("expected 2 operations in plan, got %d", len(plan))
	}

	for _, op := range plan {
		if op.SourcePath == movieFile {
			expectedDir := filepath.Join(jellyLibraryDir, "Клинок, рассекающий демонов", "Season 00")
			actualDir := filepath.Dir(op.TargetPath)
			if actualDir != expectedDir {
				t.Errorf("expected movie dir '%s', got '%s'", expectedDir, actualDir)
			}
			expectedFile := "Kimetsu no Yaiba S00E01 - Kimetsu no Yaiba - Mugen Ressha-hen.mkv"
			actualFile := filepath.Base(op.TargetPath)
			if actualFile != expectedFile {
				t.Errorf("expected movie file '%s', got '%s'", expectedFile, actualFile)
			}
		} else {
			expectedDir := filepath.Join(jellyLibraryDir, "Клинок, рассекающий демонов", "Season 01")
			actualDir := filepath.Dir(op.TargetPath)
			if actualDir != expectedDir {
				t.Errorf("expected series dir '%s', got '%s'", expectedDir, actualDir)
			}
		}
	}

	err = ApplyPlan(plan, filepath.Join(tmpDir, "state.json"))
	if err != nil {
		t.Fatalf("ApplyPlan failed: %v", err)
	}

	movieTarget := filepath.Join(jellyLibraryDir, "Клинок, рассекающий демонов", "Season 00", "Kimetsu no Yaiba S00E01 - Kimetsu no Yaiba - Mugen Ressha-hen.mkv")
	if info, err := os.Lstat(movieTarget); err != nil {
		t.Errorf("movie link not found at %s: %v", movieTarget, err)
	} else if info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("movie target is not a symlink: %s", movieTarget)
	}
}

func TestKimetsuMovieOnlyWithoutAnyPreviousSeasons(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "jelly-an-li-kimetsu-only-movie-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	restore := providers.SetShikimoriCacheForTest(map[string]providers.CachedShikimoriInfo{
		"Kimetsu no Yaiba Mugen Ressha Hen": {
			Russian:   "Клинок, рассекающий демонов",
			Romaji:    "Kimetsu no Yaiba: Mugen Ressha-hen",
			Season:    1,
			IsMovie:   true,
			IsSpecial: false,
		},
	})
	defer restore()

	torrentsDir := filepath.Join(tmpDir, "Torrents")
	jellyLibraryDir := filepath.Join(tmpDir, "Jellyfin")
	os.MkdirAll(torrentsDir, 0755)
	os.MkdirAll(jellyLibraryDir, 0755)

	// В раздачах ТОЛЬКО один файл фильма, никаких папок и сезонов нет
	movieFile := filepath.Join(torrentsDir, "Gekijouban.Kimetsu.no.Yaiba.Mugen.Ressha.Hen.2020.BDRip.1080p.JanNYy.mkv")
	os.WriteFile(movieFile, []byte("movie content"), 0644)

	cfg := &config.Config{
		TorrentDirs:  []string{torrentsDir},
		LibraryDir:   jellyLibraryDir,
		UseShikimori: true,
	}

	shows, err := Scan(cfg)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if len(shows) != 1 {
		t.Fatalf("expected 1 show scanned, got %d", len(shows))
	}

	plan := GeneratePlan(shows, cfg)
	if len(plan) != 1 {
		t.Fatalf("expected 1 operation in plan, got %d", len(plan))
	}

	expectedDir := filepath.Join(jellyLibraryDir, "Клинок, рассекающий демонов", "Season 00")
	if filepath.Dir(plan[0].TargetPath) != expectedDir {
		t.Errorf("expected target dir '%s', got '%s'", expectedDir, filepath.Dir(plan[0].TargetPath))
	}

	expectedFile := "Kimetsu no Yaiba S00E01 - Kimetsu no Yaiba - Mugen Ressha-hen.mkv"
	if filepath.Base(plan[0].TargetPath) != expectedFile {
		t.Errorf("expected target file '%s', got '%s'", expectedFile, filepath.Base(plan[0].TargetPath))
	}

	if plan[0].NfoContent == "" {
		t.Errorf("expected NfoContent to be generated for movie")
	}

	err = ApplyPlan(plan, filepath.Join(tmpDir, "state.json"))
	if err != nil {
		t.Fatalf("ApplyPlan failed: %v", err)
	}

	movieTarget := filepath.Join(expectedDir, expectedFile)
	if info, err := os.Lstat(movieTarget); err != nil {
		t.Errorf("movie link not found at %s: %v", movieTarget, err)
	} else if info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("movie target is not a symlink: %s", movieTarget)
	}

	nfoTarget := filepath.Join(expectedDir, "Kimetsu no Yaiba S00E01 - Kimetsu no Yaiba - Mugen Ressha-hen.nfo")
	if data, err := os.ReadFile(nfoTarget); err != nil {
		t.Errorf("nfo file not created at %s: %v", nfoTarget, err)
	} else {
		if !strings.Contains(string(data), "<title>Клинок, рассекающий демонов</title>") {
			t.Errorf("nfo missing expected Russian title, got: %s", string(data))
		}
	}
}

func TestBlackCloverRussianDejzDubMovie(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "jelly-an-li-bc-dejzdub-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	restore := providers.SetShikimoriCacheForTest(map[string]providers.CachedShikimoriInfo{
		"Черный Клевер Меч Короля Магов": {
			ID:        50410,
			Russian:   "Чёрный клевер",
			MovieRu:   "Чёрный клевер: Меч короля магов",
			Romaji:    "Black Clover: Mahou Tei no Ken",
			Season:    1,
			IsMovie:   true,
			IsSpecial: false,
		},
	})
	defer restore()

	torrentsDir := filepath.Join(tmpDir, "Torrents")
	jellyLibraryDir := filepath.Join(tmpDir, "Jellyfin")
	os.MkdirAll(torrentsDir, 0755)
	os.MkdirAll(jellyLibraryDir, 0755)

	movieFile := filepath.Join(torrentsDir, "Черный Клевер Меч Короля Магов 1080 (DejzDub)mp4.mp4")
	os.WriteFile(movieFile, []byte("mp4 content"), 0644)

	cfg := &config.Config{
		TorrentDirs:  []string{torrentsDir},
		LibraryDir:   jellyLibraryDir,
		UseShikimori: true,
	}

	shows, err := Scan(cfg)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if len(shows) != 1 {
		t.Fatalf("expected 1 show scanned, got %d", len(shows))
	}

	show := shows[0]
	if !show.IsMovie {
		t.Errorf("expected show.IsMovie to be true")
	}

	plan := GeneratePlan(shows, cfg)
	if len(plan) != 1 {
		t.Fatalf("expected 1 operation in plan, got %d", len(plan))
	}

	expectedDir := filepath.Join(jellyLibraryDir, "Чёрный клевер", "Season 00")
	if filepath.Dir(plan[0].TargetPath) != expectedDir {
		t.Errorf("expected target dir '%s', got '%s'", expectedDir, filepath.Dir(plan[0].TargetPath))
	}

	expectedFile := "Black Clover S00E01 - Black Clover - Mahou Tei no Ken.mp4"
	if filepath.Base(plan[0].TargetPath) != expectedFile {
		t.Errorf("expected target file '%s', got '%s'", expectedFile, filepath.Base(plan[0].TargetPath))
	}

	err = ApplyPlan(plan, filepath.Join(tmpDir, "state.json"))
	if err != nil {
		t.Fatalf("ApplyPlan failed: %v", err)
	}

	movieTarget := filepath.Join(expectedDir, expectedFile)
	if info, err := os.Lstat(movieTarget); err != nil {
		t.Errorf("movie link not found at %s: %v", movieTarget, err)
	} else if info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("movie target is not a symlink: %s", movieTarget)
	}

	nfoTarget := filepath.Join(expectedDir, "Black Clover S00E01 - Black Clover - Mahou Tei no Ken.nfo")
	if data, err := os.ReadFile(nfoTarget); err != nil {
		t.Errorf("nfo file not created at %s: %v", nfoTarget, err)
	} else {
		content := string(data)
		if !strings.Contains(content, "<title>Чёрный клевер: Меч короля магов</title>") {
			t.Errorf("nfo missing expected Russian title, got: %s", content)
		}
		if !strings.Contains(content, `<uniqueid type="shikimori" default="true">50410</uniqueid>`) {
			t.Errorf("nfo missing shikimori ID, got: %s", content)
		}
	}
}

func TestSeason00ShowsAndMovies(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "jelly-an-li-season00-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	restore := providers.SetShikimoriCacheForTest(map[string]providers.CachedShikimoriInfo{
		"Kimetsu no Yaiba": {
			Russian:   "Клинок, рассекающий демонов",
			Romaji:    "Kimetsu no Yaiba",
			Season:    1,
			IsMovie:   false,
			IsSpecial: false,
		},
		"Kimetsu no Yaiba Movie Mugen Ressha hen": {
			Russian:   "Клинок, рассекающий демонов: Поезд «Бесконечный»",
			Romaji:    "Kimetsu no Yaiba: Mugen Ressha-hen",
			Season:    1,
			IsMovie:   true,
			IsSpecial: false,
		},
	})
	defer restore()

	torrentsDir := filepath.Join(tmpDir, "Torrents")
	libraryDir := filepath.Join(tmpDir, "Anime")
	os.MkdirAll(torrentsDir, 0755)
	os.MkdirAll(libraryDir, 0755)

	// Раздача 1: Сериал
	seriesFolder := filepath.Join(torrentsDir, "[Kawaiika-Raws] (2019) Kimetsu no Yaiba [BDRip 1920x1080 HEVC FLAC]")
	os.MkdirAll(seriesFolder, 0755)
	ep1 := filepath.Join(seriesFolder, "[Kawaiika-Raws] Kimetsu no Yaiba 01.mkv")
	os.WriteFile(ep1, []byte("ep 1 content"), 0644)

	// Раздача 2: Фильм с внешней аудиодорожкой
	movieFolder := filepath.Join(torrentsDir, "Kimetsu no Yaiba Movie Mugen Ressha-hen")
	os.MkdirAll(filepath.Join(movieFolder, "Rus Dub"), 0755)
	movieVideo := filepath.Join(movieFolder, "Kimetsu no Yaiba Mugen Ressha-hen.mkv")
	movieAudio := filepath.Join(movieFolder, "Rus Dub", "Kimetsu no Yaiba Mugen Ressha-hen.mka")
	os.WriteFile(movieVideo, []byte("movie video"), 0644)
	os.WriteFile(movieAudio, []byte("movie audio"), 0644)

	cfg := &config.Config{
		TorrentDirs:      []string{torrentsDir},
		LibraryDir:       libraryDir,
		FolderNamingMode: "russian",
		UseShikimori:     true,
		LanguageMapping: map[string]string{
			"Rus Dub": "ru",
		},
	}

	shows, err := Scan(cfg)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if len(shows) != 2 {
		t.Fatalf("expected 2 shows scanned, got %d", len(shows))
	}

	plan := GeneratePlan(shows, cfg)
	if len(plan) != 3 {
		t.Fatalf("expected 3 operations in plan (1 series + 1 movie video + 1 movie audio), got %d", len(plan))
	}

	var seriesOp, movieVideoOp, movieAudioOp *LinkOperation
	for _, op := range plan {
		if op.SourcePath == ep1 {
			seriesOp = op
		} else if op.SourcePath == movieVideo {
			movieVideoOp = op
		} else if op.SourcePath == movieAudio {
			movieAudioOp = op
		}
	}

	// 1. Проверяем путь сериала -> Season 01
	if seriesOp == nil {
		t.Fatalf("missing series operation")
	}
	expectedSeriesPath := filepath.Join(libraryDir, "Клинок, рассекающий демонов", "Season 01", "Kimetsu no Yaiba S01E01.mkv")
	if seriesOp.TargetPath != expectedSeriesPath {
		t.Errorf("expected series target '%s', got '%s'", expectedSeriesPath, seriesOp.TargetPath)
	}

	// 2. Проверяем путь фильма -> Season 00
	if movieVideoOp == nil || movieAudioOp == nil {
		t.Fatalf("missing movie operations")
	}
	expectedSeason00Folder := filepath.Join(libraryDir, "Клинок, рассекающий демонов", "Season 00")
	expectedMovieVideo := filepath.Join(expectedSeason00Folder, "Kimetsu no Yaiba S00E01 - Kimetsu no Yaiba - Mugen Ressha-hen.mkv")
	expectedMovieAudio := filepath.Join(expectedSeason00Folder, "Kimetsu no Yaiba S00E01 - Kimetsu no Yaiba - Mugen Ressha-hen.ru.mka")

	if movieVideoOp.TargetPath != expectedMovieVideo {
		t.Errorf("expected movie video target '%s', got '%s'", expectedMovieVideo, movieVideoOp.TargetPath)
	}
	if movieAudioOp.TargetPath != expectedMovieAudio {
		t.Errorf("expected movie audio target '%s', got '%s'", expectedMovieAudio, movieAudioOp.TargetPath)
	}

	// 3. Применяем план
	statePath := filepath.Join(tmpDir, "state.json")
	if err := ApplyPlan(plan, statePath, cfg); err != nil {
		t.Fatalf("ApplyPlan failed: %v", err)
	}

	for _, op := range plan {
		if info, err := os.Lstat(op.TargetPath); err != nil {
			t.Errorf("symlink not found at %s: %v", op.TargetPath, err)
		} else if info.Mode()&os.ModeSymlink == 0 {
			t.Errorf("path is not a symlink: %s", op.TargetPath)
		}
	}

	// Проверяем NFO файл
	nfoPath := filepath.Join(expectedSeason00Folder, "Kimetsu no Yaiba S00E01 - Kimetsu no Yaiba - Mugen Ressha-hen.nfo")
	if data, err := os.ReadFile(nfoPath); err != nil {
		t.Errorf("expected NFO file at %s: %v", nfoPath, err)
	} else if !strings.Contains(string(data), "<title>Клинок, рассекающий демонов: Поезд «Бесконечный»</title>") {
		t.Errorf("expected NFO title, got: %s", string(data))
	}

	// 4. Проверяем CleanBrokenLinks
	os.RemoveAll(movieFolder)
	cleaned, err := CleanBrokenLinks(cfg, statePath, plan[:1]) // только сериал остался
	if err != nil {
		t.Fatalf("CleanBrokenLinks failed: %v", err)
	}
	if len(cleaned) == 0 {
		t.Errorf("expected movie links to be cleaned, got 0")
	}
}

func TestMoviesRomajiMode(t *testing.T) {
	shows := []*parser.AnimeShow{
		{
			CleanedName: "Kimetsu no Yaiba Mugen Ressha Hen",
			RussianName: "Клинок, рассекающий демонов: Поезд «Бесконечный»",
			RomajiName:  "Kimetsu no Yaiba: Mugen Ressha-hen",
			IsMovie:     true,
			Files: []*parser.EpisodeFile{
				{
					SourcePath: "/torrents/movie.mkv",
					Type:       parser.TypeVideo,
				},
			},
		},
	}

	cfg := &config.Config{
		LibraryDir:       "/media/Anime",
		FolderNamingMode: "romaji",
	}

	plan := GeneratePlan(shows, cfg)
	if len(plan) != 1 {
		t.Fatalf("expected 1 link operation, got %d", len(plan))
	}

	expectedDir := filepath.Join("/media/Anime", "Kimetsu no Yaiba - Mugen Ressha-hen", "Season 00")
	if filepath.Dir(plan[0].TargetPath) != expectedDir {
		t.Errorf("expected Romaji movie folder '%s', got '%s'", expectedDir, filepath.Dir(plan[0].TargetPath))
	}
}






