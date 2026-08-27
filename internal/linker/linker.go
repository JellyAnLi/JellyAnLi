package linker

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"jelly-an-li/internal/config"
	"jelly-an-li/internal/parser"
	"jelly-an-li/internal/providers"
)

type LinkStatus string

const (
	StatusPending LinkStatus = "pending"
	StatusSuccess LinkStatus = "success"
	StatusSkipped LinkStatus = "skipped"
	StatusError   LinkStatus = "error"
)

type LinkOperation struct {
	SourcePath string     `json:"source_path"`
	TargetPath string     `json:"target_path"`
	Status     LinkStatus `json:"status"`
	Message    string     `json:"message,omitempty"`
}

func processShowMetadata(show *parser.AnimeShow, rawName string, cfg *config.Config) {
	hasExplicitSeason := false
	if _, ok := parser.HasExplicitSeason(rawName); ok {
		hasExplicitSeason = true
	}

	providersList := cfg.MetadataProviders
	if len(providersList) == 0 && cfg.UseShikimori {
		providersList = []string{"shikimori", "anilist", "anidb"}
	}

	if len(providersList) > 0 {
		query := show.CleanedName
		if show.Season > 1 && !show.IsMovie {
			query = fmt.Sprintf("%s %d", show.CleanedName, show.Season)
		}

		var meta *providers.AnimeMetadata
		for _, provID := range providersList {
			prov := providers.Get(provID)
			if prov == nil {
				continue
			}

			proxyURL := ""
			if cfg.ProxyRouting != nil {
				proxyURL = cfg.ProxyRouting.GetProxyFor(provID)
			} else if cfg.ProxyURL != "" {
				proxyURL = cfg.ProxyURL
			}

			m, err := prov.Search(query, proxyURL)
			if err == nil && m == nil && show.Season > 1 && !show.IsMovie {
				m, err = prov.Search(show.CleanedName, proxyURL)
			}

			if err == nil && m != nil && (m.TitleRu != "" || m.TitleRomaji != "" || m.Season > 0 || m.IsMovie) {
				meta = m
				fmt.Printf("[DEBUG] linker.Scan: Provider '%s' matched '%s'\n", provID, show.CleanedName)
				break
			}
		}

		if meta != nil {
			if meta.TitleRu != "" {
				show.RussianName = meta.TitleRu
				fmt.Printf("[DEBUG] linker.Scan: Found Russian title for '%s' -> '%s'\n", show.CleanedName, show.RussianName)
			}
			if meta.TitleRomaji != "" {
				show.RomajiName = meta.TitleRomaji
				fmt.Printf("[DEBUG] linker.Scan: Found Romaji title for '%s' -> '%s'\n", show.CleanedName, show.RomajiName)
			}
			if meta.IsMovie {
				show.IsMovie = true
				fmt.Printf("[DEBUG] linker.Scan: Identified movie for '%s'\n", show.CleanedName)
			} else if meta.IsSpecial {
				if show.Season <= 1 && !hasExplicitSeason {
					show.Season = 0
					fmt.Printf("[DEBUG] linker.Scan: Identified special/OVA for '%s' -> Season 00\n", show.CleanedName)
					for _, f := range show.Files {
						f.SeasonNum = 0
					}
				}
			} else if meta.Season > 1 && show.Season == 1 && !show.IsMovie {
				show.Season = meta.Season
				fmt.Printf("[DEBUG] linker.Scan: Calculated franchise season for '%s' -> Season %d\n", show.CleanedName, meta.Season)
				for _, f := range show.Files {
					f.SeasonNum = meta.Season
				}
			}
		}
	}
}

