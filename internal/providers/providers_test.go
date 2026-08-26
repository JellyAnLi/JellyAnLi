package providers

import (
	"testing"
)

func TestProvidersRegistry(t *testing.T) {
	shikimori := Get("shikimori")
	if shikimori == nil {
		t.Fatalf("expected shikimori provider to be registered")
	}
	if shikimori.ID() != "shikimori" {
		t.Errorf("expected ID 'shikimori', got '%s'", shikimori.ID())
	}

	anilist := Get("anilist")
	if anilist == nil {
		t.Fatalf("expected anilist provider to be registered")
	}
	if anilist.ID() != "anilist" {
		t.Errorf("expected ID 'anilist', got '%s'", anilist.ID())
	}

	anidb := Get("anidb")
	if anidb == nil {
		t.Fatalf("expected anidb provider to be registered")
	}
	if anidb.ID() != "anidb" {
		t.Errorf("expected ID 'anidb', got '%s'", anidb.ID())
	}

	all := All()
	if len(all) < 3 {
		t.Errorf("expected at least 3 providers, got %d", len(all))
	}
}

func TestAniListProviderCached(t *testing.T) {
	cache := loadAniListCache()
	aniListCacheLock.Lock()
	cache.Entries["Test Show Season 2"] = CachedAniListInfo{
		TitleRomaji: "Test Show 2nd Season",
		TitleEn:     "Test Show Season 2",
		Season:      2,
		IsMovie:     false,
		IsSpecial:   false,
	}
	aniListCacheLock.Unlock()

	prov := Get("anilist")
	meta, err := prov.Search("Test Show Season 2", "")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if meta == nil {
		t.Fatalf("expected metadata result")
	}
	if meta.TitleRomaji != "Test Show 2nd Season" {
		t.Errorf("expected TitleRomaji 'Test Show 2nd Season', got '%s'", meta.TitleRomaji)
	}
	if meta.Season != 2 {
		t.Errorf("expected Season 2, got %d", meta.Season)
	}
}

func TestAniDBProviderCached(t *testing.T) {
	cache := loadAniDBCache()
	aniDBCacheLock.Lock()
	cache.Entries["Sousou no Frieren"] = CachedAniDBInfo{
		TitleRomaji: "Sousou no Frieren",
		TitleEn:     "Frieren: Beyond Journey's End",
		TitleRu:     "Провожающая в последний путь Фрирен",
		Season:      1,
		IsMovie:     false,
		IsSpecial:   false,
	}
	aniDBCacheLock.Unlock()

	prov := Get("anidb")
	meta, err := prov.Search("Sousou no Frieren", "")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if meta == nil {
		t.Fatalf("expected metadata result")
	}
	if meta.TitleRu != "Провожающая в последний путь Фрирен" {
		t.Errorf("expected Russian title, got '%s'", meta.TitleRu)
	}
}

func TestHTTPClient(t *testing.T) {
	c1 := GetHTTPClient("")
	if c1 == nil {
		t.Fatalf("expected valid client for empty proxy")
	}

	c2 := GetHTTPClient("socks5h://127.0.0.1:1080")
	if c2 == nil || c2.Transport == nil {
		t.Fatalf("expected configured transport for socks5h")
	}

	c3 := GetHTTPClient("http://127.0.0.1:8080")
	if c3 == nil || c3.Transport == nil {
		t.Fatalf("expected configured transport for http proxy")
	}
}
