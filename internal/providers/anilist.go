package providers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"jelly-an-li/internal/parser"
)

const (
	aniListEndpoint  = "https://graphql.anilist.co"
	aniListUserAgent = "JellyAnLi/1.0 (Anime Media Linker for Jellyfin; +https://github.com/JellyAnLi/JellyAnLi)"
)

var (
	aniListCacheFile = "data/anilist_cache.json"
	aniListCacheLock sync.Mutex
	aniListCacheInst *AniListCache
	aniListLastReq   time.Time
	aniListReqLock   sync.Mutex
)

type AniListMediaTitle struct {
	Romaji  string `json:"romaji"`
	English string `json:"english"`
	Native  string `json:"native"`
}

type AniListRelationNode struct {
	ID     int                `json:"id"`
	Format string             `json:"format"`
	Title  *AniListMediaTitle `json:"title"`
}

type AniListRelationEdge struct {
	RelationType string               `json:"relationType"`
	Node         *AniListRelationNode `json:"node"`
}

type AniListMediaRelations struct {
	Edges []*AniListRelationEdge `json:"edges"`
}

type AniListMedia struct {
	ID        int                    `json:"id"`
	Title     *AniListMediaTitle     `json:"title"`
	Synonyms  []string               `json:"synonyms"`
	Format    string                 `json:"format"`
	Episodes  *int                   `json:"episodes"`
	Relations *AniListMediaRelations `json:"relations"`
}