// Scan сканирует директорию с торрентами и группирует файлы по раздачам
func Scan(cfg *config.Config) ([]*parser.AnimeShow, error) {
	var shows []*parser.AnimeShow

	fmt.Println("[DEBUG] linker.Scan: Start")

	// Поддерживаем как массив torrent_dirs, так и обратную совместимость
	dirsToScan := cfg.TorrentDirs
	if len(dirsToScan) == 0 && cfg.TorrentDir != "" {
		dirsToScan = []string{cfg.TorrentDir}
	}

	for _, dir := range dirsToScan {
		if dir == "" {
			continue
		}

		fmt.Println("[DEBUG] linker.Scan: Scanning directory", dir)

		entries, err := os.ReadDir(dir)
		if err != nil {
			fmt.Println("[DEBUG] linker.Scan: ReadDir error", err)
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				showFolderName := entry.Name()
				showAbsPath := filepath.Join(dir, showFolderName)

				fmt.Println("[DEBUG] linker.Scan: Processing folder", showFolderName)

				var relFiles []string
				err := filepath.WalkDir(showAbsPath, func(path string, d fs.DirEntry, err error) error {
					if err != nil {
						return nil
					}
					if !d.IsDir() {
						relPath, err := filepath.Rel(showAbsPath, path)
						if err == nil {
							relFiles = append(relFiles, relPath)
						}
					}
					return nil
				})

				if err != nil {
					continue
				}

				show := parser.ParseShowFolder(showFolderName, relFiles, cfg.LanguageMapping)
				if len(show.Files) > 0 {
					processShowMetadata(show, showFolderName, cfg)
					// Преобразуем относительные пути файлов в абсолютные
					for _, f := range show.Files {
						f.SourcePath = filepath.Join(showAbsPath, f.SourcePath)
					}
					shows = append(shows, show)
				}
			} else {
				// Одиночный файл в корне папки раздач (фильм или серия)
				fileName := entry.Name()
				if parser.ClassifyFileType(fileName) == parser.TypeUnknown {
					continue
				}

				fmt.Println("[DEBUG] linker.Scan: Processing standalone file", fileName)

				showFolderName := strings.TrimSuffix(fileName, filepath.Ext(fileName))
				relFiles := []string{fileName}

				show := parser.ParseShowFolder(showFolderName, relFiles, cfg.LanguageMapping)
				if len(show.Files) > 0 {
					processShowMetadata(show, showFolderName, cfg)
					// Преобразуем относительные пути файлов в абсолютные
					for _, f := range show.Files {
						f.SourcePath = filepath.Join(dir, f.SourcePath)
					}
					shows = append(shows, show)
				}
			}
		}
	}

	return shows, nil
}

func sanitizeFileName(name string) string {
	name = strings.ReplaceAll(name, ": ", " - ")
	name = strings.ReplaceAll(name, ":", " - ")
	name = strings.ReplaceAll(name, "/", " ")
	name = strings.ReplaceAll(name, "\\", " ")
	name = strings.ReplaceAll(name, "*", "")
	name = strings.ReplaceAll(name, "?", "")
	name = strings.ReplaceAll(name, "\"", "")
	name = strings.ReplaceAll(name, "<", "")
	name = strings.ReplaceAll(name, ">", "")
	name = strings.ReplaceAll(name, "|", "")
	return strings.TrimSpace(strings.Join(strings.Fields(name), " "))
}

// Вспомогательная функция для нормализации строк при сравнении
func normalizeTitle(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "ё", "е")
	var sb strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || (r >= 'а' && r <= 'я') || r == ' ' {
			sb.WriteRune(r)
		}
	}
	words := strings.Fields(sb.String())
	return strings.Join(words, " ")
}

