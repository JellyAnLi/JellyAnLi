package parser

import (
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type FileType string

const (
	TypeVideo    FileType = "video"
	TypeAudio    FileType = "audio"
	TypeSubtitle FileType = "subtitle"
	TypeUnknown  FileType = "unknown"
)

type EpisodeFile struct {
	SourcePath string   // Исходный абсолютный или относительный путь к файлу
	EpisodeNum int      // Номер эпизода (-1 если не определен)
	SeasonNum  int      // Номер сезона
	PartNum    int      // Номер части/куров (1 по умолчанию)
	Type       FileType // Тип файла (video, audio, subtitle)
	Suffix     string   // Дополнительный суффикс (например, AL, SB, CR.DUB, CR.full, CR.signs)
	LangCode   string   // Код языка (ru, en или пустой для видео)
}

type AnimeShow struct {
	OriginalName string         // Оригинальное имя папки раздачи (например, "Kaijuu 8 Gou TV-2 [1080p]")
	CleanedName  string         // Очищенное имя аниме (например, "Kaijuu 8 Gou")
	RussianName  string         // Русское название аниме из базы (Shikimori)
	RomajiName   string         // Официальное ромадзи/английское название аниме из базы
	MovieTitleRu string         // Русское название конкретного фильма
	MovieRomaji  string         // Ромадзи название конкретного фильма
	ShikimoriID  int            // ID на Shikimori
	AniListID    int            // ID на AniList
	Season       int            // Дефолтный сезон раздачи
	Part         int            // Номер части/куров (1 по умолчанию)
	IsMovie      bool           // Является ли фильмом
	Files        []*EpisodeFile // Файлы, относящиеся к раздаче
}

var (
	// Регулярные выражения для очистки скобок и мусора
	bracketsRegex = regexp.MustCompile(`\[[^\]]*\]|\([^\)]*\)`)
	spacesRegex   = regexp.MustCompile(`\s+`)
	garbageRegex  = regexp.MustCompile(`(?i)\b(1080p|720p|480p|bdrip|web-dl|webrip|web|h264|h265|hevc|x264|x265|avc|aac|flac|mp3|truehd|atmos|dts(?:-hd)?|10bit|8bit|hi10p|dual-audio|multi-audio|rus|eng|jpn|raw|sub|dub|cr|crunchyroll|netflix|rutracker|nnmclub|mp4|mkv|avi|tag|rev\d*|v\d+|repack|remux|remastered|uncut|uncensored|batch|19\d{2}|20\d{2}|erai-raws|subsplease|horriblesubs|judas|cleo|pas|ember|reaktor|asw|chibi|kametsu|vcb-studio|moozzi2|sofcj-raws|kawaiika-raws|varyg|jannyy)\b`)
	releaseGroupPrefixRegex = regexp.MustCompile(`^(?i)(?:erai-raws|subsplease|horriblesubs|erai_raws|judas|cleo|pas|ember|reaktor|asw|chibi|kametsu|vcb-studio|moozzi2|sofcj-raws|kawaiika-raws|varyg)[-._\s]+`)
	moviePrefixRegex        = regexp.MustCompile(`^(?i)(?:eiga|gekijouban|gekijou-ban|movie|фильм)[-._\s]+`)
	movieKeywordsRegex      = regexp.MustCompile(`(?i)(?:\b(?:movie|film|films|фильм|фильмы|gekijouban|gekijou-ban|eiga)\b|g劇)`)

	// Регулярные выражения для извлечения сезона
	seasonRegexes = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bS(\d{1,2})\b`),
		regexp.MustCompile(`(?i)\b(?:season|сезон)\b\s?[-_]?\s?(\d{1,2})\b`),
		regexp.MustCompile(`(?i)\b(\d{1,2})(?:st|nd|rd|th)\b\s*(?:season|сезон)?`),
		regexp.MustCompile(`(?i)(?:сезон|тв)\s?[-_]?\s?(\d{1,2})`),
		regexp.MustCompile(`(?i)(\d{1,2})\s?(?:сезон|season|тв|tv)`),
		regexp.MustCompile(`(?i)\bS(\d{1,2})E(\d{1,3})\b`),
		regexp.MustCompile(`(?i)\b(?:tv)\s?[-_]?\s?(\d{1,2})\b`),
		regexp.MustCompile(`(?i)\b(?:part|cour|часть)\b\s?[-_]?\s?(\d{1,2})\b`),
		regexp.MustCompile(`(?i)\b([2-9])\b\s*(?:\(\d{4}\)|\(\s*(?:tv|тв)?\s*\d*\s*\)|\[|$)`),
	}

	// Регулярные выражения для извлечения части/куров (Part/Cour)
	partRegexes = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(?:part|cour|часть|pt)\b\s?[-_]?\s?(\d{1,2})\b`),
		regexp.MustCompile(`(?i)(\d{1,2})\s?(?:part|cour|часть|pt)\b`),
	}

	// Регулярное выражение для римских цифр частей/куров
	romanPartRegex = regexp.MustCompile(`(?i)\b(?:part|cour|часть|pt)\b\s*[-_.]?\s*\b(I|II|III|IV|V|VI|VII|VIII|IX|X|XI|XII)\b`)

	// Регулярное выражение для римских цифр сезонов (II, III, IV, V, VI, VII, VIII, IX, X, XI, XII)
	romanSeasonRegex = regexp.MustCompile(`(?i)\b(?:season|part|cour|сезон|часть|tv|тв)?\s*[-_.]?\s*\b(II|III|IV|V|VI|VII|VIII|IX|X|XI|XII)\b`)

	// Регулярные выражения для извлечения номера эпизода
	episodeRegexes = []*regexp.Regexp{
		regexp.MustCompile(`(?i)S\d+E(\d+)\b`),
		regexp.MustCompile(`(?i)\bEP?_?(\d+)\b`),
		regexp.MustCompile(`\s+-\s+(\d+)\b`),
		regexp.MustCompile(`(?i)\b(?:серия|серия\s+-|episode)\s*(\d+)\b`),
		regexp.MustCompile(`^\s*(\d{1,3})\s*[\.-]`),
	}

	// Регулярные выражения для удаления языковых меток из суффиксов
	langSuffixRegex = regexp.MustCompile(`(?i)\b(rus?|eng?|dub|sub|full|signs)\b`)

	// Регулярное выражение для извлечения содержимого скобок
	bracketsContentRegex = regexp.MustCompile(`\[([^\]]+)\]|\(([^)]+)\)`)

	// Регулярное выражение для извлечения/удаления экстр, оппенингов и эндингов из имён
	extraTagsRegex = regexp.MustCompile(`(?i)\b(?:NCOP\d*[a-z]?|NCED\d*[a-z]?|OP\d*[a-z]?|ED\d*[a-z]?|PV\d*|CM\d*|Menu|Preview|Trailer|Extras?|Bonus(?:es)?|Special|Specials)\b.*$`)
)

