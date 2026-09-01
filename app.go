package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"jelly-an-li/internal/config"
	"jelly-an-li/internal/linker"
	"jelly-an-li/internal/providers"
)

// ClientSubscriber представляет подписчика на события с приоритетным каналом статуса
type ClientSubscriber struct {
	StatusCh chan string
	LogCh    chan string
	ResetCh  chan struct{}
}

// App struct — основная структура приложения
type App struct {
	config             *config.Config
	configPath         string
	statePath          string
	configDir          string
	syncMutex          sync.Mutex
	ticker             *time.Ticker
	tickerStop         chan struct{}
	tickerLock         sync.Mutex
	logs               []string
	logsMutex          sync.Mutex
	isSyncing          bool
	statusMutex        sync.RWMutex
	eventSubscribers   map[*ClientSubscriber]struct{}
	eventListenersLock sync.Mutex
	dryRunTimer        *time.Timer
	dryRunTimerLock    sync.Mutex
}

// NewApp создает новый экземпляр приложения
func NewApp() *App {
	return &App{}
}

// startup вызывается при старте приложения
func (a *App) startup() {
	var configDir string
	if envDir := os.Getenv("CONFIG_DIR"); envDir != "" {
		configDir = envDir
	} else {
		execPath, err := os.Executable()
		if err != nil {
			configDir = "."
		} else {
			configDir = filepath.Dir(execPath)
		}
	}
	a.configDir = configDir
	a.configPath = filepath.Join(configDir, "config.json")
	a.statePath = filepath.Join(configDir, "state.json")

	providers.SetCacheDir(configDir)

	fmt.Printf("Starting Jellyfin Anime Linker. Config path: %s\n", a.configPath)

	// Загружаем конфигурацию
	cfg, err := config.Load(a.configPath)
	if err != nil {
		fmt.Printf("Error loading config: %v. Using default.\n", err)
		cfg = config.NewDefaultConfig()
	}
	a.config = cfg
	_ = a.config.Save(a.configPath)
}

// StartBackgroundSync запускает фоновый тикер периодической проверки
func (a *App) StartBackgroundSync() {
	a.startTicker()

	// Запускаем стартовую синхронизацию в фоне, только если настроены папки торрентов
	go func() {
		time.Sleep(1 * time.Second)
		if a.config != nil && len(a.config.TorrentDirs) > 0 && a.config.TorrentDirs[0] != "" {
			a.log("Выполнение стартовой синхронизации при запуске...")
			a.RunSync(false)
		} else {
			a.log("Стартовая синхронизация пропущена: не настроена папка торрентов.")
		}
	}()
}

// shutdown останавливает фоновые таймеры
func (a *App) shutdown() {
	a.stopTicker()
	a.cancelDryRunTimer()
}

// stopTicker останавливает фоновый таймер
func (a *App) stopTicker() {
	a.tickerLock.Lock()
	defer a.tickerLock.Unlock()
	if a.ticker != nil {
		a.ticker.Stop()
		a.ticker = nil
		a.log("Фоновый таймер остановлен.")
	}
	if a.tickerStop != nil {
		close(a.tickerStop)
		a.tickerStop = nil
	}
}

// startTicker запускает или перезапускает фоновый таймер синхронизации
func (a *App) startTicker() {
	a.stopTicker()

	a.tickerLock.Lock()
	defer a.tickerLock.Unlock()

	a.tickerStop = make(chan struct{})
	interval := time.Duration(a.config.SyncIntervalMinutes) * time.Minute
	a.ticker = time.NewTicker(interval)

	a.log("Запущен фоновый таймер проверки. Интервал: %d мин.", a.config.SyncIntervalMinutes)

	go func(stop chan struct{}) {
		for {
			select {
			case <-a.ticker.C:
				a.RunSync(false, true)
			case <-stop:
				return
			}
		}
	}(a.tickerStop)
}

// log добавляет сообщение в журнал и отправляет событие клиентам
func (a *App) log(format string, args ...interface{}) {
	msg := fmt.Sprintf("[%s] ", time.Now().Format("15:04:05")) + fmt.Sprintf(format, args...)
	fmt.Println(msg)

	a.logsMutex.Lock()
	a.logs = append(a.logs, msg)
	if len(a.logs) > 50000 {
		a.logs = a.logs[len(a.logs)-50000:]
	}
	a.logsMutex.Unlock()

	a.eventListenersLock.Lock()
	defer a.eventListenersLock.Unlock()
	for sub := range a.eventSubscribers {
		select {
		case sub.LogCh <- msg:
		default:
		}
	}
}

