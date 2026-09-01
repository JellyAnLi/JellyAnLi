<script setup>
import { reactive, watch, ref, onMounted, onUnmounted } from 'vue'
import FolderBrowserModal from './FolderBrowserModal.vue'
import { GetVersion, GetCacheStats, ClearCache } from '../api.js'

const props = defineProps({
  config: {
    type: Object,
    required: true
  },
  syncing: {
    type: Boolean,
    default: false
  },
  serverOnline: {
    type: Boolean,
    default: true
  }
})

const emit = defineEmits(['save', 'sync', 'toast'])

// Активная подвкладка настроек: 'storage' | 'metadata' | 'maintenance'
const activeSubTab = ref('storage')

const ALL_PROVIDERS = [
  {
    id: 'shikimori',
    name: 'Shikimori',
    badge: '🇷🇺 RU / Франшизы',
    desc: 'Русскоязычная база аниме с детальным деревом сезонов и франшиз'
  },
  {
    id: 'anilist',
    name: 'AniList',
    badge: '🌐 Global / GraphQL',
    desc: 'Открытая мировая база: официальные Romaji, English названия, форматы и сезоны'
  },
  {
    id: 'anidb',
    name: 'AniDB',
    badge: '📑 Синонимы',
    desc: 'Эталонная база альтернативных названий и синонимов'
  }
]

// Локальная копия конфигурации для редактирования
const form = reactive({
  torrent_dirs: [''],
  library_dir: '',
  sync_interval_minutes: 5,
  folder_naming_mode: 'russian',
  use_relative_symlinks: true,
  providers: [
    { id: 'shikimori', enabled: true, use_proxy: false },
    { id: 'anilist', enabled: true, use_proxy: false },
    { id: 'anidb', enabled: true, use_proxy: false }
  ],
  proxy_url: '',
  language_mapping: []
})

// Состояние модального окна выбора папок
const isModalShow = ref(false)
const currentModalField = ref(null)
const currentModalInitialPath = ref('')

// Состояние проверки обновлений
const versionData = ref(null)
const checkingUpdate = ref(false)

// Состояние кэша
const cacheStats = ref(null)
const clearingCache = ref(false)

async function loadCacheStats() {
  try {
    const stats = await GetCacheStats()
    if (stats) cacheStats.value = stats
  } catch (e) {
    /* ignore */
  }
}

async function handleClearMetadataOnly() {
  if (!props.serverOnline) {
    emit('toast', 'Сервер недоступен. Дождитесь переподключения.', 'error')
    return
  }
  clearingCache.value = true
  try {
    await ClearCache({ clear_metadata: true, clear_state: false, resync: false })
    await loadCacheStats()
    emit('toast', 'Кэш метаданных (Shikimori / AniList / AniDB) успешно очищен!', 'success')
  } catch (e) {
    emit('toast', 'Ошибка очистки кэша: ' + e, 'error')
  } finally {
    clearingCache.value = false
  }
}

async function handleResyncWithClear(dryRun = false) {
  if (!props.serverOnline) {
    emit('toast', 'Сервер недоступен. Дождитесь переподключения.', 'error')
    return
  }
  if (props.syncing) return

  const actionMsg = dryRun
    ? 'Сбросить кэш метаданных и состояния и запустить предпросмотр (Dry-Run)?'
    : 'Сбросить сохранённый кэш метаданных и состояние и выполнить полную пересинхронизацию всех раздач?'

  if (!confirm(actionMsg)) {
    return
  }

  clearingCache.value = true
  try {
    await ClearCache({ clear_metadata: true, clear_state: true, resync: true, dry_run: dryRun })
    await loadCacheStats()
    emit('toast', dryRun ? 'Кэш сброшен. Запущен предпросмотр...' : 'Кэш сброшен. Запущена полная пересинхронизация!', 'success')
    emit('sync', dryRun)
  } catch (e) {
    emit('toast', 'Ошибка сброса кэша: ' + e, 'error')
  } finally {
    clearingCache.value = false
  }
}