func parseRomanNumeral(s string) int {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "I":
		return 1
	case "II":
		return 2
	case "III":
		return 3
	case "IV":
		return 4
	case "V":
		return 5
	case "VI":
		return 6
	case "VII":
		return 7
	case "VIII":
		return 8
	case "IX":
		return 9
	case "X":
		return 10
	case "XI":
		return 11
	case "XII":
		return 12
	default:
		return 0
	}
}

// CleanShowName очищает имя папки от скобок, сезонов и мусорных тегов
func CleanShowName(folderName string) string {
	orig := folderName

	name := releaseGroupPrefixRegex.ReplaceAllString(folderName, "")
	name = bracketsRegex.ReplaceAllString(name, " ")
	name = moviePrefixRegex.ReplaceAllString(name, "")
	name = extraTagsRegex.ReplaceAllString(name, " ")

	// Заменяем разделители _ и . на пробелы ДО regex, сохраняя дефисы для тегов вроде web-dl
	name = strings.ReplaceAll(name, "_", " ")
	name = strings.ReplaceAll(name, ".", " ")

	for _, re := range seasonRegexes {
		name = re.ReplaceAllString(name, " ")
	}
	for _, re := range partRegexes {
		name = re.ReplaceAllString(name, " ")
	}
	name = romanPartRegex.ReplaceAllString(name, " ")
	name = romanSeasonRegex.ReplaceAllString(name, " ")

	name = garbageRegex.ReplaceAllString(name, " ")

	name = strings.ReplaceAll(name, "-", " ")
	name = spacesRegex.ReplaceAllString(name, " ")
	name = strings.TrimSpace(name)

	if name == "" {
		return orig
	}
	return name
}

