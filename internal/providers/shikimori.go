package providers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"jelly-an-li/internal/parser"
)

const (
	shikimoriHost      = "https://shikimori.one"
	shikimoriUserAgent = "JellyAnLi/1.0 (Anime Media Linker for Jellyfin; +https://github.com/JellyAnLi/JellyAnLi)"
)

var (
	shikimoriCacheFile = "data/shikimori_cache.json"
	shikimoriCacheLock sync.Mutex
	shikimoriCacheInst *ShikimoriCache
	shikimoriLastReq   time.Time
	shikimoriReqLock   sync.Mutex

	shikimoriRuSeasonRegexes = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\s+(?:[2-9]|\d{2,})(?:\.\s*часть\s*\d+)?$`),
		regexp.MustCompile(`(?i)\s+(?:тв|tv)\s*[-_]?\s*\d+$`),
		regexp.MustCompile(`(?i)\s+сезон\s*\d+$`),
		regexp.MustCompile(`(?i)\s*\[(?:тв|tv)\s*[-_]?\s*\d+\]$`),
		regexp.MustCompile(`(?i)\s*\(сезон\s*\d+\)$`),
		regexp.MustCompile(`(?i)\s*\(тв\s*[-_]?\s*\d+\)$`),
		regexp.MustCompile(`(?i)\s*:\s*часть\s*(?:[2-9]|\d{2,}|ii|iii|iv|v)$`),
		regexp.MustCompile(`(?i)\s+часть\s*(?:[2-9]|\d{2,}|ii|iii|iv|v)$`),
	}
)

func cleanRussianSeason(name string) string {
	cleaned := name
	for _, re := range shikimoriRuSeasonRegexes {
		cleaned = re.ReplaceAllString(cleaned, "")
	}
	return strings.TrimSpace(cleaned)
}

type ShikimoriAnime struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Russian string `json:"russian"`
	Kind    string `json:"kind"`
}

type ShikimoriNode struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Russian string `json:"russian"`
	Kind    string `json:"kind"`
	Date    int64  `json:"date"`
}

type ShikimoriFranchise struct {
	Nodes []ShikimoriNode `json:"nodes"`
}

type CachedShikimoriInfo struct {
	Russian   string `json:"russian"`
	Romaji    string `json:"romaji,omitempty"`
	Season    int    `json:"season"`
	IsMovie   bool   `json:"is_movie"`
	IsSpecial bool   `json:"is_special"`
}

type ShikimoriCache struct {
	Translations map[string]CachedShikimoriInfo `json:"translations"`
}

func (c *ShikimoriCache) UnmarshalJSON(data []byte) error {
	var raw struct {
		Translations map[string]json.RawMessage `json:"translations"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	c.Translations = make(map[string]CachedShikimoriInfo)
	for k, v := range raw.Translations {
		var info CachedShikimoriInfo
		if err := json.Unmarshal(v, &info); err == nil && info.Russian != "" {
			c.Translations[k] = info
		} else {
			var s string
			if err := json.Unmarshal(v, &s); err == nil {
				c.Translations[k] = CachedShikimoriInfo{Russian: s, Season: 0}
			}
		}
	}
	return nil
}

func SetShikimoriCacheForTest(translations map[string]CachedShikimoriInfo) func() {
	shikimoriCacheLock.Lock()
	old := shikimoriCacheInst
	shikimoriCacheInst = &ShikimoriCache{Translations: translations}
	shikimoriCacheLock.Unlock()

	return func() {
		shikimoriCacheLock.Lock()
		shikimoriCacheInst = old
		shikimoriCacheLock.Unlock()
	}
}

func loadShikimoriCache() *ShikimoriCache {
	shikimoriCacheLock.Lock()
	defer shikimoriCacheLock.Unlock()

	if shikimoriCacheInst != nil {
		return shikimoriCacheInst
	}

	c := &ShikimoriCache{Translations: make(map[string]CachedShikimoriInfo)}
	data, err := os.ReadFile(shikimoriCacheFile)
	if err == nil {
		_ = json.Unmarshal(data, c)
	}
	shikimoriCacheInst = c
	return c
}

