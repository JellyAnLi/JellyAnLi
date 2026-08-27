package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigLoadSave(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "jelly-an-li-config-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")

	// Тест 1: Загрузка несуществующего файла должна вернуть дефолтный конфиг
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("expected no error on missing file, got: %v", err)
	}
	if cfg.SyncIntervalMinutes != 5 {
		t.Errorf("expected default sync interval 5, got %d", cfg.SyncIntervalMinutes)
	}
	if !cfg.UseShikimori {
		t.Error("expected default use shikimori to be true")
	}
	if cfg.LanguageMapping["RUS Sound"] != "ru" {
		t.Errorf("expected default language mapping for RUS Sound to be ru, got %s", cfg.LanguageMapping["RUS Sound"])
	}

	// Тест 2: Сохранение измененной конфигурации
	cfg.TorrentDirs = []string{"/path/to/torrent"}
	cfg.LibraryDir = "/path/to/library"
	cfg.SyncIntervalMinutes = 10
	err = cfg.Save(configPath)
	if err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// Тест 3: Загрузка сохраненного файла
	cfg2, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load saved config: %v", err)
	}
	if len(cfg2.TorrentDirs) != 1 || cfg2.TorrentDirs[0] != "/path/to/torrent" {
		t.Errorf("expected TorrentDirs '/path/to/torrent', got '%v'", cfg2.TorrentDirs)
	}
	if cfg2.LibraryDir != "/path/to/library" || cfg2.GetLibraryDir() != "/path/to/library" {
		t.Errorf("expected LibraryDir '/path/to/library', got '%s'", cfg2.LibraryDir)
	}
	if cfg2.SyncIntervalMinutes != 10 {
		t.Errorf("expected SyncIntervalMinutes 10, got %d", cfg2.SyncIntervalMinutes)
	}

	// Тест 4: ProxyRouting и MetadataProviders
	cfg2.MetadataProviders = []string{"anilist", "shikimori"}
	cfg2.ProxyRouting = &ProxyRouting{
		URL:       "socks5h://127.0.0.1:1080",
		Shikimori: false,
		AniList:   true,
		AniDB:     true,
	}
	_ = cfg2.Save(configPath)

	cfg3, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load saved config: %v", err)
	}
	if len(cfg3.MetadataProviders) != 2 || cfg3.MetadataProviders[0] != "anilist" {
		t.Errorf("expected MetadataProviders ['anilist', 'shikimori'], got %v", cfg3.MetadataProviders)
	}
	if cfg3.ProxyRouting == nil || cfg3.ProxyRouting.URL != "socks5h://127.0.0.1:1080" {
		t.Errorf("expected ProxyRouting URL 'socks5h://127.0.0.1:1080', got %v", cfg3.ProxyRouting)
	}
	if !cfg3.ProxyRouting.IsEnabled("anilist") {
		t.Errorf("expected anilist proxy to be enabled")
	}
	if cfg3.ProxyRouting.IsEnabled("shikimori") {
		t.Errorf("expected shikimori proxy to be disabled")
	}
	if cfg3.ProxyRouting.GetProxyFor("anilist") != "socks5h://127.0.0.1:1080" {
		t.Errorf("expected proxy URL for anilist")
	}
	if cfg3.ProxyRouting.GetProxyFor("shikimori") != "" {
		t.Errorf("expected empty proxy URL for shikimori")
	}
}

func TestLegacyConfigMigration(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "jelly-an-li-legacy-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	legacyJSON := `{
		"torrent_dirs": ["/torrents"],
		"shows_dir": "/legacy_shows",
		"sync_interval_minutes": 15
	}`
	configPath := filepath.Join(tmpDir, "config.json")
	if err := os.WriteFile(configPath, []byte(legacyJSON), 0644); err != nil {
		t.Fatalf("failed to write legacy config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load legacy config: %v", err)
	}
	if cfg.LibraryDir != "/legacy_shows" {
		t.Errorf("expected LibraryDir migrated from shows_dir, got '%s'", cfg.LibraryDir)
	}
	if cfg.GetLibraryDir() != "/legacy_shows" {
		t.Errorf("expected GetLibraryDir to return '/legacy_shows', got '%s'", cfg.GetLibraryDir())
	}
}