type AniListGraphQLResponse struct {
	Data struct {
		Page struct {
			Media []*AniListMedia `json:"media"`
		} `json:"Page"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

type CachedAniListInfo struct {
	TitleRomaji string `json:"title_romaji"`
	TitleEn     string `json:"title_en"`
	Season      int    `json:"season"`
	IsMovie     bool   `json:"is_movie"`
	IsSpecial   bool   `json:"is_special"`
}

type AniListCache struct {
	Entries map[string]CachedAniListInfo `json:"entries"`
}

func loadAniListCache() *AniListCache {
	aniListCacheLock.Lock()
	defer aniListCacheLock.Unlock()

	if aniListCacheInst != nil {
		return aniListCacheInst
	}

	c := &AniListCache{Entries: make(map[string]CachedAniListInfo)}
	data, err := os.ReadFile(aniListCacheFile)
	if err == nil {
		_ = json.Unmarshal(data, c)
	}
	aniListCacheInst = c
	return c
}

func saveAniListCache(c *AniListCache) error {
	aniListCacheLock.Lock()
	defer aniListCacheLock.Unlock()

	_ = os.MkdirAll(filepath.Dir(aniListCacheFile), 0755)
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(aniListCacheFile, data, 0644)
}

func waitAniListRateLimit() {
	aniListReqLock.Lock()
	defer aniListReqLock.Unlock()

	elapsed := time.Since(aniListLastReq)
	if elapsed < 500*time.Millisecond {
		time.Sleep(500*time.Millisecond - elapsed)
	}
	aniListLastReq = time.Now()
}

type AniListProvider struct{}

func (p *AniListProvider) ID() string {
	return "anilist"
}

func (p *AniListProvider) Name() string {
	return "AniList"
}

func (p *AniListProvider) Description() string {
	return "Открытая мировая база аниме с поддержкой Romaji, English и форматов"
}

func (p *AniListProvider) Search(query string, proxyURL string) (*AnimeMetadata, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}

	cache := loadAniListCache()

	aniListCacheLock.Lock()
	if val, exists := cache.Entries[query]; exists && (val.Season > 0 || val.IsMovie || val.IsSpecial || val.TitleRomaji == "-") {
		aniListCacheLock.Unlock()
		if val.TitleRomaji == "-" {
			return nil, nil
		}
		return &AnimeMetadata{
			Provider:    "anilist",
			TitleRomaji: val.TitleRomaji,
			TitleEn:     val.TitleEn,
			Season:      val.Season,
			IsMovie:     val.IsMovie,
			IsSpecial:   val.IsSpecial,
		}, nil
	}
	aniListCacheLock.Unlock()

	cleanedQuery := parser.CleanQueryForSearch(query)
	if cleanedQuery == "" {
		cleanedQuery = query
	}

	waitAniListRateLimit()

	graphqlQuery := `
query ($search: String) {
  Page(page: 1, perPage: 5) {
    media(search: $search, type: ANIME) {
      id
      title {
        romaji
        english
        native
      }
      synonyms
      format
      episodes
      relations {
        edges {
          relationType
          node {
            id
            format
            title {
              romaji
            }
          }
        }
      }
    }
  }
}
`

	payload := map[string]interface{}{
		"query": graphqlQuery,
		"variables": map[string]interface{}{
			"search": cleanedQuery,
		},
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	client := GetHTTPClient(proxyURL)

	req, err := http.NewRequest("POST", aniListEndpoint, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", aniListUserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("anilist api error status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var gqlResp AniListGraphQLResponse
	if err := json.Unmarshal(body, &gqlResp); err != nil {
		return nil, err
	}

	results := gqlResp.Data.Page.Media
	if len(results) == 0 {
		aniListCacheLock.Lock()
		cache.Entries[query] = CachedAniListInfo{TitleRomaji: "-", TitleEn: "-", Season: 1}
		aniListCacheLock.Unlock()
		_ = saveAniListCache(cache)
		return nil, nil
	}

	querySeason, hasQuerySeason := parser.HasExplicitSeason(cleanedQuery)
	if !hasQuerySeason {
		querySeason, hasQuerySeason = parser.HasExplicitSeason(query)
	}

	queryCleanBase := parser.CleanShowName(cleanedQuery)
	normQueryBase := normalizeTitleString(queryCleanBase)
	queryWords := strings.Fields(normQueryBase)

	isQuerySpecial := strings.Contains(strings.ToLower(query), "special") || strings.Contains(strings.ToLower(query), "спешл") || strings.Contains(strings.ToLower(query), "ova") || strings.Contains(strings.ToLower(query), "ова")
	isQueryMovie := parser.IsMovieFolder(query) || parser.IsMovieFolder(cleanedQuery)

	targetMedia := results[0]
	bestScore := -999

	for _, media := range results {
		var romaji, english string
		if media.Title != nil {
			romaji = media.Title.Romaji
			english = media.Title.English
		}

		resKind := strings.ToUpper(media.Format)
		isCandMovie := resKind == "MOVIE"
		isCandSpecial := resKind == "SPECIAL" || resKind == "OVA" || resKind == "ONA"
		isCandTv := resKind == "TV" || resKind == "TV_SHORT"

		candSeasonName, hasCandSeasonName := parser.HasExplicitSeason(romaji)
		candSeasonEn, hasCandSeasonEn := parser.HasExplicitSeason(english)
		candSeason := 1
		hasCandSeason := false
		if hasCandSeasonName {
			candSeason = candSeasonName
			hasCandSeason = true
		} else if hasCandSeasonEn {
			candSeason = candSeasonEn
			hasCandSeason = true
		}

		score := 0

		candCleanBase := normalizeTitleString(parser.CleanShowName(romaji))
		candCleanEnBase := normalizeTitleString(parser.CleanShowName(english))

		if normQueryBase != "" && (candCleanBase == normQueryBase || candCleanEnBase == normQueryBase) {
			score += 15
		} else if normQueryBase != "" && (strings.HasPrefix(candCleanBase, normQueryBase) || strings.HasPrefix(normQueryBase, candCleanBase)) {
			score += 10
		}

		for idx, w := range queryWords {
			if len(w) >= 2 {
				if strings.Contains(strings.ToLower(romaji), w) || strings.Contains(strings.ToLower(english), w) {
					score += 3
					if idx < 2 {
						score += 1
					}
				}
			}
		}

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

		if isQueryMovie {
			if isCandMovie {
				score += 15
			} else {
				score -= 10
			}
		} else if isQuerySpecial {
			if isCandSpecial {
				score += 15
			}
		} else {
			if isCandTv {
				score += 8
			} else if isCandSpecial {
				score -= 10
			} else if isCandMovie {
				score -= 8
			}
		}

		if score > bestScore {
			bestScore = score
			targetMedia = media
		}
	}

	var titleRomaji, titleEn string
	if targetMedia.Title != nil {
		titleRomaji = parser.CleanShowName(targetMedia.Title.Romaji)
		if titleRomaji == "" {
			titleRomaji = targetMedia.Title.Romaji
		}
		titleEn = parser.CleanShowName(targetMedia.Title.English)
		if titleEn == "" {
			titleEn = targetMedia.Title.English
		}
	}

	format := strings.ToUpper(targetMedia.Format)
	isMovie := format == "MOVIE"
	isSpecial := format == "SPECIAL" || format == "OVA"

	seasonNum := 1
	if isSpecial {
		seasonNum = 0
	} else if !isMovie {
		// Вычисляем сезон по цепочке relations
		if targetMedia.Relations != nil {
			prequelCount := 0
			for _, edge := range targetMedia.Relations.Edges {
				if edge != nil && strings.ToUpper(edge.RelationType) == "PREQUEL" {
					if edge.Node != nil && (strings.ToUpper(edge.Node.Format) == "TV" || strings.ToUpper(edge.Node.Format) == "TV_SHORT") {
						prequelCount++
					}
				}
			}
			if prequelCount > 0 {
				seasonNum = prequelCount + 1
			}
		}
	}

	meta := &AnimeMetadata{
		Provider:    "anilist",
		TitleRomaji: titleRomaji,
		TitleEn:     titleEn,
		Season:      seasonNum,
		IsMovie:     isMovie,
		IsSpecial:   isSpecial,
	}

	aniListCacheLock.Lock()
	cache.Entries[query] = CachedAniListInfo{
		TitleRomaji: meta.TitleRomaji,
		TitleEn:     meta.TitleEn,
		Season:      meta.Season,
		IsMovie:     meta.IsMovie,
		IsSpecial:   meta.IsSpecial,
	}
	aniListCacheLock.Unlock()
	_ = saveAniListCache(cache)

	return meta, nil
}

func init() {
	Register(&AniListProvider{})
}