async function checkUpdates(force = true) {
  checkingUpdate.value = true
  try {
    const data = await GetVersion(force)
    if (data) versionData.value = data
  } catch (e) {
    /* ignore */
  } finally {
    checkingUpdate.value = false
  }
}

function handleKeydown(e) {
  if ((e.ctrlKey || e.metaKey) && e.key === 's') {
    e.preventDefault()
    handleSave()
  }
}

const isMac = ref(false)

onMounted(() => {
  isMac.value = typeof navigator !== 'undefined' && (/Mac|iPhone|iPod|iPad/.test(navigator.platform) || /Macintosh|Mac OS X/.test(navigator.userAgent))
  checkUpdates(false)
  loadCacheStats()
  window.addEventListener('keydown', handleKeydown)
})

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeydown)
})

// Синхронизируем при изменении props
watch(() => props.config, (cfg) => {
  if (cfg) {
    form.torrent_dirs = Array.isArray(cfg.torrent_dirs) && cfg.torrent_dirs.length > 0
      ? [...cfg.torrent_dirs]
      : (cfg.torrents_dir ? [cfg.torrents_dir] : [''])
      
    form.library_dir = cfg.library_dir || cfg.shows_dir || ''
    form.sync_interval_minutes = cfg.sync_interval_minutes !== undefined ? cfg.sync_interval_minutes : 5
    form.folder_naming_mode = cfg.folder_naming_mode || 'russian'
    form.use_relative_symlinks = cfg.use_relative_symlinks !== undefined ? cfg.use_relative_symlinks : true

    // Прокси
    if (cfg.proxy_routing) {
      form.proxy_url = cfg.proxy_routing.url || cfg.proxy_url || ''
    } else {
      form.proxy_url = cfg.proxy_url || ''
    }

    // Собираем упорядоченный список провайдеров
    const savedList = Array.isArray(cfg.metadata_providers) ? cfg.metadata_providers : ['shikimori', 'anilist', 'anidb']
    const providerMap = new Map()

    // Инициализируем дефолты
    for (const p of ALL_PROVIDERS) {
      const useProxy = cfg.proxy_routing ? !!cfg.proxy_routing[p.id] : false
      providerMap.set(p.id, {
        id: p.id,
        enabled: savedList.includes(p.id),
        use_proxy: useProxy
      })
    }

    const ordered = []
    // Сначала добавляем сохраненные в их порядке
    for (const id of savedList) {
      if (providerMap.has(id)) {
        ordered.push(providerMap.get(id))
        providerMap.delete(id)
      }
    }
    // Затем оставшиеся (выключенные)
    for (const p of providerMap.values()) {
      ordered.push(p)
    }

    form.providers = ordered

    // Преобразуем map в массив для удобного редактирования
    const langList = []
    if (cfg.language_mapping && typeof cfg.language_mapping === 'object') {
      for (const [key, val] of Object.entries(cfg.language_mapping)) {
        langList.push({ key, val })
      }
    }
    form.language_mapping = langList
  }
}, { immediate: true, deep: true })

function getProviderMeta(id) {
  return ALL_PROVIDERS.find(p => p.id === id) || { name: id, badge: '', desc: '' }
}

function moveProviderUp(index) {
  if (index > 0) {
    const item = form.providers.splice(index, 1)[0]
    form.providers.splice(index - 1, 0, item)
  }
}

function moveProviderDown(index) {
  if (index < form.providers.length - 1) {
    const item = form.providers.splice(index, 1)[0]
    form.providers.splice(index + 1, 0, item)
  }
}

function addTorrentDir() {
  form.torrent_dirs.push('')
}

function removeTorrentDir(index) {
  if (form.torrent_dirs.length > 1) {
    form.torrent_dirs.splice(index, 1)
  } else {
    form.torrent_dirs[0] = ''
  }
}