// SubscribeEvents подписывает клиента на события и возвращает каналы и функцию отписки
func (a *App) SubscribeEvents() (*ClientSubscriber, func()) {
	sub := &ClientSubscriber{
		StatusCh: make(chan string, 20),
		LogCh:    make(chan string, 50000), // огромный буфер на 50 000 сообщений
		ResetCh:  make(chan struct{}, 10),
	}
	a.eventListenersLock.Lock()
	if a.eventSubscribers == nil {
		a.eventSubscribers = make(map[*ClientSubscriber]struct{})
	}
	a.eventSubscribers[sub] = struct{}{}
	a.eventListenersLock.Unlock()

	unsubscribe := func() {
		a.eventListenersLock.Lock()
		delete(a.eventSubscribers, sub)
		close(sub.StatusCh)
		close(sub.LogCh)
		close(sub.ResetCh)
		a.eventListenersLock.Unlock()
	}
	return sub, unsubscribe
}

// setSyncing меняет статус синхронизации и мгновенно оповещает SSE-клиентов по приоритетному каналу
func (a *App) setSyncing(syncing bool) {
	a.statusMutex.Lock()
	a.isSyncing = syncing
	a.statusMutex.Unlock()

	statusJSON := fmt.Sprintf(`{"syncing":%t}`, syncing)

	a.eventListenersLock.Lock()
	defer a.eventListenersLock.Unlock()
	for sub := range a.eventSubscribers {
		select {
		case sub.StatusCh <- statusJSON:
		default:
			// Если канал статуса заполнен, освобождаем старый статус и пишем актуальный
			select {
			case <-sub.StatusCh:
			default:
			}
			sub.StatusCh <- statusJSON
		}
	}
}

// GetConfig возвращает текущую конфигурацию
func (a *App) GetConfig() *config.Config {
	return a.config
}

// SaveConfig сохраняет новую конфигурацию
func (a *App) SaveConfig(newCfg *config.Config) error {
	if newCfg.SyncIntervalMinutes <= 0 {
		return fmt.Errorf("некорректный интервал синхронизации")
	}

	a.config.TorrentDirs = newCfg.TorrentDirs
	a.config.LibraryDir = newCfg.GetLibraryDir()
	a.config.SyncIntervalMinutes = newCfg.SyncIntervalMinutes
	a.config.MetadataProviders = newCfg.MetadataProviders
	a.config.FolderNamingMode = newCfg.FolderNamingMode
	a.config.ProxyRouting = newCfg.ProxyRouting
	a.config.ProxyURL = newCfg.ProxyURL
	a.config.UseShikimori = newCfg.UseShikimori
	a.config.UseRelativeSymlinks = newCfg.UseRelativeSymlinks
	if newCfg.LanguageMapping != nil {
		a.config.LanguageMapping = newCfg.LanguageMapping
	}

	if err := a.config.Save(a.configPath); err != nil {
		return fmt.Errorf("ошибка сохранения конфигурации: %v", err)
	}

	a.log("Настройки успешно сохранены.")

	// Перезапускаем тикер с новым интервалом
	a.startTicker()

	return nil
}

// GetLogs возвращает все накопленные логи
func (a *App) GetLogs() []string {
	a.logsMutex.Lock()
	defer a.logsMutex.Unlock()
	result := make([]string, len(a.logs))
	copy(result, a.logs)
	return result
}

// isDryRunLog проверяет, относится ли строка лога к выводу предпросмотра (дереву файлов)
func isDryRunLog(msg string) bool {
	content := msg
	if idx := strings.Index(msg, "] "); idx != -1 {
		content = msg[idx+2:]
	}
	trimmed := strings.TrimSpace(content)
	if strings.Contains(trimmed, "Предпросмотр:") || strings.Contains(trimmed, "Предпросмотр завершен:") {
		return true
	}
	if strings.HasPrefix(trimmed, "📁") {
		return true
	}
	if strings.Contains(trimmed, "📂") {
		return true
	}
	if strings.Contains(trimmed, "➔ (из ") || strings.Contains(trimmed, "➔ (из") {
		return true
	}
	return false
}

// removeDryRunLogsLocked удаляет строки предпросмотра из журнала (вызывать под logsMutex)
func (a *App) removeDryRunLogsLocked() bool {
	originalLen := len(a.logs)
	filtered := a.logs[:0]
	for _, l := range a.logs {
		if !isDryRunLog(l) {
			filtered = append(filtered, l)
		}
	}
	a.logs = filtered
	return len(a.logs) != originalLen
}