func saveShikimoriCache(c *ShikimoriCache) error {
	shikimoriCacheLock.Lock()
	defer shikimoriCacheLock.Unlock()

	_ = os.MkdirAll(filepath.Dir(shikimoriCacheFile), 0755)
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(shikimoriCacheFile, data, 0644)
}

func waitShikimoriRateLimit() {
	shikimoriReqLock.Lock()
	defer shikimoriReqLock.Unlock()

	elapsed := time.Since(shikimoriLastReq)
	if elapsed < 350*time.Millisecond {
		time.Sleep(350*time.Millisecond - elapsed)
	}
	shikimoriLastReq = time.Now()
}

type ShikimoriProvider struct{}

func (p *ShikimoriProvider) ID() string {
	return "shikimori"
}

func (p *ShikimoriProvider) Name() string {
	return "Shikimori"
}

func (p *ShikimoriProvider) Description() string {
	return "Русскоязычная база аниме с поддержкой франшиз и связей сезонов"
}

func (p *ShikimoriProvider) Search(query string, proxyURL string) (*AnimeMetadata, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}

	cache := loadShikimoriCache()

	shikimoriCacheLock.Lock()
	if val, exists := cache.Translations[query]; exists && (val.Season > 0 || val.IsMovie || val.IsSpecial || val.Russian == "-") {
		shikimoriCacheLock.Unlock()
		if val.Russian == "-" {
			return nil, nil
		}
		return &AnimeMetadata{
			Provider:    "shikimori",
			TitleRu:     val.Russian,
			TitleRomaji: val.Romaji,
			Season:      val.Season,
			IsMovie:     val.IsMovie,
			IsSpecial:   val.IsSpecial,
		}, nil
	}
	shikimoriCacheLock.Unlock()

	cleanedQuery := parser.CleanQueryForSearch(query)
	if cleanedQuery == "" {
		cleanedQuery = query
	}

	waitShikimoriRateLimit()

	apiUrl := fmt.Sprintf("%s/api/animes?search=%s&limit=5", shikimoriHost, url.QueryEscape(cleanedQuery))
	client := GetHTTPClient(proxyURL)

	var resp *http.Response
	for attempt := 0; attempt < 2; attempt++ {
		req, err := http.NewRequest("GET", apiUrl, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", shikimoriUserAgent)
		req.Header.Set("Accept", "application/json")

		resp, err = client.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			break
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(1 * time.Second)
	}

	if resp == nil || resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("shikimori api error")
	}

	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}

	var results []ShikimoriAnime
	if err := json.Unmarshal(body, &results); err != nil {
		return nil, err
	}

	if len(results) == 0 {
		shikimoriCacheLock.Lock()
		cache.Translations[query] = CachedShikimoriInfo{Russian: "-", Romaji: "-", Season: 1}
		shikimoriCacheLock.Unlock()
		_ = saveShikimoriCache(cache)
		return nil, nil
	}

	querySeason, hasQuerySeason := parser.HasExplicitSeason(cleanedQuery)
	if !hasQuerySeason {
		querySeason, hasQuerySeason = parser.HasExplicitSeason(query)
	}

	queryCleanBase := parser.CleanShowName(cleanedQuery)
	normQueryBase := normalizeTitleString(queryCleanBase)
	normQueryCompact := strings.ReplaceAll(normQueryBase, " ", "")
	queryWords := strings.Fields(normQueryBase)

	var significantQueryWords []string
	for _, w := range queryWords {
		if !isStopWord(w) {
			significantQueryWords = append(significantQueryWords, w)
		}
	}

	isQuerySpecial := strings.Contains(strings.ToLower(query), "special") || strings.Contains(strings.ToLower(query), "спешл") || strings.Contains(strings.ToLower(query), "ova") || strings.Contains(strings.ToLower(query), "ова")
	isQueryMovie := parser.IsMovieFolder(query) || parser.IsMovieFolder(cleanedQuery)

	targetAnime := results[0]
	bestScore := -999

	for _, res := range results {
		resName := strings.ToLower(res.Name)
		resRus := strings.ToLower(res.Russian)
		resKind := strings.ToLower(res.Kind)

		isCandMovie := resKind == "movie" || strings.Contains(resKind, "фильм")
		isCandSpecial := resKind == "special" || resKind == "ova" || strings.Contains(resKind, "спец")
		isCandOna := resKind == "ona"
		isCandTv := resKind == "tv" || strings.Contains(resKind, "tv") || strings.Contains(resKind, "сериал")

		candSeasonName, hasCandSeasonName := parser.HasExplicitSeason(res.Name)
		candSeasonRus, hasCandSeasonRus := parser.HasExplicitSeason(res.Russian)
		candSeason := 1
		hasCandSeason := false
		if hasCandSeasonName {
			candSeason = candSeasonName
			hasCandSeason = true
		} else if hasCandSeasonRus {
			candSeason = candSeasonRus
			hasCandSeason = true
		}

		score := 0

		candCleanBase := normalizeTitleString(parser.CleanShowName(res.Name))
		candCleanRusBase := normalizeTitleString(cleanRussianSeason(res.Russian))
		candCompactName := strings.ReplaceAll(normalizeTitleString(res.Name), " ", "")
		candCompactRus := strings.ReplaceAll(normalizeTitleString(res.Russian), " ", "")

		// 1. Точное совпадение названий или без пробелов (Mahoutei no Ken == Mahou Tei no Ken)
		if normQueryBase != "" && (candCleanBase == normQueryBase || candCleanRusBase == normQueryBase) {
			score += 35
		} else if normQueryCompact != "" && (candCompactName == normQueryCompact || candCompactRus == normQueryCompact) {
			score += 35
		} else if normQueryBase != "" && (strings.HasPrefix(candCleanBase, normQueryBase) || strings.HasPrefix(normQueryBase, candCleanBase) || strings.HasPrefix(candCleanRusBase, normQueryBase) || strings.HasPrefix(normQueryBase, candCleanRusBase)) {
			score += 20
		} else if normQueryCompact != "" && (strings.HasPrefix(candCompactName, normQueryCompact) || strings.HasPrefix(normQueryCompact, candCompactName) || strings.HasPrefix(candCompactRus, normQueryCompact) || strings.HasPrefix(normQueryCompact, candCompactRus)) {
			score += 20
		}

		// 2. Проверка значимых слов
		for idx, w := range significantQueryWords {
			if strings.Contains(resName, w) || strings.Contains(resRus, w) {
				score += 6
				if idx == 0 {
					score += 8 // Первое слово названия — самое важное
				} else if idx == 1 {
					score += 4
				}
			}
		}

		// Если первое значимое слово полностью отсутствует в названии кандидата — штрафуем
		if len(significantQueryWords) > 0 {
			firstWord := significantQueryWords[0]
			if !strings.Contains(resName, firstWord) && !strings.Contains(resRus, firstWord) {
				score -= 25
			}
		}

		// 3. Сезоны
		if hasQuerySeason && querySeason > 1 {
			if hasCandSeason && candSeason == querySeason {
				score += 20
			} else if hasCandSeason && candSeason != querySeason {
				score -= 15
			}
		} else if hasQuerySeason && querySeason == 1 {
			if hasCandSeason && candSeason > 1 {
				score -= 10
			}
		}

		// 4. Формат
		if isQueryMovie {
			if isCandMovie {
				score += 15
			} else {
				score -= 10
			}
		} else if isQuerySpecial {
			if isCandSpecial || isCandOna {
				score += 15
			}
		} else {
			if isCandTv {
				score += 4
			} else if isCandSpecial {
				score -= 10
			}
		}

		if score > bestScore {
			bestScore = score
			targetAnime = res
		}
	}

	russianName := targetAnime.Russian
	if russianName == "" {
		russianName = targetAnime.Name
	}
	russianName = cleanRussianSeason(russianName)

	romajiName := parser.CleanShowName(targetAnime.Name)
	if romajiName == "" {
		romajiName = targetAnime.Name
	}

	targetKind := strings.ToLower(targetAnime.Kind)
	isMovie := targetKind == "movie" || strings.Contains(targetKind, "фильм")
	isSpecial := targetKind == "special" || targetKind == "ova" || strings.Contains(targetKind, "спец")

	seasonNum := 1
	if isSpecial {
		seasonNum = 0
	}

	if !isSpecial {
		waitShikimoriRateLimit()
		franchiseUrl := fmt.Sprintf("%s/api/animes/%d/franchise", shikimoriHost, targetAnime.ID)
		fReq, err := http.NewRequest("GET", franchiseUrl, nil)
		if err == nil {
			fReq.Header.Set("User-Agent", shikimoriUserAgent)
			fReq.Header.Set("Accept", "application/json")

			fResp, err := client.Do(fReq)
			if err == nil && fResp.StatusCode == http.StatusOK {
				fBody, err := io.ReadAll(fResp.Body)
				fResp.Body.Close()
				if err == nil {
					var franchise ShikimoriFranchise
					if err := json.Unmarshal(fBody, &franchise); err == nil {
						hasTV := false
						for _, node := range franchise.Nodes {
							k := strings.ToLower(node.Kind)
							if k == "tv" || strings.Contains(k, "tv") || strings.Contains(strings.ToLower(node.Kind), "сериал") {
								hasTV = true
								break
							}
						}

						var tvNodes []ShikimoriNode
						for _, node := range franchise.Nodes {
							k := strings.ToLower(node.Kind)
							isTV := k == "tv" || strings.Contains(k, "tv") || strings.Contains(strings.ToLower(node.Kind), "сериал")
							if isTV || (!hasTV && k == "ona") {
								tvNodes = append(tvNodes, node)
							}
						}
						sort.Slice(tvNodes, func(i, j int) bool {
							return tvNodes[i].Date < tvNodes[j].Date
						})

						if !isMovie {
							for idx, node := range tvNodes {
								if node.ID == targetAnime.ID {
									seasonNum = idx + 1
									if len(tvNodes) > 0 {
										rootClean := cleanRussianSeason(tvNodes[0].Name)
										if rootClean != "" {
											russianName = rootClean
										}
									}
									break
								}
							}
						} else {
							// Для фильма: если во франшизе есть основной сериал, используем его корневое русское название для папки
							if len(tvNodes) > 0 {
								rootClean := cleanRussianSeason(tvNodes[0].Name)
								if rootClean != "" {
									russianName = rootClean
								}
							}
						}
					}
				}
			}
		}
	}

	meta := &AnimeMetadata{
		Provider:    "shikimori",
		TitleRu:     russianName,
		TitleRomaji: romajiName,
		Season:      seasonNum,
		IsMovie:     isMovie,
		IsSpecial:   isSpecial,
	}

	shikimoriCacheLock.Lock()
	cache.Translations[query] = CachedShikimoriInfo{
		Russian:   meta.TitleRu,
		Romaji:    meta.TitleRomaji,
		Season:    meta.Season,
		IsMovie:   meta.IsMovie,
		IsSpecial: meta.IsSpecial,
	}
	shikimoriCacheLock.Unlock()
	_ = saveShikimoriCache(cache)

	return meta, nil
}

func normalizeTitleString(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "ё", "е")
	var sb strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || (r >= 'а' && r <= 'я') || r == ' ' {
			sb.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(sb.String()), " ")
}

func init() {
	Register(&ShikimoriProvider{})
}