// ExtractShowNameFromFile извлекает имя шоу из названия файла
func ExtractShowNameFromFile(fileName string) string {
	base := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	base = releaseGroupPrefixRegex.ReplaceAllString(base, "")
	base = moviePrefixRegex.ReplaceAllString(base, "")
	name := bracketsRegex.ReplaceAllString(base, " ")
	name = extraTagsRegex.ReplaceAllString(name, " ")

	// Заменяем разделители _ и . на пробелы ДО regex
	name = strings.ReplaceAll(name, "_", " ")
	name = strings.ReplaceAll(name, ".", " ")

	// Удаляем сезоны и части
	for _, re := range seasonRegexes {
		name = re.ReplaceAllString(name, " ")
	}
	for _, re := range partRegexes {
		name = re.ReplaceAllString(name, " ")
	}
	name = romanPartRegex.ReplaceAllString(name, " ")
	name = romanSeasonRegex.ReplaceAllString(name, " ")

	name = garbageRegex.ReplaceAllString(name, " ")
	name = strings.ReplaceAll(name, "-", " ")

	// Очищаем лишние пробелы перед делением на слова
	name = spacesRegex.ReplaceAllString(name, " ")
	name = strings.TrimSpace(name)

	// Удаляем номер серии, если он идет в конце названия
	words := spacesRegex.Split(name, -1)
	if len(words) > 0 {
		lastWord := strings.Trim(words[len(words)-1], "-_ \t.")
		// Если последнее слово — это число, удаляем его
		if _, err := strconv.Atoi(lastWord); err == nil {
			words = words[:len(words)-1]
			name = strings.Join(words, " ")
		}
	}

	name = spacesRegex.ReplaceAllString(name, " ")
	return strings.TrimSpace(name)
}

// CleanQueryForSearch очищает поисковый запрос от мусора, скобок и тегов
func CleanQueryForSearch(query string) string {
	cleaned := releaseGroupPrefixRegex.ReplaceAllString(query, " ")
	cleaned = bracketsRegex.ReplaceAllString(cleaned, " ")
	cleaned = moviePrefixRegex.ReplaceAllString(cleaned, " ")
	cleaned = extraTagsRegex.ReplaceAllString(cleaned, " ")

	cleaned = strings.ReplaceAll(cleaned, "_", " ")
	cleaned = strings.ReplaceAll(cleaned, ".", " ")

	for _, re := range partRegexes {
		cleaned = re.ReplaceAllString(cleaned, " ")
	}
	cleaned = romanPartRegex.ReplaceAllString(cleaned, " ")

	cleaned = garbageRegex.ReplaceAllString(cleaned, " ")
	cleaned = strings.ReplaceAll(cleaned, "-", " ")
	cleaned = spacesRegex.ReplaceAllString(cleaned, " ")
	return strings.TrimSpace(cleaned)
}

var (
	specialFileRegex = regexp.MustCompile(`(?i)\b(?:special|specials|sp|ova|ona|oad|спешл|спешлы|ова)\s*[-_]?\s*(\d*)\b`)
	sxxEyyRegex      = regexp.MustCompile(`(?i)\bS\d{1,2}E(\d{1,4})\b`)
	bracketEpRegex   = regexp.MustCompile(`(?i)[\[\(]\s*(?:ep|e|серия)?\s*[-_.]?\s*(\d{1,4})(?:[v_\-\.].*)?\s*[\]\)]`)
)