// resolveShowFolderName определяет имя папки назначения для шоу
func resolveShowFolderName(show *parser.AnimeShow, allShows []*parser.AnimeShow, libraryDir string, namingMode ...string) string {
	mode := "russian"
	if len(namingMode) > 0 && namingMode[0] != "" {
		mode = namingMode[0]
	}

	showName := show.CleanedName
	switch strings.ToLower(mode) {
	case "romaji", "english", "en":
		if show.RomajiName != "" {
			showName = show.RomajiName
		}
	case "original", "orig":
		showName = show.CleanedName
	case "russian", "ru", "":
		if show.RussianName != "" {
			showName = show.RussianName
		}
	}

	normShowName := normalizeTitle(showName)
	if normShowName == "" {
		return showName
	}

	bestMatch := showName
	bestMatchLen := len(normShowName)

	for _, other := range allShows {
		if other == show {
			continue
		}
		otherName := other.CleanedName
		switch strings.ToLower(mode) {
		case "romaji", "english", "en":
			if other.RomajiName != "" {
				otherName = other.RomajiName
			}
		case "original", "orig":
			otherName = other.CleanedName
		case "russian", "ru", "":
			if other.RussianName != "" {
				otherName = other.RussianName
			}
		}
		normOtherName := normalizeTitle(otherName)
		if normOtherName != "" {
			if normShowName == normOtherName {
				bestMatch = otherName
				bestMatchLen = len(normOtherName)
			} else if strings.HasPrefix(normShowName, normOtherName+" ") {
				if len(normOtherName) < bestMatchLen {
					bestMatchLen = len(normOtherName)
					bestMatch = otherName
				}
			}
		}
	}

	// Ищем среди уже существующих папок в libraryDir
	if libraryDir != "" {
		if entries, err := os.ReadDir(libraryDir); err == nil {
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				existName := entry.Name()
				normExistName := normalizeTitle(existName)
				if normExistName != "" {
					if normShowName == normExistName {
						bestMatch = existName
						bestMatchLen = len(normExistName)
					} else if strings.HasPrefix(normShowName, normExistName+" ") {
						if len(normExistName) < bestMatchLen {
							bestMatchLen = len(normExistName)
							bestMatch = existName
						}
					}
				}
			}
		}
	}

	if bestMatch != showName {
		fmt.Printf("[DEBUG] resolveShowFolderName: Matched show '%s' to root show '%s'\n", showName, bestMatch)
	}

	return bestMatch
}

// AlignShowsPartEpisodes выравнивает части/куры между всеми раздачами одного сериала и сезона
func AlignShowsPartEpisodes(shows []*parser.AnimeShow, cfg *config.Config) {
	type seasonKey struct {
		showFolder string
		seasonNum  int
	}

	filesBySeason := make(map[seasonKey][]*parser.EpisodeFile)

	for _, show := range shows {
		showFolder := resolveShowFolderName(show, shows, cfg.LibraryDir, cfg.FolderNamingMode)
		for _, f := range show.Files {
			key := seasonKey{
				showFolder: showFolder,
				seasonNum:  f.SeasonNum,
			}
			filesBySeason[key] = append(filesBySeason[key], f)
		}
	}

	for _, seasonFiles := range filesBySeason {
		parser.AlignPartEpisodes(seasonFiles)
		dummyShow := &parser.AnimeShow{
			Files: seasonFiles,
		}
		parser.AlignEpisodeNumbers(dummyShow)
	}
}

// resolveEnglishShowName определяет самое короткое базовое английское имя сериала (без подзаголовков арок)
func resolveEnglishShowName(show *parser.AnimeShow, allShows []*parser.AnimeShow) string {
	showName := show.CleanedName
	normShowName := normalizeTitle(showName)
	if normShowName == "" {
		return showName
	}

	bestMatch := showName
	bestMatchLen := len(normShowName)

	for _, other := range allShows {
		otherName := other.CleanedName
		normOtherName := normalizeTitle(otherName)
		if normOtherName != "" {
			if normShowName == normOtherName || strings.HasPrefix(normShowName, normOtherName+" ") {
				if len(normOtherName) < bestMatchLen {
					bestMatchLen = len(normOtherName)
					bestMatch = otherName
				}
			}
		}
	}
	return bestMatch
}

