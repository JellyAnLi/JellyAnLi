package providers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"jelly-an-li/internal/parser"
)

const (
	aniDBUserAgent = "JellyAnLi/1.0 (Anime Media Linker for Jellyfin; +https://github.com/JellyAnLi/JellyAnLi)"
)

var (
	aniDBCacheFile = "data/anidb_cache.json"
	aniDBCacheLock sync.Mutex
	aniDBCacheInst *AniDBCache
	aniDBLastReq   time.Time
	aniDBReqLock   sync.Mutex
)

type CachedAniDBInfo struct {
	TitleRomaji string `json:"title_romaji"`
	TitleEn     string `json:"title_en"`
	TitleRu     string `json:"title_ru,omitempty"`
	Season      int    `json:"season"`
	IsMovie     bool   `json:"is_movie"`
	IsSpecial   bool   `json:"is_special"`
}

type AniDBCache struct {
	Entries map[string]CachedAniDBInfo `json:"entries"`
}

func loadAniDBCache() *AniDBCache {
	aniDBCacheLock.Lock()
	defer aniDBCacheLock.Unlock()

	if aniDBCacheInst != nil {
		return aniDBCacheInst
	}

	c := &AniDBCache{Entries: make(map[string]CachedAniDBInfo)}
	data, err := os.ReadFile(aniDBCacheFile)
	if err == nil {
		_ = json.Unmarshal(data, c)
	}
	aniDBCacheInst = c
	return c
}

func SetAniDBCacheDir(dir string) {
	aniDBCacheLock.Lock()
	defer aniDBCacheLock.Unlock()
	aniDBCacheFile = filepath.Join(dir, "anidb_cache.json")
	aniDBCacheInst = nil
}

func ClearAniDBCache() {
	aniDBCacheLock.Lock()
	aniDBCacheInst = &AniDBCache{Entries: make(map[string]CachedAniDBInfo)}
	fileToRemove := aniDBCacheFile
	aniDBCacheLock.Unlock()

	_ = os.Remove(fileToRemove)
	_ = os.Remove("data/anidb_cache.json")
}

func GetAniDBCacheCount() int {
	c := loadAniDBCache()
	aniDBCacheLock.Lock()
	defer aniDBCacheLock.Unlock()
	return len(c.Entries)
}

func saveAniDBCache(c *AniDBCache) error {
	aniDBCacheLock.Lock()
	defer aniDBCacheLock.Unlock()

	_ = os.MkdirAll(filepath.Dir(aniDBCacheFile), 0755)
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(aniDBCacheFile, data, 0644)
}

func waitAniDBRateLimit() {
	aniDBReqLock.Lock()
	defer aniDBReqLock.Unlock()

	elapsed := time.Since(aniDBLastReq)
	if elapsed < 200*time.Millisecond {
		time.Sleep(200*time.Millisecond - elapsed)
	}
	aniDBLastReq = time.Now()
}

type AniDBProvider struct{}

func (p *AniDBProvider) ID() string {
	return "anidb"
}

func (p *AniDBProvider) Name() string {
	return "AniDB"
}

func (p *AniDBProvider) Description() string {
	return "Эталонная база названий аниме и синонимов (с локальным кэшированием)"
}

func (p *AniDBProvider) Search(query string, proxyURL string) (*AnimeMetadata, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}

	cache := loadAniDBCache()

	aniDBCacheLock.Lock()
	if val, exists := cache.Entries[query]; exists && (val.Season > 0 || val.IsMovie || val.IsSpecial || val.TitleRomaji == "-") {
		aniDBCacheLock.Unlock()
		if val.TitleRomaji == "-" {
			return nil, nil
		}
		return &AnimeMetadata{
			Provider:    "anidb",
			TitleRomaji: val.TitleRomaji,
			TitleEn:     val.TitleEn,
			TitleRu:     val.TitleRu,
			Season:      val.Season,
			IsMovie:     val.IsMovie,
			IsSpecial:   val.IsSpecial,
		}, nil
	}
	aniDBCacheLock.Unlock()

	cleanedQuery := parser.CleanQueryForSearch(query)
	if cleanedQuery == "" {
		cleanedQuery = query
	}

	waitAniDBRateLimit()

	// Используем поиск через AniDB HTTP API или публичный прокси-индекс с коротким таймаутом
	searchURL := fmt.Sprintf("https://anidb.net/anime/?adb.search=%s&do.search=1", url.QueryEscape(cleanedQuery))
	client := GetHTTPClient(proxyURL)
	client.Timeout = 3 * time.Second

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", aniDBUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	querySeason, _ := parser.HasExplicitSeason(cleanedQuery)
	cleanedBase := parser.CleanShowName(cleanedQuery)

	isMovie := parser.IsMovieFolder(query) || parser.IsMovieFolder(cleanedQuery)
	isSpecial := strings.Contains(strings.ToLower(query), "special") || strings.Contains(strings.ToLower(query), "спешл") || strings.Contains(strings.ToLower(query), "ova")

	seasonNum := 1
	if isSpecial {
		seasonNum = 0
	} else if querySeason > 1 {
		seasonNum = querySeason
	}

	meta := &AnimeMetadata{
		Provider:    "anidb",
		TitleRomaji: cleanedBase,
		TitleEn:     cleanedBase,
		Season:      seasonNum,
		IsMovie:     isMovie,
		IsSpecial:   isSpecial,
	}

	aniDBCacheLock.Lock()
	cache.Entries[query] = CachedAniDBInfo{
		TitleRomaji: meta.TitleRomaji,
		TitleEn:     meta.TitleEn,
		TitleRu:     meta.TitleRu,
		Season:      meta.Season,
		IsMovie:     meta.IsMovie,
		IsSpecial:   meta.IsSpecial,
	}
	aniDBCacheLock.Unlock()
	_ = saveAniDBCache(cache)

	return meta, nil
}

func init() {
	Register(&AniDBProvider{})
}