func isResolutionOrYear(val int) bool {
	if val == 480 || val == 576 || val == 720 || val == 1080 || val == 2160 || val == 1920 {
		return true
	}
	if val >= 1950 && val <= 2050 {
		return true
	}
	return false
}

// HasExplicitSeason извлекает номер сезона и возвращает true, если сезон был явно указан в строке
func HasExplicitSeason(s string) (int, bool) {
	// Проверяем спешлы / OVA (Season 0)
	if specialFileRegex.MatchString(s) {
		return 0, true
	}

	for _, re := range seasonRegexes {
		matches := re.FindStringSubmatch(s)
		if len(matches) > 1 {
			if val, err := strconv.Atoi(matches[1]); err == nil {
				return val, true
			}
		}
	}
	// Проверяем римские цифры сезона (например, Mushoku Tensei ... II)
	matches := romanSeasonRegex.FindStringSubmatch(s)
	if len(matches) > 1 {
		if val := parseRomanNumeral(matches[1]); val > 0 {
			return val, true
		}
	}
	return 1, false
}

// ExtractSeason извлекает номер сезона из имени папки (по умолчанию 1)
func ExtractSeason(folderName string) int {
	s, _ := HasExplicitSeason(folderName)
	return s
}

// ExtractPart извлекает номер части/куров из строки (по умолчанию 1)
func ExtractPart(s string) int {
	for _, re := range partRegexes {
		matches := re.FindStringSubmatch(s)
		if len(matches) > 1 {
			if val, err := strconv.Atoi(matches[1]); err == nil {
				return val
			}
		}
	}
	matches := romanPartRegex.FindStringSubmatch(s)
	if len(matches) > 1 {
		if val := parseRomanNumeral(matches[1]); val > 0 {
			return val
		}
	}
	return 1
}

// IsMovieFolder определяет, является ли папка фильмом по ключевым словам
func IsMovieFolder(folderName string) bool {
	return movieKeywordsRegex.MatchString(folderName)
}

// ClassifyFileType определяет тип файла по его расширению
func ClassifyFileType(path string) FileType {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".mkv", ".mp4", ".avi", ".m4v", ".mov", ".ts", ".m2ts", ".webm", ".flv", ".wmv":
		return TypeVideo
	case ".mka", ".ac3", ".flac", ".mp3", ".wav", ".aac", ".ogg", ".wma", ".dts", ".m4a":
		return TypeAudio
	case ".ass", ".srt", ".vtt", ".ssa", ".sub", ".idx", ".sup":
		return TypeSubtitle
	default:
		return TypeUnknown
	}
}

var themeOrExtraRegex = regexp.MustCompile(`(?i)\b(?:NCOP\d*[a-z]?|NCED\d*[a-z]?|OP\s*\d+[a-z]?|ED\s*\d+[a-z]?|PV\d*|CM\d*|Menu|Preview|Trailer|Clean\s*OP|Clean\s*ED)\b|ED_e\d+|OP_e\d+`)
var ignoredFoldersRegex = regexp.MustCompile(`(?i)^(?:bonus(?:es)?|бонус(?:ы)?|extras?|экстр[аы]|bd\s*menu|bdmenu|menu|меню|scans?|сканы|cd|ost|music|soundtrack|ост|музыка|covers?|artbooks?|артбук(?:и)?|booklets?|буклет(?:ы)?|openings?(?:\s*(?:&|and)\s*endings?)?|endings?|op\s*(?:&|and|_|-)?\s*ed|oped|ncop|nced|nc\s*op|nc\s*ed|nc|creditless|themes?|theme\s*songs?|pv|cm|trailers?|трейлер(?:ы)?|previews?|promos?|samples?|web-dl\s*extras?)$`)