// GeneratePlan генерирует список операций линкования
func GeneratePlan(shows []*parser.AnimeShow, cfg *config.Config) []*LinkOperation {
	AlignShowsPartEpisodes(shows, cfg)

	var plan []*LinkOperation
	usedTargets := make(map[string]string)

	// Считаем максимальный номер эпизода для каждого сезона сериала, чтобы корректно форматировать S01E001 или S01E01
	type seasonKey struct {
		showFolder string
		seasonNum  int
	}
	maxEpBySeason := make(map[seasonKey]int)
	for _, show := range shows {
		showFolder := resolveShowFolderName(show, shows, cfg.LibraryDir, cfg.FolderNamingMode)
		for _, file := range show.Files {
			if !show.IsMovie && file.EpisodeNum > 0 {
				key := seasonKey{showFolder: showFolder, seasonNum: file.SeasonNum}
				if file.EpisodeNum > maxEpBySeason[key] {
					maxEpBySeason[key] = file.EpisodeNum
				}
			}
		}
	}

	for _, show := range shows {
		showFolder := resolveShowFolderName(show, shows, cfg.LibraryDir, cfg.FolderNamingMode)
		englishShowTitle := resolveEnglishShowName(show, shows)

		for _, file := range show.Files {
			if file.Type == parser.TypeUnknown {
				continue
			}

			if cfg.LibraryDir == "" {
				continue
			}

			var targetDir string
			var targetName string
			ext := filepath.Ext(file.SourcePath)

			fileTitle := englishShowTitle
			if fileTitle == "" {
				fileTitle = showFolder
			}

			if show.IsMovie {
				// Кладем фильм во вложенную папку "Films" в папке аниме
				targetDir = filepath.Join(cfg.LibraryDir, showFolder, "Films")

				movieTitle := show.CleanedName
				if show.RomajiName != "" && show.RomajiName != englishShowTitle {
					movieTitle = show.RomajiName
				}
				if movieTitle == "" {
					movieTitle = fileTitle
				}
				movieTitle = sanitizeFileName(movieTitle)

				if file.Type == parser.TypeVideo {
					targetName = movieTitle + ext
				} else {
					parts := []string{movieTitle}
					if file.Suffix != "" {
						parts = append(parts, file.Suffix)
					}
					if file.LangCode != "" {
						parts = append(parts, file.LangCode)
					}
					targetName = strings.Join(parts, ".") + ext
				}
			} else {
				// Кладем сериал в "Season XX"
				targetDir = filepath.Join(cfg.LibraryDir, showFolder, fmt.Sprintf("Season %02d", file.SeasonNum))

				if file.EpisodeNum != -1 {
					key := seasonKey{showFolder: showFolder, seasonNum: file.SeasonNum}
					maxEp := maxEpBySeason[key]

					epTag := fmt.Sprintf("S%02dE%02d", file.SeasonNum, file.EpisodeNum)
					if maxEp >= 1000 {
						epTag = fmt.Sprintf("S%02dE%04d", file.SeasonNum, file.EpisodeNum)
					} else if maxEp >= 100 {
						epTag = fmt.Sprintf("S%02dE%03d", file.SeasonNum, file.EpisodeNum)
					}

					// Формируем имя файла на английском: ShowCleanName SXXEYYY.[Suffix].[Lang].ext
					parts := []string{
						fmt.Sprintf("%s %s", fileTitle, epTag),
					}
					if file.Suffix != "" {
						parts = append(parts, file.Suffix)
					}
					if file.LangCode != "" {
						parts = append(parts, file.LangCode)
					}
					targetName = strings.Join(parts, ".") + ext
				} else {
					origBase := strings.TrimSuffix(filepath.Base(file.SourcePath), ext)
					parts := []string{origBase}
					if file.Suffix != "" && !strings.Contains(origBase, file.Suffix) {
						parts = append(parts, file.Suffix)
					}
					if file.LangCode != "" && !strings.Contains(origBase, "."+file.LangCode) {
						parts = append(parts, file.LangCode)
					}
					targetName = strings.Join(parts, ".") + ext
				}
			}

			targetPath := filepath.Join(targetDir, targetName)
			if existingSource, exists := usedTargets[targetPath]; exists && existingSource != file.SourcePath {
				if file.Type == parser.TypeVideo {
					fmt.Printf("[DEBUG] GeneratePlan: Skipping duplicate video file for '%s' (already linked from '%s')\n", targetPath, existingSource)
					continue
				}
				ext := filepath.Ext(targetName)
				baseName := strings.TrimSuffix(targetName, ext)
				counter := 2
				newTargetName := fmt.Sprintf("%s_%d%s", baseName, counter, ext)
				newTargetPath := filepath.Join(targetDir, newTargetName)
				for usedTargets[newTargetPath] != "" && usedTargets[newTargetPath] != file.SourcePath {
					counter++
					newTargetName = fmt.Sprintf("%s_%d%s", baseName, counter, ext)
					newTargetPath = filepath.Join(targetDir, newTargetName)
				}
				targetPath = newTargetPath
			}
			usedTargets[targetPath] = file.SourcePath

			plan = append(plan, &LinkOperation{
				SourcePath: file.SourcePath,
				TargetPath: targetPath,
				Status:     StatusPending,
			})
		}
	}

	return plan
}