function addLanguageMapping() {
  form.language_mapping.push({ key: '', val: 'ru' })
}

function removeLanguageMapping(index) {
  form.language_mapping.splice(index, 1)
}

function openFolderBrowser(field, initialPath = '') {
  currentModalField.value = field
  currentModalInitialPath.value = initialPath
  isModalShow.value = true
}

function handleFolderSelect(selectedPath) {
  if (currentModalField.value === 'library_dir') {
    form.library_dir = selectedPath
  } else if (currentModalField.value.startsWith('torrent_dir_')) {
    const index = parseInt(currentModalField.value.replace('torrent_dir_', ''), 10)
    if (!isNaN(index) && index >= 0 && index < form.torrent_dirs.length) {
      form.torrent_dirs[index] = selectedPath
    }
  }
}

function handleSave() {
  const cleanDirs = form.torrent_dirs.map(d => d.trim()).filter(d => d !== '')

  // Собираем языковой map
  const langMap = {}
  for (const item of form.language_mapping) {
    const k = item.key.trim()
    const v = item.val.trim()
    if (k && v) {
      langMap[k] = v
    }
  }

  // Собираем активные провайдеры в порядке приоритета
  const activeProviders = form.providers.filter(p => p.enabled).map(p => p.id)

  const proxyRouting = {
    url: form.proxy_url.trim(),
    shikimori: form.providers.find(p => p.id === 'shikimori')?.use_proxy || false,
    anilist: form.providers.find(p => p.id === 'anilist')?.use_proxy || false,
    anidb: form.providers.find(p => p.id === 'anidb')?.use_proxy || false
  }

  emit('save', {
    torrent_dirs: cleanDirs.length > 0 ? cleanDirs : [''],
    library_dir: form.library_dir.trim(),
    sync_interval_minutes: form.sync_interval_minutes,
    folder_naming_mode: form.folder_naming_mode,
    metadata_providers: activeProviders,
    proxy_routing: proxyRouting,
    proxy_url: form.proxy_url.trim(),
    use_shikimori: activeProviders.includes('shikimori'),
    use_relative_symlinks: form.use_relative_symlinks,
    language_mapping: langMap
  })
}
</script>