// ExtractEpisodeNumber извлекает номер серии из названия файла
func ExtractEpisodeNumber(fileName string) int {
	base := strings.TrimSuffix(fileName, filepath.Ext(fileName))

	if themeOrExtraRegex.MatchString(base) {
		return -1
	}

	// 1. Проверяем спешлы вида "Special 1", "Special", "SP 01", "OVA 1"
	if sm := specialFileRegex.FindStringSubmatch(base); len(sm) > 0 {
		if len(sm) > 1 && sm[1] != "" {
			if val, err := strconv.Atoi(sm[1]); err == nil && val > 0 {
				return val
			}
		}
		return 1
	}

	// 2. Проверяем явный тег S01E01 / S1E10
	if sm := sxxEyyRegex.FindStringSubmatch(base); len(sm) > 1 {
		if val, err := strconv.Atoi(sm[1]); err == nil && val > 0 {
			return val
		}
	}

	// 3. Проверяем явные указания серии в теле строки: " - 01", "EP01", "Episode 1", "01."
	baseClean := bracketsRegex.ReplaceAllString(base, " ")
	baseClean = strings.ReplaceAll(baseClean, "_", " ")

	for _, re := range episodeRegexes {
		matches := re.FindStringSubmatch(baseClean)
		if len(matches) > 1 {
			if val, err := strconv.Atoi(matches[1]); err == nil && val > 0 {
				if !isResolutionOrYear(val) {
					return val
				}
			}
		}
	}

	// 4. Проверяем серии в скобках вида "[01]", "[01v2]", "(01)", "[EP01]", "[E01]" (VCB-Studio, Erai-raws, AniLibria)
	for _, bm := range bracketEpRegex.FindAllStringSubmatch(base, -1) {
		if len(bm) > 1 && bm[1] != "" {
			if val, err := strconv.Atoi(bm[1]); err == nil && val > 0 {
				if isResolutionOrYear(val) {
					continue
				}
				return val
			}
		}
	}

	// 5. Поиск числа с конца строки в очищенном имени
	words := spacesRegex.Split(baseClean, -1)
	for i := len(words) - 1; i >= 0; i-- {
		word := strings.Trim(words[i], "-_ \t.")
		if val, err := strconv.Atoi(word); err == nil && val > 0 {
			if !isResolutionOrYear(val) {
				return val
			}
		}
	}

	return -1
}

// ParseEpisodeFile парсит отдельный файл внутри раздачи
func ParseEpisodeFile(relPath string, defaultSeason int, defaultPart int, languageMapping map[string]string) *EpisodeFile {
	normalizedPath := filepath.ToSlash(relPath)
	pathParts := strings.Split(normalizedPath, "/")
	for i := 0; i < len(pathParts)-1; i++ {
		folderName := strings.TrimSpace(pathParts[i])
		if ignoredFoldersRegex.MatchString(folderName) {
			return nil
		}
	}

	fileName := filepath.Base(relPath)
	fileType := ClassifyFileType(fileName)

	if fileType == TypeUnknown {
		return nil
	}

	// Игнорируем опенинги, эндинги, трейлеры, превью и сэмплы
	if themeOrExtraRegex.MatchString(fileName) {
		return nil
	}
	if strings.Contains(strings.ToLower(fileName), "sample") {
		return nil
	}

	epNum := ExtractEpisodeNumber(fileName)

	fileSeason := defaultSeason
	if s := ExtractSeason(fileName); s != 1 {
		fileSeason = s
	} else {
		normalizedPath := filepath.ToSlash(relPath)
		parts := strings.Split(normalizedPath, "/")
		for i := len(parts) - 2; i >= 0; i-- {
			if s := ExtractSeason(parts[i]); s != 1 {
				fileSeason = s
				break
			}
		}
	}

	filePart := defaultPart
	if p := ExtractPart(fileName); p != 1 {
		filePart = p
	} else {
		normalizedPath := filepath.ToSlash(relPath)
		parts := strings.Split(normalizedPath, "/")
		for i := len(parts) - 2; i >= 0; i-- {
			if p := ExtractPart(parts[i]); p != 1 {
				filePart = p
				break
			}
		}
	}

	langCode := ""
	if fileType == TypeAudio || fileType == TypeSubtitle {
		normalizedPath := filepath.ToSlash(relPath)
		for folderKey, code := range languageMapping {
			if strings.Contains(strings.ToLower(normalizedPath), strings.ToLower(folderKey)) {
				langCode = code
				break
			}
		}
	}

	suffix := ""
	if fileType == TypeAudio || fileType == TypeSubtitle {
		suffix = extractSuffix(relPath, fileName)
	}

	return &EpisodeFile{
		SourcePath: relPath,
		EpisodeNum: epNum,
		SeasonNum:  fileSeason,
		PartNum:    filePart,
		Type:       fileType,
		Suffix:     suffix,
		LangCode:   langCode,
	}
}

