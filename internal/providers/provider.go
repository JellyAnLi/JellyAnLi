package providers

import (
	"strings"
	"sync"
)

// AnimeMetadata содержит стандартизированные метаданные об аниме от любого провайдера
type AnimeMetadata struct {
	Provider    string // Идентификатор провайдера ("shikimori", "anilist", "anidb")
	TitleRu     string // Русское название
	TitleRomaji string // Официальное название на Ромадзи
	TitleEn     string // Английское название
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
