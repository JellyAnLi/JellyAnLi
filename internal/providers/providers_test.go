package providers

import (
	"net/http"
	"net/http/httptest"
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

func TestShikimoriScoringBlackCloverMovie(t *testing.T) {
	// Инициализируем изолированный кэш Shikimori
	restore := SetShikimoriCacheForTest(map[string]CachedShikimoriInfo{
		"Black Clover Mahoutei no Ken": {
			Russian:   "Чёрный клевер: Меч короля магов",
			Romaji:    "Black Clover: Mahou Tei no Ken",
			Season:    1,
			IsMovie:   true,
			IsSpecial: false,
		},
	})
	defer restore()

	prov := Get("shikimori")
	meta, err := prov.Search("Black Clover Mahoutei no Ken", "")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if meta == nil {
		t.Fatalf("expected metadata result")
	}
	if meta.TitleRu != "Чёрный клевер: Меч короля магов" {
		t.Errorf("expected TitleRu 'Чёрный клевер: Меч короля магов', got '%s'", meta.TitleRu)
	}
	if meta.TitleRomaji != "Black Clover: Mahou Tei no Ken" {
		t.Errorf("expected TitleRomaji 'Black Clover: Mahou Tei no Ken', got '%s'", meta.TitleRomaji)
	}
	if !meta.IsMovie {
		t.Errorf("expected IsMovie to be true")
	}
}

func TestShikimoriFateZeroCached(t *testing.T) {
	restore := SetShikimoriCacheForTest(map[string]CachedShikimoriInfo{
		"Fate Zero": {
			Russian:   "Судьба/Начало",
			Romaji:    "Fate/Zero",
			ShowRu:    "Судьба/Начало",
			Season:    1,
			IsMovie:   false,
			IsSpecial: false,
		},
	})
	defer restore()

	prov := Get("shikimori")
	meta, err := prov.Search("Fate Zero", "")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if meta == nil {
		t.Fatalf("expected metadata result")
	}
	if meta.ShowTitleRu != "Судьба/Начало" {
		t.Errorf("expected ShowTitleRu 'Судьба/Начало', got '%s'", meta.ShowTitleRu)
	}
	if meta.Season != 1 {
		t.Errorf("expected Season 1, got %d", meta.Season)
	}
	if meta.IsMovie || meta.IsSpecial {
		t.Errorf("expected regular TV series, got movie=%v, special=%v", meta.IsMovie, meta.IsSpecial)
	}
}

func TestClearAllCachesAndStats(t *testing.T) {
	// Добавляем тестовые данные в кэш
	cache := loadAniListCache()
	aniListCacheLock.Lock()
	cache.Entries["Test AniList Title"] = CachedAniListInfo{
		TitleRomaji: "Test Title",
		Season:      1,
	}
	aniListCacheLock.Unlock()

	dbCache := loadAniDBCache()
	aniDBCacheLock.Lock()
	dbCache.Entries["Test AniDB Title"] = CachedAniDBInfo{
		TitleRomaji: "Test Title AniDB",
		Season:      1,
	}
	aniDBCacheLock.Unlock()

	shikiCache := loadShikimoriCache()
	shikimoriCacheLock.Lock()
	shikiCache.Translations["Test Shikimori Title"] = CachedShikimoriInfo{
		Russian: "Тестовое аниме",
		Season:  1,
	}
	shikimoriCacheLock.Unlock()

	stats := GetCacheStats()
	if stats.AniListCount < 1 || stats.AniDBCount < 1 || stats.ShikimoriCount < 1 {
		t.Errorf("expected cache stats to have at least 1 in each provider, got %+v", stats)
	}

	ClearAllCaches()

	statsAfter := GetCacheStats()
	if statsAfter.AniListCount != 0 || statsAfter.AniDBCount != 0 || statsAfter.ShikimoriCount != 0 {
		t.Errorf("expected all cache counts to be 0 after ClearAllCaches, got %+v", statsAfter)
	}
}

func TestShikimoriRejectsIrrelevantResults(t *testing.T) {
	// Создаем тестовый сервер, имитирующий реальную ситуацию из output.log:
	// На запрос "MASHLE 2" API Шикимори возвращает "Diamond no Ace" (Путь аса),
	// а на запрос "MASHLE" возвращает "Mashle" (Магия и мускулы).
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		q := r.URL.Query().Get("search")
		if r.URL.Path == "/api/animes" {
			if q == "MASHLE 2" {
				// Ложный результат из API
				w.Write([]byte(`[{"id":64505,"name":"Diamond no Ace: Act II Second Season Part 2","russian":"Путь аса: Акт II 2. Часть 2","kind":"tv"}]`))
				return
			}
			if q == "MASHLE" {
				w.Write([]byte(`[{"id":52211,"name":"Mashle","russian":"Магия и мускулы","kind":"tv"}]`))
				return
			}
		}
		if r.URL.Path == "/api/animes/52211/franchise" {
			w.Write([]byte(`{"links":[],"nodes":[{"id":52211,"name":"Магия и мускулы","kind":"tv"}]}`))
			return
		}
		w.Write([]byte(`[]`))
	}))
	defer ts.Close()

	oldHost := shikimoriHost
	shikimoriHost = ts.URL
	defer func() { shikimoriHost = oldHost }()

	// Очищаем кэш перед тестом
	ClearShikimoriCache()
	defer ClearShikimoriCache()

	prov := Get("shikimori")

	// 1. Проверяем, что ложный результат (Diamond no Ace) для "MASHLE 2" отсеивается и возвращается nil!
	metaIrrelevant, err := prov.Search("MASHLE 2", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if metaIrrelevant != nil {
		t.Fatalf("expected nil for irrelevant query 'MASHLE 2', got %+v", metaIrrelevant)
	}

	// 2. Проверяем, что запрос "MASHLE" успешно находит "Магия и мускулы"
	metaMashle, err := prov.Search("MASHLE", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if metaMashle == nil {
		t.Fatalf("expected valid metadata for 'MASHLE'")
	}
	if metaMashle.ShowTitleRu != "Магия и мускулы" {
		t.Errorf("expected ShowTitleRu 'Магия и мускулы', got '%s'", metaMashle.ShowTitleRu)
	}
	if metaMashle.TitleRomaji != "Mashle" {
		t.Errorf("expected TitleRomaji 'Mashle', got '%s'", metaMashle.TitleRomaji)
	}
}