// extractSuffix извлекает суффикс на основе имени файла и его пути
func extractSuffix(relPath, fileName string) string {
	base := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	relPath = filepath.ToSlash(relPath)
	parts := strings.Split(relPath, "/")

	var suffixParts []string

	for i, part := range parts {
		if i == len(parts)-1 {
			// Имя файла обработаем отдельно (из base)
			continue
		}

		lowerPart := strings.ToLower(part)

		// 1. Извлекаем содержимое скобок, если есть
		matches := bracketsContentRegex.FindStringSubmatch(part)
		if len(matches) > 0 {
			for _, m := range matches[1:] {
				if m != "" {
					suffixParts = append(suffixParts, strings.TrimSpace(m))
					break
				}
			}
		}

		// 2. Проверяем, является ли папка языковым/типовым контейнером для релиз-группы
		normPart := strings.ReplaceAll(lowerPart, " ", "")
		isLangContainer := normPart == "russound" || normPart == "russubs" ||
			normPart == "engsound" || normPart == "engsubs" ||
			normPart == "rusdub" || normPart == "russub" ||
			normPart == "engdub" || normPart == "engsub" ||
			normPart == "sound" || normPart == "subs" ||
			normPart == "dub" || normPart == "sub" ||
			normPart == "audio" || normPart == "subtitles"

		if isLangContainer {
			if i+1 < len(parts)-1 {
				nextPart := parts[i+1]
				nextClean := strings.Trim(nextPart, "[]()")
				nextPartLower := strings.ToLower(nextClean)
				isServiceFolder := nextPartLower == "signs" || nextPartLower == "надписи" ||
					nextPartLower == "crunchyroll" || nextPartLower == "katsurasub" ||
					nextPartLower == "full" || nextPartLower == "полные" ||
					nextPartLower == "forced" || nextPartLower == "форсированные"

				if !isServiceFolder && strings.TrimSpace(nextClean) != "" {
					suffixParts = append(suffixParts, nextClean)
				}
			}
		}

		// 3. Проверяем служебные папки (например, "Надписи" или "Signs")
		cleanPart := strings.Trim(part, "[]()")
		cleanPartLower := strings.ToLower(cleanPart)
		isServiceFolder := cleanPartLower == "signs" || cleanPartLower == "надписи" ||
			cleanPartLower == "crunchyroll" || cleanPartLower == "katsurasub" ||
			cleanPartLower == "full" || cleanPartLower == "полные" ||
			cleanPartLower == "forced" || cleanPartLower == "форсированные"

		if isServiceFolder && strings.TrimSpace(cleanPart) != "" {
			suffixParts = append(suffixParts, cleanPart)
		}
	}

	cleanBase := bracketsRegex.ReplaceAllString(base, "")
	cleanBase = strings.TrimSpace(cleanBase)

	fileParts := strings.Split(cleanBase, ".")
	if len(fileParts) > 1 {
		for i := 1; i < len(fileParts); i++ {
			part := strings.TrimSpace(fileParts[i])
			if part != "" {
				cleanedPart := langSuffixRegex.ReplaceAllString(part, "")
				cleanedPart = strings.Trim(cleanedPart, "-_ ")
				if cleanedPart != "" {
					suffixParts = append(suffixParts, cleanedPart)
				} else if langSuffixRegex.MatchString(part) {
					lower := strings.ToLower(part)
					if lower == "signs" || lower == "full" || lower == "dub" {
						suffixParts = append(suffixParts, lower)
					}
				}
			}
		}
	}

	uniqueParts := make([]string, 0)
	seen := make(map[string]bool)
	for _, p := range suffixParts {
		pLower := strings.ToLower(p)
		if !seen[pLower] {
			seen[pLower] = true
			uniqueParts = append(uniqueParts, p)
		}
	}

	if len(uniqueParts) > 0 {
		return strings.Join(uniqueParts, ".")
	}

	return ""
}