// ClearDryRunLogs удаляет старые логи предпросмотра и оповещает клиентов
func (a *App) ClearDryRunLogs() {
	a.logsMutex.Lock()
	changed := a.removeDryRunLogsLocked()
	a.logsMutex.Unlock()

	if changed {
		a.notifyLogsReset()
	}
}

// notifyLogsReset оповещает всех подписчиков о необходимости сброса/перезагрузки логов
func (a *App) notifyLogsReset() {
	a.eventListenersLock.Lock()
	defer a.eventListenersLock.Unlock()
	for sub := range a.eventSubscribers {
		select {
		case sub.ResetCh <- struct{}{}:
		default:
		}
	}
}

// cancelDryRunTimer останавливает таймер автоочистки предпросмотра
func (a *App) cancelDryRunTimer() {
	a.dryRunTimerLock.Lock()
	defer a.dryRunTimerLock.Unlock()
	if a.dryRunTimer != nil {
		a.dryRunTimer.Stop()
		a.dryRunTimer = nil
	}
}

// scheduleDryRunExpiry планирует автоочистку предпросмотра через указанный интервал времени
func (a *App) scheduleDryRunExpiry(d time.Duration) {
	a.cancelDryRunTimer()

	a.dryRunTimerLock.Lock()
	defer a.dryRunTimerLock.Unlock()

	a.dryRunTimer = time.AfterFunc(d, func() {
		a.logsMutex.Lock()
		changed := a.removeDryRunLogsLocked()
		a.logsMutex.Unlock()

		if changed {
			a.notifyLogsReset()
			a.log("ℹ️ Дерево предпросмотра автоматически очищено по таймауту (15 мин).")
		}
	})
}

// ClearLogs очищает буфер логов
func (a *App) ClearLogs() {
	a.cancelDryRunTimer()
	a.logsMutex.Lock()
	a.logs = a.logs[:0]
	a.logsMutex.Unlock()
	a.notifyLogsReset()
}

// CacheInfo содержит сводные данные о кэшах приложения
type CacheInfo struct {
	ShikimoriCount  int `json:"shikimori_count"`
	AniListCount    int `json:"anilist_count"`
	AniDBCount      int `json:"anidb_count"`
	TotalMetaCount  int `json:"total_meta_count"`
	StateFilesCount int `json:"state_files_count"`
}

// GetCacheInfo возвращает актуальную статистику по кэшу метаданных и состоянию
func (a *App) GetCacheInfo() CacheInfo {
	stats := providers.GetCacheStats()
	stateCount := linker.GetStateFilesCount(a.statePath)
	return CacheInfo{
		ShikimoriCount:  stats.ShikimoriCount,
		AniListCount:    stats.AniListCount,
		AniDBCount:      stats.AniDBCount,
		TotalMetaCount:  stats.TotalCount,
		StateFilesCount: stateCount,
	}
}

// ClearCache очищает кэш метаданных провайдеров и/или файл состояния связей
func (a *App) ClearCache(clearMetadata bool, clearState bool) {
	if clearMetadata {
		providers.ClearAllCaches()
		a.log("🧹 Кэш метаданных провайдеров (Shikimori, AniList, AniDB) очищен.")
	}
	if clearState {
		_ = os.Remove(a.statePath)
		a.log("🧹 Кэш состояния связей (state.json) удален.")
	}
}

// IsSyncing возвращает текущий статус синхронизации
func (a *App) IsSyncing() bool {
	a.statusMutex.RLock()
	defer a.statusMutex.RUnlock()
	return a.isSyncing
}

// naturalLess реализует человеческую (естественную) сортировку чисел в строках
// (например, "E09" < "E10" < "E11" < "E99" < "E100" < "E101")
func naturalLess(s1, s2 string) bool {
	i, j := 0, 0
	for i < len(s1) && j < len(s2) {
		r1, size1 := utf8.DecodeRuneInString(s1[i:])
		r2, size2 := utf8.DecodeRuneInString(s2[j:])

		if unicode.IsDigit(r1) && unicode.IsDigit(r2) {
			end1 := i
			for end1 < len(s1) && s1[end1] >= '0' && s1[end1] <= '9' {
				end1++
			}
			end2 := j
			for end2 < len(s2) && s2[end2] >= '0' && s2[end2] <= '9' {
				end2++
			}

			numStr1 := s1[i:end1]
			numStr2 := s2[j:end2]

			trim1 := strings.TrimLeft(numStr1, "0")
			trim2 := strings.TrimLeft(numStr2, "0")

			if len(trim1) != len(trim2) {
				return len(trim1) < len(trim2)
			}
			if trim1 != trim2 {
				return trim1 < trim2
			}
			if len(numStr1) != len(numStr2) {
				return len(numStr1) > len(numStr2)
			}

			i = end1
			j = end2
			continue
		}

		lower1 := unicode.ToLower(r1)
		lower2 := unicode.ToLower(r2)
		if lower1 != lower2 {
			return lower1 < lower2
		}

		i += size1
		j += size2
	}

	return len(s1) < len(s2)
}

