package providers

import (
	"strings"
	"sync"
)

// AnimeMetadata содержит стандартизированные метаданные об аниме от любого провайдера
type AnimeMetadata struct {
	Provider    string // Идентификатор провайдера ("shikimori", "anilist", "anidb")
	ID          int    // ID на платформе провайдера (например, Shikimori ID)
	TitleRu     string // Русское название (для фильма — полное название фильма)
	TitleRomaji string // Официальное название на Ромадзи
	TitleEn     string // Английское название
	ShowTitleRu string // Корневое русское название франшизы/сериала
	Season      int    // Вычисленный номер сезона
	IsMovie     bool   // Полнометражный фильм
	IsSpecial   bool   // Спешл / OVA (Season 00)
}

// Provider определяет интерфейс для любого поставщика аниме-метаданных
type Provider interface {
	ID() string
	Name() string
	Description() string
	Search(query string, proxyURL string) (*AnimeMetadata, error)
}

var (
	providersMutex sync.RWMutex
	providers      = make(map[string]Provider)
)

// Register регистрирует провайдер метаданных
func Register(p Provider) {
	providersMutex.Lock()
	defer providersMutex.Unlock()
	providers[strings.ToLower(p.ID())] = p
}

// Get возвращает провайдер по его идентификатору
func Get(id string) Provider {
	providersMutex.RLock()
	defer providersMutex.RUnlock()
	return providers[strings.ToLower(id)]
}

// All возвращает список всех зарегистрированных провайдеров
func All() []Provider {
	providersMutex.RLock()
	defer providersMutex.RUnlock()
	list := make([]Provider, 0, len(providers))
	for _, p := range providers {
		list = append(list, p)
	}
	return list
}

var stopWords = map[string]bool{
	"no": true, "to": true, "in": true, "of": true, "on": true, "at": true, "by": true, "an": true, "a": true, "is": true, "it": true, "as": true, "or": true, "the": true, "and": true,
	"и": true, "в": true, "на": true, "с": true, "по": true, "о": true, "к": true, "из": true, "за": true, "от": true, "до": true, "не": true,
}

func isStopWord(w string) bool {
	return stopWords[strings.ToLower(w)] || len(w) < 2
}

// SetCacheDir устанавливает рабочую директорию для файлов кэша всех провайдеров
func SetCacheDir(dir string) {
	SetShikimoriCacheDir(dir)
	SetAniListCacheDir(dir)
	SetAniDBCacheDir(dir)
}

// ClearAllCaches очищает кэш всех провайдеров (в памяти и на диске)
func ClearAllCaches() {
	ClearShikimoriCache()
	ClearAniListCache()
	ClearAniDBCache()
}

// ProviderCacheStats содержит статистику по количеству закэшированных тайтлов
type ProviderCacheStats struct {
	ShikimoriCount int `json:"shikimori_count"`
	AniListCount   int `json:"anilist_count"`
	AniDBCount     int `json:"anidb_count"`
	TotalCount     int `json:"total_count"`
}

// GetCacheStats возвращает количество записей в кэше каждого провайдера
func GetCacheStats() ProviderCacheStats {
	shiki := GetShikimoriCacheCount()
	anilist := GetAniListCacheCount()
	anidb := GetAniDBCacheCount()
	return ProviderCacheStats{
		ShikimoriCount: shiki,
		AniListCount:   anilist,
		AniDBCount:     anidb,
		TotalCount:     shiki + anilist + anidb,
	}
}