// AlignPartEpisodes сдвигает номера серий для Part 2, Part 3 и т.д., если их нумерация начинается с 1
func AlignPartEpisodes(files []*EpisodeFile) {
	filesBySeason := make(map[int][]*EpisodeFile)
	for _, f := range files {
		filesBySeason[f.SeasonNum] = append(filesBySeason[f.SeasonNum], f)
	}

	for _, seasonFiles := range filesBySeason {
		filesByPart := make(map[int][]*EpisodeFile)
		var parts []int
		for _, f := range seasonFiles {
			if _, exists := filesByPart[f.PartNum]; !exists {
				parts = append(parts, f.PartNum)
			}
			filesByPart[f.PartNum] = append(filesByPart[f.PartNum], f)
		}

		if len(parts) <= 1 {
			continue
		}

		sort.Ints(parts)

		maxEpOfPreviousParts := 0

		for _, part := range parts {
			pFiles := filesByPart[part]
			minEp := 99999
			maxEp := -1
			hasEps := false

			for _, f := range pFiles {
				if f.EpisodeNum != -1 {
					hasEps = true
					if f.EpisodeNum < minEp {
						minEp = f.EpisodeNum
					}
					if f.EpisodeNum > maxEp {
						maxEp = f.EpisodeNum
					}
				}
			}

			if !hasEps {
				continue
			}

			if maxEpOfPreviousParts > 0 && minEp <= maxEpOfPreviousParts {
				offset := maxEpOfPreviousParts
				for _, f := range pFiles {
					if f.EpisodeNum != -1 {
						f.EpisodeNum += offset
					}
				}
				maxEp += offset
			}

			if maxEp > maxEpOfPreviousParts {
				maxEpOfPreviousParts = maxEp
			}
		}
	}
}

// AlignEpisodeNumbers выравнивает сквозные номера серий аудио и субтитров под видеофайлы
func AlignEpisodeNumbers(show *AnimeShow) {
	filesBySeason := make(map[int][]*EpisodeFile)
	for _, f := range show.Files {
		filesBySeason[f.SeasonNum] = append(filesBySeason[f.SeasonNum], f)
	}

	for season, seasonFiles := range filesBySeason {
		videoEpisodes := make(map[int]bool)
		minVideoEp := 9999
		maxVideoEp := -1
		videoCount := 0

		for _, f := range seasonFiles {
			if f.Type == TypeVideo && f.EpisodeNum != -1 {
				videoEpisodes[f.EpisodeNum] = true
				if f.EpisodeNum < minVideoEp {
					minVideoEp = f.EpisodeNum
				}
				if f.EpisodeNum > maxVideoEp {
					maxVideoEp = f.EpisodeNum
				}
				videoCount++
			}
		}

		if videoCount == 0 {
			continue
		}

		// Сценарий А: Сквозная нумерация ВСЕХ файлов в сезоне S > 1.
		// Например: видео 13..25 и сабы 13..25 во 2-м сезоне.
		// Если минимальный эпизод видео больше 1, значит весь сезон сдвинут!
		if season > 1 && minVideoEp > 1 && minVideoEp != 9999 {
			offset := minVideoEp - 1
			for _, f := range seasonFiles {
				if f.EpisodeNum != -1 {
					f.EpisodeNum = f.EpisodeNum - offset
				}
			}
			continue // Выравнивание завершено для этого сезона
		}

		// Сценарий Б: Сквозная нумерация только аудио/сабов (видео 1..13, а сабы 13..25).
		nonVideoEps := make(map[int]bool)
		minNonVideoEp := 9999
		for _, f := range seasonFiles {
			if f.Type != TypeVideo && f.EpisodeNum != -1 {
				if _, exists := videoEpisodes[f.EpisodeNum]; !exists {
					nonVideoEps[f.EpisodeNum] = true
					if f.EpisodeNum < minNonVideoEp {
						minNonVideoEp = f.EpisodeNum
					}
				}
			}
		}

		if len(nonVideoEps) == 0 || minNonVideoEp == 9999 {
			continue
		}

		offset := minNonVideoEp - minVideoEp
		if offset <= 0 {
			continue
		}

		validOffset := true
		for ep := range nonVideoEps {
			targetEp := ep - offset
			if !videoEpisodes[targetEp] {
				validOffset = false
				break
			}
		}

		if validOffset {
			for _, f := range seasonFiles {
				if f.Type != TypeVideo && f.EpisodeNum != -1 {
					if _, exists := nonVideoEps[f.EpisodeNum]; exists {
						f.EpisodeNum = f.EpisodeNum - offset
					}
				}
			}
		}
	}
}

