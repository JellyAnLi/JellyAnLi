package config

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// ProxyRouting определяет настройки прокси для каждого провайдера отдельно
type ProxyRouting struct {
	URL       string `json:"url"`
	Shikimori bool   `json:"shikimori"`
	AniList   bool   `json:"anilist"`
	AniDB     bool   `json:"anidb"`
}

func (pr *ProxyRouting) IsEnabled(providerID string) bool {
	if pr == nil || pr.URL == "" {
		return false
	}
	switch strings.ToLower(providerID) {
	case "shikimori":
		return pr.Shikimori
	case "anilist":
		return pr.AniList
	case "anidb":
		return pr.AniDB
	default:
		return false
	}
}

func (pr *ProxyRouting) GetProxyFor(providerID string) string {
	if pr != nil && pr.IsEnabled(providerID) {
		return pr.URL
	}
	return ""
}

// Config представляет структуру конфигурационного файла config.json
type Config struct {
	TorrentDirs         []string          `json:"torrent_dirs"`
	LibraryDir          string            `json:"library_dir"`
	SyncIntervalMinutes int               `json:"sync_interval_minutes"`
	LanguageMapping     map[string]string `json:"language_mapping"`
	MetadataProviders   []string          `json:"metadata_providers"` // ["shikimori", "anilist", "anidb"]
	FolderNamingMode    string            `json:"folder_naming_mode"` // "russian", "romaji", "original"
	ProxyRouting        *ProxyRouting     `json:"proxy_routing,omitempty"`
	ProxyURL            string            `json:"proxy_url,omitempty"` // legacy
	UseShikimori        bool              `json:"use_shikimori"`       // legacy
	UseRelativeSymlinks bool              `json:"use_relative_symlinks"`

	// Для обратной совместимости при загрузке старых конфигов
	TorrentDir        string `json:"torrent_dir,omitempty"`
	ShowsDir          string `json:"shows_dir,omitempty"`
	MoviesDir         string `json:"movies_dir,omitempty"`
	JellyfinShowsDir  string `json:"jellyfin_shows_dir,omitempty"`
	JellyfinMoviesDir string `json:"jellyfin_movies_dir,omitempty"`
}

func (c *Config) GetLibraryDir() string {
	if c == nil {
		return ""
	}
	if c.LibraryDir != "" {
		return c.LibraryDir
	}
	if c.ShowsDir != "" {
		return c.ShowsDir
	}
	return ""
}

// NewDefaultConfig создает конфигурацию по умолчанию
func NewDefaultConfig() *Config {
	return &Config{
		TorrentDirs:         []string{},
		SyncIntervalMinutes: 5,
		MetadataProviders:   []string{"shikimori", "anilist", "anidb"},
		FolderNamingMode:    "russian",
		ProxyRouting: &ProxyRouting{
			URL:       "",
			Shikimori: false,
			AniList:   false,
			AniDB:     false,
		},
		UseShikimori:        true,
		UseRelativeSymlinks: true,
		LanguageMapping: map[string]string{
			"RUS Sound": "ru",
			"RUS Subs":  "ru",
			"ENG Sound": "en",
			"ENG Subs":  "en",
			"Rus Dub":   "ru",
			"Rus Sub":   "ru",
			"Eng Dub":   "en",
			"Eng Sub":   "en",
		},
	}
}

// Load считывает конфигурацию из указанного файла
func Load(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Если файл не существует, возвращаем дефолтный конфиг
			return NewDefaultConfig(), nil
		}
		return nil, err
	}

	cfg := NewDefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	if cfg.FolderNamingMode == "" {
		cfg.FolderNamingMode = "russian"
	}

	// Миграция для провайдеров метаданных
	if len(cfg.MetadataProviders) == 0 {
		if bytes.Contains(data, []byte("\"use_shikimori\": false")) {
			cfg.MetadataProviders = []string{}
		} else {
			cfg.MetadataProviders = []string{"shikimori", "anilist", "anidb"}
		}
	}

	// Миграция для прокси
	if cfg.ProxyRouting == nil {
		cfg.ProxyRouting = &ProxyRouting{
			URL:       cfg.ProxyURL,
			Shikimori: cfg.ProxyURL != "",
			AniList:   cfg.ProxyURL != "",
			AniDB:     cfg.ProxyURL != "",
		}
	} else if cfg.ProxyRouting.URL == "" && cfg.ProxyURL != "" {
		cfg.ProxyRouting.URL = cfg.ProxyURL
	}

	if !bytes.Contains(data, []byte("use_relative_symlinks")) {
		cfg.UseRelativeSymlinks = true
	}

	// Дополняем недостающие ключи в LanguageMapping дефолтными значениями
	defaultCfg := NewDefaultConfig()
	if cfg.LanguageMapping == nil {
		cfg.LanguageMapping = defaultCfg.LanguageMapping
	} else {
		for k, v := range defaultCfg.LanguageMapping {
			if _, exists := cfg.LanguageMapping[k]; !exists {
				cfg.LanguageMapping[k] = v
			}
		}
	}

	// Миграция для обратной совместимости
	if len(cfg.TorrentDirs) == 0 && cfg.TorrentDir != "" {
		cfg.TorrentDirs = []string{cfg.TorrentDir}
	}
	if cfg.LibraryDir == "" {
		if cfg.ShowsDir != "" {
			cfg.LibraryDir = cfg.ShowsDir
		} else if cfg.JellyfinShowsDir != "" {
			cfg.LibraryDir = cfg.JellyfinShowsDir
		} else if cfg.JellyfinMoviesDir != "" {
			cfg.LibraryDir = cfg.JellyfinMoviesDir
		}
	}

	// Сбрасываем старые поля
	cfg.TorrentDir = ""
	cfg.ShowsDir = ""
	cfg.MoviesDir = ""
	cfg.JellyfinShowsDir = ""
	cfg.JellyfinMoviesDir = ""

	return cfg, nil
}

// Save сохраняет конфигурацию в указанный файл
func (c *Config) Save(configPath string) error {
	// Сбрасываем старые поля перед сохранением, чтобы они не попали в файл
	c.TorrentDir = ""
	c.ShowsDir = ""
	c.MoviesDir = ""
	c.JellyfinShowsDir = ""
	c.JellyfinMoviesDir = ""

	// Создаем родительские директории, если их нет
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}