// ApplyPlan применяет сгенерированный план с оптимизацией через state.json кэш
func ApplyPlan(plan []*LinkOperation, statePath string, cfgOptions ...*config.Config) error {
	fmt.Printf("[DEBUG] linker.ApplyPlan: Start. Operations: %d\n", len(plan))
	state, err := LoadState(statePath)
	if err != nil {
		state = NewSyncState()
	}

	useRelative := true
	if len(cfgOptions) > 0 && cfgOptions[0] != nil {
		useRelative = cfgOptions[0].UseRelativeSymlinks
	}

	for i, op := range plan {
		fmt.Printf("[DEBUG] linker.ApplyPlan: [%d/%d] processing file: %s -> %s\n", i+1, len(plan), op.SourcePath, op.TargetPath)
		// Получаем информацию об источнике
		info, err := os.Stat(op.SourcePath)
		if os.IsNotExist(err) {
			op.Status = StatusError
			op.Message = fmt.Sprintf("source file does not exist: %v", err)
			continue
		} else if err != nil {
			op.Status = StatusError
			op.Message = fmt.Sprintf("failed to stat source file: %v", err)
			continue
		}

		// Создаем директории назначения
		targetDir := filepath.Dir(op.TargetPath)
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			op.Status = StatusError
			op.Message = fmt.Sprintf("failed to create directory: %v", err)
			continue
		}

		absSource, err := filepath.Abs(op.SourcePath)
		if err != nil {
			op.Status = StatusError
			op.Message = fmt.Sprintf("failed to get absolute source path: %v", err)
			continue
		}

		linkTarget := absSource
		if useRelative {
			if relTarget, err := filepath.Rel(targetDir, absSource); err == nil {
				linkTarget = relTarget
			}
		}

		// Если файл уже существует на диске
		if info, err := os.Lstat(op.TargetPath); err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				linkTargetOnDisk, err := os.Readlink(op.TargetPath)
				if err == nil && linkTargetOnDisk == linkTarget {
					op.Status = StatusSkipped
					op.Message = "link already correct"
					state.Files[op.SourcePath] = FileState{
						Mtime:      info.ModTime(),
						Size:       info.Size(),
						TargetPath: op.TargetPath,
					}
					continue
				}
			}

			if err := os.Remove(op.TargetPath); err != nil {
				op.Status = StatusError
				op.Message = fmt.Sprintf("failed to remove existing file: %v", err)
				continue
			}
		}

		err = os.Symlink(linkTarget, op.TargetPath)
		if err != nil {
			op.Status = StatusError
			if strings.Contains(err.Error(), "privilege is not held") {
				op.Message = "Windows error: administrator privileges or developer mode required"
			} else {
				op.Message = err.Error()
			}
		} else {
			fmt.Printf("[DEBUG] Created symlink: %s -> %s\n", op.TargetPath, linkTarget)
			op.Status = StatusSuccess
			// Обновляем кэш состояния
			state.Files[op.SourcePath] = FileState{
				Mtime:      info.ModTime(),
				Size:       info.Size(),
				TargetPath: op.TargetPath,
			}
		}
	}

	// Сохраняем обновленный кэш состояния
	_ = state.Save(statePath)
	return nil
}