// ParseShowFolder парсит всю раздачу целиком, группируя файлы
func ParseShowFolder(folderName string, relFiles []string, languageMapping map[string]string) *AnimeShow {
	defaultSeason := ExtractSeason(folderName)
	defaultPart := ExtractPart(folderName)

	show := &AnimeShow{
		OriginalName: folderName,
		CleanedName:  CleanShowName(folderName),
		Season:       defaultSeason,
		Part:         defaultPart,
		IsMovie:      IsMovieFolder(folderName),
		Files:        make([]*EpisodeFile, 0),
	}

	// Сначала временно парсим все файлы с дефолтным сезоном и частью
	var tempFiles []*EpisodeFile
	for _, f := range relFiles {
		epFile := ParseEpisodeFile(f, defaultSeason, defaultPart, languageMapping)
		if epFile != nil {
			tempFiles = append(tempFiles, epFile)
		}
	}

	// Попробуем определить более точное имя шоу по именам видеофайлов.
	// Ищем самый популярный очищенный паттерн имени видеофайлов.
	cleanedNamesCount := make(map[string]int)
	for _, f := range tempFiles {
		if f.Type == TypeVideo {
			nameFromFile := ExtractShowNameFromFile(filepath.Base(f.SourcePath))
			if nameFromFile != "" {
				cleanedNamesCount[nameFromFile]++
			}
		}
	}

	bestCleanedName := ""
	maxCount := 0
	for name, count := range cleanedNamesCount {
		if count > maxCount {
			maxCount = count
			bestCleanedName = name
		}
	}

	// Используем имя из файлов только в том случае, если исходное имя папки пустое
	// или если имя из файлов является чистым префиксом папки (например, удаляет "Season 2" из названия папки).
	if bestCleanedName != "" {
		normFolder := normalizeForCompare(show.CleanedName)
		normFile := normalizeForCompare(bestCleanedName)
		if normFolder == "" {
			show.CleanedName = bestCleanedName
		} else if normFile != "" && strings.HasPrefix(normFolder, normFile) && len(normFile) < len(normFolder) {
			show.CleanedName = bestCleanedName
		}
	}

	show.Files = tempFiles

	// Выравниваем сквозные номера эпизодов для частей/куров
	AlignPartEpisodes(show.Files)

	// Выравниваем сквозные номера эпизодов аудио и субтитров под видео
	AlignEpisodeNumbers(show)

	return show
}

func normalizeForCompare(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "ё", "е")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, "_", "")
	s = strings.ReplaceAll(s, ".", "")
	return s
}