func (a *App) relTarget(targetPath string) string {
	if a.config != nil {
		lib := a.config.GetLibraryDir()
		if lib != "" {
			if rel, err := filepath.Rel(lib, targetPath); err == nil && !strings.HasPrefix(rel, "..") {
				return rel
			}
		}
	}
	return filepath.Base(targetPath)
}

func (a *App) relSource(sourcePath string) string {
	if a.config != nil {
		for _, td := range a.config.TorrentDirs {
			if td != "" {
				if rel, err := filepath.Rel(td, sourcePath); err == nil && !strings.HasPrefix(rel, "..") {
					return rel
				}
			}
		}
	}
	parent := filepath.Base(filepath.Dir(sourcePath))
	if parent != "" && parent != "." && parent != "/" {
		return filepath.Join(parent, filepath.Base(sourcePath))
	}
	return filepath.Base(sourcePath)
}

// RunSync запускает процесс сканирования и линкования
func (a *App) RunSync(dryRun bool, isBackground ...bool) {
	bg := len(isBackground) > 0 && isBackground[0]

	if !a.syncMutex.TryLock() {
		if !bg {
			a.log("Предупреждение: Синхронизация уже выполняется.")
		}
		return
	}
	a.setSyncing(true)
	defer func() {
		a.setSyncing(false)
		a.syncMutex.Unlock()
	}()

	// Сбрасываем старый таймер и очищаем предыдущий предпросмотр
	a.cancelDryRunTimer()
	a.ClearDryRunLogs()

	if len(a.config.TorrentDirs) == 0 || a.config.TorrentDirs[0] == "" {
		a.log("Ошибка: Не указаны папки с торрентами.")
		return
	}

	if a.config.GetLibraryDir() == "" {
		a.log("Ошибка: Не указана библиотека Jellyfin.")
		return
	}

	// 1. Сканирование
	shows, err := linker.Scan(a.config)
	if err != nil {
		a.log("Ошибка сканирования: %v", err)
		return
	}

	// 2. Построение плана
	plan := linker.GeneratePlan(shows, a.config)

	if dryRun {
		if len(plan) == 0 {
			a.log("✨ Предпросмотр: библиотека актуальна, новых файлов нет (раздач: %d).", len(shows))
			return
		}

		a.log("🔍 Предпросмотр: %d новых файлов в плане (раздач: %d)", len(plan), len(shows))

		type fileEntry struct {
			targetFile string
			sourceRel  string
		}
		type seasonGroup struct {
			seasonName string
			files      []fileEntry
		}
		type showGroup struct {
			showName string
			seasons  []*seasonGroup
		}

		var showList []*showGroup
		showMap := make(map[string]*showGroup)
		seasonMap := make(map[string]map[string]*seasonGroup)

		for _, op := range plan {
			rel := a.relTarget(op.TargetPath)
			parts := strings.Split(filepath.ToSlash(rel), "/")

			showName := "Библиотека"
			seasonName := "Корневые файлы"
			fileName := rel

			if len(parts) >= 3 {
				showName = parts[0]
				seasonName = parts[1]
				fileName = strings.Join(parts[2:], "/")
			} else if len(parts) == 2 {
				showName = parts[0]
				seasonName = "Фильм"
				fileName = parts[1]
			}

			sg, exists := showMap[showName]
			if !exists {
				sg = &showGroup{showName: showName}
				showMap[showName] = sg
				seasonMap[showName] = make(map[string]*seasonGroup)
				showList = append(showList, sg)
			}

			sng, sExists := seasonMap[showName][seasonName]
			if !sExists {
				sng = &seasonGroup{seasonName: seasonName}
				seasonMap[showName][seasonName] = sng
				sg.seasons = append(sg.seasons, sng)
			}

			sng.files = append(sng.files, fileEntry{
				targetFile: fileName,
				sourceRel:  a.relSource(op.SourcePath),
			})
		}

		// Сортировка тайтлов по названию (естественный порядок)
		sort.Slice(showList, func(i, j int) bool {
			return naturalLess(showList[i].showName, showList[j].showName)
		})

		// Сортировка сезонов и файлов (человеческий порядок чисел: E09 < E10 < E100)
		for _, sg := range showList {
			sort.Slice(sg.seasons, func(i, j int) bool {
				return naturalLess(sg.seasons[i].seasonName, sg.seasons[j].seasonName)
			})
			for _, sng := range sg.seasons {
				sort.Slice(sng.files, func(i, j int) bool {
					return naturalLess(sng.files[i].targetFile, sng.files[j].targetFile)
				})
			}
		}

		for _, sg := range showList {
			a.log("📁 %s", sg.showName)
			for sIdx, sng := range sg.seasons {
				isLastSeason := sIdx == len(sg.seasons)-1
				seasonBranch := "├──"
				if isLastSeason {
					seasonBranch = "└──"
				}

				a.log("  %s 📂 %s", seasonBranch, sng.seasonName)
				for fIdx, f := range sng.files {
					isLastFile := fIdx == len(sng.files)-1
					fileBranch := "├──"
					if isLastFile {
						fileBranch = "└──"
					}

					icon := "🎬"
					ext := strings.ToLower(filepath.Ext(f.targetFile))
					if ext == ".mka" || ext == ".ac3" || ext == ".flac" || ext == ".aac" {
						icon = "🎵"
					} else if ext == ".ass" || ext == ".srt" || ext == ".vtt" {
						icon = "💬"
					}

					a.log("    %s %s %s ➔ (из %s)", fileBranch, icon, f.targetFile, f.sourceRel)
				}
			}
		}

		a.log("✓ Предпросмотр завершен: %d симлинков готово к созданию", len(plan))
		a.scheduleDryRunExpiry(15 * time.Minute)
		return
	}

	// 3. Применение плана
	err = linker.ApplyPlan(plan, a.statePath, a.config)
	if err != nil {
		a.log("Ошибка в процессе линкования: %v", err)
	}

	// Сбор статистики
	successCount := 0
	skippedCount := 0
	errorCount := 0
	var successLogs []string
	var errorLogs []string

	for _, op := range plan {
		switch op.Status {
		case linker.StatusSuccess:
			successCount++
			if successCount <= 30 {
				successLogs = append(successLogs, fmt.Sprintf("➔ Создана ссылка: %s", a.relTarget(op.TargetPath)))
			}
		case linker.StatusSkipped:
			skippedCount++
		case linker.StatusError:
			errorCount++
			errorLogs = append(errorLogs, fmt.Sprintf("Ошибка создания ссылки для %s: %s", a.relTarget(op.TargetPath), op.Message))
		}
	}

	// 4. Очистка нерабочих и устаревших симлинков (не входящих в актуальный план)
	cleaned, err := linker.CleanBrokenLinks(a.config, a.statePath, plan)
	if err != nil {
		a.log("Ошибка очистки сломанных ссылок: %v", err)
	}

	// 5. Итоговый аккуратный вывод
	if successCount == 0 && errorCount == 0 && len(cleaned) == 0 {
		// В фоновом режиме по расписанию не засоряем логи, если изменений нет
		if !bg {
			a.log("✨ Библиотека актуальна: без изменений (раздач: %d, проверено: %d)", len(shows), skippedCount)
		}
		return
	}

	// Если ЕСТЬ изменения или ошибки — выводим красивый информативный блок
	a.log("🚀 Синхронизация: создано ссылок: %d, удалено: %d, ошибок: %d", successCount, len(cleaned), errorCount)
	for _, l := range successLogs {
		a.log("%s", l)
	}
	if successCount > 30 {
		a.log("...и еще %d успешно созданных ссылок скрыто для экономии места.", successCount-30)
	}
	for _, l := range errorLogs {
		a.log("%s", l)
	}
	if len(cleaned) > 0 {
		a.log("Удалено недействительных ссылок/папок: %d", len(cleaned))
		for i, p := range cleaned {
			if i >= 10 {
				a.log("  ...и еще %d удаленных путей скрыто.", len(cleaned)-10)
				break
			}
			a.log("  Удалено: %s", p)
		}
	}
	a.log("✓ Синхронизация завершена успешно")
}