<template>
  <div class="settings-view-wrapper">
    <!-- Верхняя панель подвкладок настроек -->
    <div class="settings-subtabs-bar">
      <button
        class="settings-subtab-btn"
        :class="{ active: activeSubTab === 'storage' }"
        @click="activeSubTab = 'storage'"
        type="button"
      >
        <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>
        </svg>
        <span>Папки и хранилище</span>
      </button>

      <button
        class="settings-subtab-btn"
        :class="{ active: activeSubTab === 'metadata' }"
        @click="activeSubTab = 'metadata'"
        type="button"
      >
        <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="12" cy="12" r="10"></circle>
          <line x1="2" y1="12" x2="22" y2="12"></line>
          <path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"></path>
        </svg>
        <span>Метаданные и сеть</span>
      </button>

      <button
        class="settings-subtab-btn"
        :class="{ active: activeSubTab === 'maintenance' }"
        @click="activeSubTab = 'maintenance'"
        type="button"
      >
        <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2"></polygon>
        </svg>
        <span>Автоматизация и кэш</span>
      </button>
    </div>

    <!-- Вкладка 1: ПАПКИ И МЕДИАТЕКА -->
    <div v-show="activeSubTab === 'storage'" class="settings-tab-pane">
      <!-- Карточка: Папки торрентов -->
      <div class="card">
        <div class="card-header-row">
          <div class="card-icon-pill">
            <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/>
              <polyline points="7 10 12 15 17 10"/>
              <line x1="12" y1="15" x2="12" y2="3"/>
            </svg>
          </div>
          <div>
            <div class="card-title">Папки торрентов</div>
            <div class="card-subtitle">Пути к исходным раздачам аниме (можно указать несколько источников)</div>
          </div>
        </div>

        <div class="form-group">
          <label class="form-label">Папки, куда скачиваются раздачи:</label>

          <div
            v-for="(dir, index) in form.torrent_dirs"
            :key="index"
            class="input-row"
            style="margin-bottom: 10px;"
          >
            <input
              type="text"
              v-model="form.torrent_dirs[index]"
              placeholder="/media/Torrents/Anime"
              style="flex: 1;"
            />
            <button
              class="btn-icon"
              @click="openFolderBrowser('torrent_dir_' + index, form.torrent_dirs[index])"
              title="Выбрать папку на сервере"
              type="button"
            >
              <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>
              </svg>
            </button>
            <button
              class="btn-icon btn-icon-danger"
              @click="removeTorrentDir(index)"
              title="Удалить папку"
              type="button"
            >
              <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <polyline points="3 6 5 6 21 6"></polyline>
                <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path>
                <line x1="10" y1="11" x2="10" y2="17"></line>
                <line x1="14" y1="11" x2="14" y2="17"></line>
              </svg>
            </button>
          </div>

          <button
            class="btn btn-secondary btn-sm"
            @click="addTorrentDir"
            type="button"
          >
            <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <line x1="12" y1="5" x2="12" y2="19"></line>
              <line x1="5" y1="12" x2="19" y2="12"></line>
            </svg>
            Добавить папку
          </button>
        </div>
      </div>

      <!-- Карточка: Медиатека Jellyfin -->
      <div class="card">
        <div class="card-header-row">
          <div class="card-icon-pill accent">
            <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <rect x="2" y="2" width="20" height="8" rx="2" ry="2"/>
              <rect x="2" y="14" width="20" height="8" rx="2" ry="2"/>
              <line x1="6" y1="6" x2="6.01" y2="6"/>
              <line x1="6" y1="18" x2="6.01" y2="18"/>
            </svg>
          </div>
          <div>
            <div class="card-title">Медиатека Jellyfin</div>
            <div class="card-subtitle">Конечная директория, куда создаются аккуратные симлинки для Jellyfin</div>
          </div>
        </div>

        <div class="form-group">
          <label class="form-label">Папка медиатеки аниме в Jellyfin:</label>
          <div class="input-row">
            <input
              type="text"
              v-model="form.library_dir"
              placeholder="/media/Jellyfin/Anime"
            />
            <button
              class="btn-icon"
              @click="openFolderBrowser('library_dir', form.library_dir)"
              title="Выбрать папку медиатеки"
              type="button"
            >
              <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>
              </svg>
            </button>
          </div>
          <p class="form-help-text">
            Сериалы раскладываются по сезонам (<code>Season 01</code>, <code>Season 02</code>), а фильмы и спешлы — в <code>Season 00</code> с метаданными <code>.nfo</code>.
          </p>
        </div>

        <div class="library-notice-box">
          <svg class="notice-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="12" r="10"></circle>
            <line x1="12" y1="8" x2="12" y2="12"></line>
            <line x1="12" y1="16" x2="12.01" y2="16"></line>
          </svg>
          <div class="notice-text">
            <strong>Автоматический порядок:</strong> JellyAnLi автоматически удаляет устаревшие симлинки, битые ссылки и пустые папки при удалении или переименовании файлов в торрентах.
          </div>
        </div>
      </div>

      <!-- Карточка: Именование и симлинки -->
      <div class="card">
        <div class="card-header-row">
          <div class="card-icon-pill">
            <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"></path>
              <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"></path>
            </svg>
          </div>
          <div>
            <div class="card-title">Именование папок и симлинки</div>
            <div class="card-subtitle">Формат названий каталогов и тип создаваемых ссылок</div>
          </div>
        </div>

        <div class="form-group">
          <label class="form-label">Формат названий папок аниме:</label>
          <div class="naming-options-grid">
            <label
              class="naming-option-card"
              :class="{ selected: form.folder_naming_mode === 'russian' }"
              @click="form.folder_naming_mode = 'russian'"
            >
              <input type="radio" name="folder_naming_mode" value="russian" :checked="form.folder_naming_mode === 'russian'" style="display: none;" />
              <div class="naming-option-header">
                <span class="naming-flag">🇷🇺</span>
                <strong>Русские названия</strong>
              </div>
              <span class="naming-example">«Клинок, рассекающий демонов»</span>
            </label>

            <label
              class="naming-option-card"
              :class="{ selected: form.folder_naming_mode === 'romaji' }"
              @click="form.folder_naming_mode = 'romaji'"
            >
              <input type="radio" name="folder_naming_mode" value="romaji" :checked="form.folder_naming_mode === 'romaji'" style="display: none;" />
              <div class="naming-option-header">
                <span class="naming-flag">🇬🇧</span>
                <strong>Официальные Ромадзи</strong>
              </div>
              <span class="naming-example">«Kimetsu no Yaiba»</span>
            </label>

            <label
              class="naming-option-card"
              :class="{ selected: form.folder_naming_mode === 'original' }"
              @click="form.folder_naming_mode = 'original'"
            >
              <input type="radio" name="folder_naming_mode" value="original" :checked="form.folder_naming_mode === 'original'" style="display: none;" />
              <div class="naming-option-header">
                <span class="naming-flag">📁</span>
                <strong>Оригинал из раздачи</strong>
              </div>
              <span class="naming-example">Как назван каталог в торренте</span>
            </label>
          </div>
        </div>

        <div class="form-group" style="margin-top: 18px;">
          <label class="checkbox-row" @click.prevent="form.use_relative_symlinks = !form.use_relative_symlinks">
            <input type="checkbox" :checked="form.use_relative_symlinks" @click.stop="form.use_relative_symlinks = !form.use_relative_symlinks" />
            <div>
              <span class="checkbox-label">Использовать относительные симлинки</span>
              <div class="checkbox-subtext">Рекомендуется для Docker, NAS и CasaOS, чтобы ссылки не ломались при монтировании путей.</div>
            </div>
          </label>
        </div>
      </div>
    </div>

    <!-- Вкладка 2: МЕТАДАННЫЕ И СЕТЬ -->
    <div v-show="activeSubTab === 'metadata'" class="settings-tab-pane">
      <!-- Карточка: Провайдеры метаданных -->
      <div class="card">
        <div class="card-header-row">
          <div class="card-icon-pill accent">
            <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <circle cx="12" cy="12" r="10"></circle>
              <line x1="2" y1="12" x2="22" y2="12"></line>
              <path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"></path>
            </svg>
          </div>
          <div>
            <div class="card-title">Провайдеры метаданных и приоритет поиска</div>
            <div class="card-subtitle">Цепочка поиска информации об аниме (поиск выполняется сверху вниз)</div>
          </div>
        </div>

        <div class="providers-list">
          <div
            v-for="(p, index) in form.providers"
            :key="p.id"
            class="provider-item"
            :class="{ disabled: !p.enabled }"
          >
            <div class="provider-info-col">
              <input
                type="checkbox"
                v-model="p.enabled"
                :id="'prov_' + p.id"
                class="provider-checkbox"
              />
              <div>
                <div class="provider-name-row">
                  <label :for="'prov_' + p.id" class="provider-title">
                    {{ getProviderMeta(p.id).name }}
                  </label>
                  <span class="provider-badge">
                    {{ getProviderMeta(p.id).badge }}
                  </span>
                </div>
                <div class="provider-desc">
                  {{ getProviderMeta(p.id).desc }}
                </div>
              </div>
            </div>

            <div class="provider-actions-col">
              <!-- Тумблер Proxy -->
              <label class="provider-proxy-toggle" :class="{ disabled: !p.enabled }" title="Маршрутизировать запросы через Proxy">
                <input
                  type="checkbox"
                  v-model="p.use_proxy"
                  :disabled="!p.enabled"
                />
                <span>Через Proxy</span>
              </label>

              <!-- Кнопки перемещения приоритета -->
              <div class="reorder-btn-group">
                <button
                  class="btn-icon"
                  @click="moveProviderUp(index)"
                  :disabled="index === 0"
                  title="Повысить приоритет"
                  type="button"
                >
                  <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <polyline points="18 15 12 9 6 15"></polyline>
                  </svg>
                </button>
                <button
                  class="btn-icon"
                  @click="moveProviderDown(index)"
                  :disabled="index === form.providers.length - 1"
                  title="Понизить приоритет"
                  type="button"
                >
                  <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <polyline points="6 9 12 15 18 9"></polyline>
                  </svg>
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Карточка: Прокси-сервер (Proxy) -->
      <div class="card">
        <div class="card-header-row">
          <div class="card-icon-pill">
            <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <rect x="2" y="2" width="20" height="8" rx="2" ry="2"/>
              <rect x="2" y="14" width="20" height="8" rx="2" ry="2"/>
              <line x1="6" y1="6" x2="6.01" y2="6"/>
              <line x1="6" y1="18" x2="6.01" y2="18"/>
            </svg>
          </div>
          <div>
            <div class="card-title">Прокси-сервер (Proxy)</div>
            <div class="card-subtitle">Для обхода сетевых ограничений при обращении к базам метаданных</div>
          </div>
        </div>

        <div class="form-group">
          <label class="form-label">Адрес прокси (SOCKS5 / SOCKS5h / HTTP):</label>
          <div class="input-row">
            <input
              type="text"
              v-model="form.proxy_url"
              placeholder="socks5://127.0.0.1:1080 или http://proxy.local:8080"
              style="flex: 1;"
            />
          </div>
          <p class="form-help-text">
            Поддерживаются протоколы <code>socks5://</code>, <code>socks5h://</code> (с удаленным DNS) и <code>http://</code>. Включается отдельно для каждого провайдера выше.
          </p>
        </div>
      </div>

      <!-- Карточка: Языковые сопоставления -->
      <div class="card">
        <div class="card-header-row">
          <div class="card-icon-pill">
            <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="m5 8 6 6"/>
              <path d="m4 14 6-6 2-3"/>
              <path d="M2 5h12"/>
              <path d="M7 2h1"/>
              <path d="m22 22-5-10-5 10"/>
              <path d="M14 18h6"/>
            </svg>
          </div>
          <div>
            <div class="card-title">Языковые сопоставления (Аудио и субтитры)</div>
            <div class="card-subtitle">Привязка языковых меток (например, <code>.ru.mka</code>, <code>.en.ass</code>) по папкам в раздачах</div>
          </div>
        </div>

        <div class="form-group">
          <div
            v-for="(mapping, index) in form.language_mapping"
            :key="index"
            class="input-row"
            style="margin-bottom: 8px;"
          >
            <input
              type="text"
              v-model="mapping.key"
              placeholder="Ключевое слово (например: Rus sound, Eng subs)"
              style="flex: 2;"
            />
            <span class="mapping-arrow">➔</span>
            <input
              type="text"
              v-model="mapping.val"
              placeholder="Код (ru, en)"
              style="flex: 1; max-width: 110px;"
            />
            <button
              class="btn-icon btn-icon-danger"
              @click="removeLanguageMapping(index)"
              title="Удалить сопоставление"
              type="button"
            >
              <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <polyline points="3 6 5 6 21 6"></polyline>
                <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path>
              </svg>
            </button>
          </div>

          <button
            class="btn btn-secondary btn-sm"
            @click="addLanguageMapping"
            type="button"
          >
            <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <line x1="12" y1="5" x2="12" y2="19"></line>
              <line x1="5" y1="12" x2="19" y2="12"></line>
            </svg>
            Добавить сопоставление
          </button>
        </div>
      </div>
    </div>

    <!-- Вкладка 3: АВТОМАТИЗАЦИЯ И КЭШ -->
    <div v-show="activeSubTab === 'maintenance'" class="settings-tab-pane">
      <!-- Карточка: Кэш метаданных и состояние связей -->
      <div class="card">
        <div class="card-header-row">
          <div class="card-icon-pill accent">
            <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M21.5 2v6h-6M21.34 15.57a10 10 0 1 1-.57-8.38l5.67-5.67"/>
            </svg>
          </div>
          <div style="flex: 1;">
            <div style="display: flex; align-items: center; justify-content: space-between;">
              <div class="card-title">Кэш метаданных и состояние связей</div>
              <button
                class="btn-icon"
                @click="loadCacheStats"
                title="Обновить статистику кэша"
                type="button"
              >
                <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <polyline points="23 4 23 10 17 10"></polyline>
                  <polyline points="1 20 1 14 7 14"></polyline>
                  <path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"></path>
                </svg>
              </button>
            </div>
            <div class="card-subtitle">Сохранённые ответы баз аниме и кэш отслеживаемых файлов</div>
          </div>
        </div>

        <!-- Счётчики записей -->
        <div v-if="cacheStats" class="cache-stats-grid">
          <div class="cache-stat-card">
            <div class="cache-stat-label">Shikimori</div>
            <div class="cache-stat-value">{{ cacheStats.shikimori_count }}</div>
          </div>
          <div class="cache-stat-card">
            <div class="cache-stat-label">AniList</div>
            <div class="cache-stat-value">{{ cacheStats.anilist_count }}</div>
          </div>
          <div class="cache-stat-card">
            <div class="cache-stat-label">AniDB</div>
            <div class="cache-stat-value">{{ cacheStats.anidb_count }}</div>
          </div>
          <div class="cache-stat-card highlight">
            <div class="cache-stat-label">state.json</div>
            <div class="cache-stat-value">{{ cacheStats.state_files_count }}</div>
          </div>
        </div>

        <p class="form-help-text" style="margin-bottom: 14px;">
          При изменении названий в раздачах или базах данных сброс кэша позволяет мгновенно перекачать актуальные метаданные без ручной очистки файлов на сервере.
        </p>

        <!-- Кнопки очистки кэша -->
        <div class="cache-actions-row">
          <button
            class="btn btn-secondary btn-sm"
            @click="handleClearMetadataOnly"
            :disabled="clearingCache"
            type="button"
            title="Удаляет только кэш ответов Shikimori, AniList и AniDB"
          >
            <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <polyline points="3 6 5 6 21 6"/>
              <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/>
            </svg>
            Очистить кэш баз
          </button>

          <button
            class="btn btn-secondary btn-sm"
            @click="handleResyncWithClear(true)"
            :disabled="clearingCache || syncing"
            type="button"
            title="Очищает кэш и показывает предпросмотр изменений (Dry Run)"
          >
            <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <circle cx="11" cy="11" r="8"/>
              <line x1="21" y1="21" x2="16.65" y2="16.65"/>
            </svg>
            Сбросить кэш и Dry Run
          </button>

          <button
            class="btn btn-danger btn-sm"
            @click="handleResyncWithClear(false)"
            :disabled="clearingCache || syncing"
            type="button"
            title="Сбрасывает все кэши (базы + state.json) и запускает полную синхронизацию заново"
          >
            <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M21.5 2v6h-6M21.34 15.57a10 10 0 1 1-.57-8.38l5.67-5.67"/>
            </svg>
            Сбросить всё и перекачать мету
          </button>
        </div>
      </div>

      <!-- Карточка: Периодическая проверка -->
      <div class="card">
        <div class="card-header-row">
          <div class="card-icon-pill">
            <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <circle cx="12" cy="12" r="10"></circle>
              <polyline points="12 6 12 12 16 14"></polyline>
            </svg>
          </div>
          <div>
            <div class="card-title">Фоновое сканирование</div>
            <div class="card-subtitle">Периодическая проверка появления новых серий и раздач</div>
          </div>
        </div>

        <div class="form-group">
          <label class="form-label">Интервал автоматической фоновой синхронизации:</label>
          <div class="input-row" style="max-width: 200px;">
            <input
              type="number"
              v-model.number="form.sync_interval_minutes"
              min="0"
              max="1440"
              placeholder="10"
            />
            <span class="input-suffix">минут</span>
          </div>
          <p class="form-help-text">
            Укажите <code>0</code> для отключения автоматического фонового сканирования.
          </p>
        </div>
      </div>

      <!-- Карточка: О программе и обновления -->
      <div class="card" v-if="versionData">
        <div class="card-header-row">
          <div class="card-icon-pill">
            <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <circle cx="12" cy="12" r="10"></circle>
              <line x1="12" y1="16" x2="12" y2="12"></line>
              <line x1="12" y1="8" x2="12.01" y2="8"></line>
            </svg>
          </div>
          <div style="flex: 1;">
            <div style="display: flex; align-items: center; justify-content: space-between;">
              <div class="card-title">О программе и обновления</div>
              <span class="version-badge">
                {{ versionData.current_version }}
              </span>
            </div>
            <div class="card-subtitle">Проверка релизов на GitHub и статус обновлений</div>
          </div>
        </div>

        <div v-if="versionData.has_update" class="update-banner">
          <div class="update-banner-content">
            <div>
              <div class="update-title">
                🎉 Доступна новая версия {{ versionData.latest_version }}!
              </div>
              <div class="update-desc">
                Рекомендуется обновиться для получения свежих исправлений и улучшений.
              </div>
            </div>
            <a
              :href="versionData.release_url || 'https://github.com/JellyAnLi/JellyAnLi/releases'"
              target="_blank"
              rel="noopener noreferrer"
              class="btn btn-primary btn-sm"
              style="text-decoration: none;"
            >
              Открыть релиз на GitHub ➔
            </a>
          </div>
        </div>

        <div v-else class="version-status-row">
          <div class="version-status-left">
            <span class="version-check-icon">✓</span>
            <span>У вас установлена последняя актуальная версия ({{ versionData.current_version }}).</span>
          </div>
          <button
            class="btn btn-secondary btn-sm"
            @click="checkUpdates(true)"
            :disabled="checkingUpdate"
            type="button"
          >
            <span v-if="checkingUpdate" class="spinner" style="width: 12px; height: 12px; margin-right: 4px;"></span>
            {{ checkingUpdate ? 'Проверка...' : 'Проверить снова' }}
          </button>
        </div>
      </div>
    </div>

    <!-- Плавающая нижняя панель сохранения -->
    <div class="settings-save-bar">
      <div class="save-bar-hint">
        <span v-if="isMac">Нажмите <kbd>⌘</kbd> <kbd>S</kbd> для быстрого сохранения</span>
        <span v-else>Нажмите <kbd>Ctrl</kbd> + <kbd>S</kbd> для быстрого сохранения</span>
      </div>
      <button class="btn btn-primary btn-save" @click="handleSave">
        <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M19 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11l5 5v11a2 2 0 0 1-2 2z"/>
          <polyline points="17 21 17 13 7 13 7 21"/>
          <polyline points="7 3 7 8 15 8"/>
        </svg>
        Сохранить настройки
      </button>
    </div>

    <!-- Веб-диалог выбора папок -->
    <FolderBrowserModal
      :show="isModalShow"
      :initial-path="currentModalInitialPath"
      title="Выберите папку на сервере"
      @select="handleFolderSelect"
      @close="isModalShow = false"
    />
  </div>
</template>