// hasValidMediaOrLinks проверяет, есть ли в директории (или ее поддиректориях) хотя бы один валидный симлинк или медиафайл
func hasValidMediaOrLinks(dirPath string) bool {
	hasMedia := false
	_ = filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil || hasMedia {
			return nil
		}
		if d.IsDir() {
			return nil
		}

		info, err := os.Lstat(path)
		if err != nil {
			return nil
		}

		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err == nil {
				if !filepath.IsAbs(target) {
					target = filepath.Join(filepath.Dir(path), target)
				}
				if _, err := os.Stat(target); err == nil {
					hasMedia = true
					return filepath.SkipAll
				}
			}
		} else {
			if parser.ClassifyFileType(path) != parser.TypeUnknown {
				hasMedia = true
				return filepath.SkipAll
			}
		}
		return nil
	})
	return hasMedia
}

// CleanBrokenLinks находит и удаляет неработающие симлинки, устаревшие ссылки и пустые/осиротевшие папки
func CleanBrokenLinks(cfg *config.Config, statePath string, activePlan ...[]*LinkOperation) ([]string, error) {
	var deletedPaths []string

	validTargets := make(map[string]bool)
	hasPlan := len(activePlan) > 0 && activePlan[0] != nil
	if hasPlan {
		for _, op := range activePlan[0] {
			validTargets[filepath.Clean(op.TargetPath)] = true
		}
	}

	// Если передан путь к кэшу состояния, очистим из него удаленные записи
	if statePath != "" {
		if state, err := LoadState(statePath); err == nil && state.Files != nil {
			changed := false
			for src, fState := range state.Files {
				if _, err := os.Stat(src); os.IsNotExist(err) {
					delete(state.Files, src)
					changed = true
				} else if _, err := os.Lstat(fState.TargetPath); os.IsNotExist(err) {
					delete(state.Files, src)
					changed = true
				}
			}
			if changed {
				_ = state.Save(statePath)
			}
		}
	}

	dirsToClean := []string{cfg.LibraryDir}

	for _, dir := range dirsToClean {
		if dir == "" {
			continue
		}

		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}

		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			info, err := os.Lstat(path)
			if err != nil {
				return nil
			}

			if info.Mode()&os.ModeSymlink != 0 {
				target, err := os.Readlink(path)
				if err != nil {
					if os.Remove(path) == nil {
						deletedPaths = append(deletedPaths, path)
					}
					return nil
				}

				absTarget := target
				if !filepath.IsAbs(target) {
					absTarget = filepath.Join(filepath.Dir(path), target)
				}

				// 1. Недействительная ссылка (источник больше не существует)
				if _, err := os.Stat(absTarget); err != nil {
					if os.Remove(path) == nil {
						deletedPaths = append(deletedPaths, path)
					}
					return nil
				}

				// 2. Устаревшая ссылка, указывающая в папки торрентов, но больше не входящая в план (например, старый формат E10 или опенинг)
				if hasPlan && !validTargets[filepath.Clean(path)] {
					isFromTorrent := false
					for _, td := range cfg.TorrentDirs {
						if td != "" {
							if rel, err := filepath.Rel(td, absTarget); err == nil && !strings.HasPrefix(rel, "..") {
								isFromTorrent = true
								break
							}
						}
					}
					if isFromTorrent {
						if os.Remove(path) == nil {
							deletedPaths = append(deletedPaths, path)
						}
					}
				}
			}

			return nil
		})

		if err != nil {
			return deletedPaths, err
		}

		var folders []string
		_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err == nil && d.IsDir() && filepath.Clean(path) != filepath.Clean(dir) {
				folders = append(folders, path)
			}
			return nil
		})

		for i := len(folders) - 1; i >= 0; i-- {
			fPath := folders[i]

			if _, err := os.Stat(fPath); os.IsNotExist(err) {
				continue
			}

			if !hasValidMediaOrLinks(fPath) {
				if err := os.RemoveAll(fPath); err == nil {
					deletedPaths = append(deletedPaths, fPath+" (directory cleaned)")
				}
			} else {
				if err := os.Remove(fPath); err == nil {
					deletedPaths = append(deletedPaths, fPath+" (directory cleaned)")
				}
			}
		}
	}

	return deletedPaths, nil
}
