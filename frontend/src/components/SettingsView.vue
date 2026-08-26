<script setup>
import { reactive, watch, ref } from 'vue'
import FolderBrowserModal from './FolderBrowserModal.vue'

const props = defineProps({
  config: {
    type: Object,
    required: true
  }
})

const emit = defineEmits(['save'])

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

// Синхронизируем при изменении props
watch(() => props.config, (cfg) => {
  if (cfg) {
    form.torrent_dirs = Array.isArray(cfg.torrent_dirs) && cfg.torrent_dirs.length > 0
      ? [...cfg.torrent_dirs]
      : ['']
    form.library_dir = cfg.library_dir || ''
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
  <!-- Карточка: Папки торрентов -->
  <div class="card">
    <div class="card-title">Папки торрентов</div>
    <div class="card-subtitle">Пути к исходным раздачам аниме (можно указать несколько источников)</div>

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
          placeholder="/path/to/torrents"
          style="flex: 1;"
        />
        <button
          class="btn-icon"
          @click="openFolderBrowser('torrent_dir_' + index, form.torrent_dirs[index])"
          title="Выбрать папку"
          type="button"
        >
          <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>
          </svg>
        </button>
        <button
          class="btn-icon"
          @click="removeTorrentDir(index)"
          title="Удалить папку"
          type="button"
          style="border-color: rgba(239, 68, 68, 0.2); color: rgb(239, 68, 68);"
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
        class="btn btn-secondary"
        @click="addTorrentDir"
        type="button"
        style="margin-top: 5px; font-size: 0.85rem; padding: 6px 12px;"
      >
        <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="width: 14px; height: 14px; margin-right: 4px;">
          <line x1="12" y1="5" x2="12" y2="19"></line>
          <line x1="5" y1="12" x2="19" y2="12"></line>
        </svg>
        Добавить папку
      </button>
    </div>
  </div>

  <!-- Карточка: Библиотека Jellyfin -->
  <div class="card">
    <div class="card-title">Библиотека Jellyfin</div>
    <div class="card-subtitle">Конечная директория для создания структуры симлинков</div>

    <div class="form-group">
      <label class="form-label">Путь к общей папке медиатеки аниме:</label>
      <div class="input-row">
        <input
          type="text"
          v-model="form.library_dir"
          placeholder="/path/to/jellyfin/media"
        />
        <button
          class="btn-icon"
          @click="openFolderBrowser('library_dir', form.library_dir)"
          title="Выбрать папку"
          type="button"
        >
          <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>
          </svg>
        </button>
      </div>
      <p style="font-size: 0.8rem; color: var(--text-secondary); margin-top: 6px; line-height: 1.4;">
        В этой папке будут создаваться директории аниме,
        внутри которых серии будут разложены по сезонам (<code>Season 01</code>, <code>Season 02</code>),
        а полнометражные фильмы — в подпапку <code>Films</code>.
      </p>
      <div class="library-notice-box">
        <svg class="notice-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="12" cy="12" r="10"></circle>
          <line x1="12" y1="8" x2="12" y2="12"></line>
          <line x1="12" y1="16" x2="12.01" y2="16"></line>
        </svg>
        <div class="notice-text">
          <strong>Внимание:</strong> В этой папке порядок полностью наводит JellyAnLi. Сервис автоматически удаляет недействительные симлинки, устаревшие ссылки и пустые каталоги.
        </div>
      </div>
    </div>
  </div>

  <!-- Карточка: Именование папок в медиатеке -->
  <div class="card">
    <div class="card-title">Именование папок в медиатеке</div>
    <div class="card-subtitle">Формат названий тайтлов для каталога Jellyfin</div>

    <div class="form-group">
      <div style="display: flex; flex-direction: column; gap: 8px; margin-top: 4px;">
        <label class="checkbox-row" style="padding: 4px 0;" @click.prevent="form.folder_naming_mode = 'russian'">
          <input type="radio" name="folder_naming_mode" value="russian" :checked="form.folder_naming_mode === 'russian'" @click.stop="form.folder_naming_mode = 'russian'" />
          <span class="checkbox-label">🇷🇺 <strong>Русские названия</strong> (например, <em>«Клинок, рассекающий демонов»</em>)</span>
        </label>
        <label class="checkbox-row" style="padding: 4px 0;" @click.prevent="form.folder_naming_mode = 'romaji'">
          <input type="radio" name="folder_naming_mode" value="romaji" :checked="form.folder_naming_mode === 'romaji'" @click.stop="form.folder_naming_mode = 'romaji'" />
          <span class="checkbox-label">🇬🇧 <strong>Официальные Ромадзи</strong> (например, <em>«Kimetsu no Yaiba»</em>)</span>
        </label>
        <label class="checkbox-row" style="padding: 4px 0;" @click.prevent="form.folder_naming_mode = 'original'">
          <input type="radio" name="folder_naming_mode" value="original" :checked="form.folder_naming_mode === 'original'" @click.stop="form.folder_naming_mode = 'original'" />
          <span class="checkbox-label">📁 <strong>Оригинальные из раздач</strong> (как назван торрент, без переименования)</span>
        </label>
      </div>
    </div>

    <div class="form-group" style="margin-top: 14px;">
      <label class="checkbox-row" @click.prevent="form.use_relative_symlinks = !form.use_relative_symlinks">
        <input type="checkbox" :checked="form.use_relative_symlinks" @click.stop="form.use_relative_symlinks = !form.use_relative_symlinks" />
        <span class="checkbox-label">Использовать относительные симлинки (рекомендуется для Docker, NAS, CasaOS)</span>
      </label>
    </div>
  </div>

  <!-- Карточка: Провайдеры метаданных и приоритеты поиска -->
  <div class="card">
    <div class="card-title">Провайдеры метаданных и приоритеты поиска</div>
    <div class="card-subtitle">Цепочка поиска информации об аниме (поиск идет сверху вниз по списку)</div>

    <div style="display: flex; flex-direction: column; gap: 8px; margin-top: 10px;">
      <div
        v-for="(p, index) in form.providers"
        :key="p.id"
        style="display: flex; align-items: center; justify-content: space-between; padding: 10px 14px; background: var(--bg-card-solid); border: 1px solid var(--border-color); border-radius: var(--radius-md); gap: 12px;"
      >
        <div style="display: flex; align-items: center; gap: 12px; flex: 1;">
          <input
            type="checkbox"
            v-model="p.enabled"
            :id="'prov_' + p.id"
            style="width: 17px; height: 17px; cursor: pointer;"
          />
          <div>
            <div style="display: flex; align-items: center; gap: 8px;">
              <label :for="'prov_' + p.id" style="font-weight: 600; font-size: 13.5px; cursor: pointer; color: var(--text-primary);">
                {{ getProviderMeta(p.id).name }}
              </label>
              <span style="font-size: 11px; padding: 2px 6px; background: rgba(59, 130, 246, 0.15); color: var(--text-accent); border-radius: 4px; font-weight: 500;">
                {{ getProviderMeta(p.id).badge }}
              </span>
            </div>
            <div style="font-size: 11.5px; color: var(--text-secondary); margin-top: 2px;">
              {{ getProviderMeta(p.id).desc }}
            </div>
          </div>
        </div>

        <div style="display: flex; align-items: center; gap: 14px;">
          <!-- Тумблер прокси для провайдера -->
          <label class="checkbox-row" style="padding: 0; font-size: 12px; gap: 6px;" title="Маршрутизировать запросы к этому сервису через Proxy">
            <input
              type="checkbox"
              v-model="p.use_proxy"
              :disabled="!p.enabled"
            />
            <span style="color: var(--text-secondary); font-size: 12px;">Через Proxy</span>
          </label>

          <!-- Кнопки перемещения приоритета -->
          <div style="display: flex; gap: 4px;">
            <button
              class="btn-icon"
              @click="moveProviderUp(index)"
              :disabled="index === 0"
              title="Повысить приоритет"
              type="button"
              style="padding: 6px; width: 28px; height: 28px;"
            >
              <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="width: 14px; height: 14px;">
                <polyline points="18 15 12 9 6 15"></polyline>
              </svg>
            </button>
            <button
              class="btn-icon"
              @click="moveProviderDown(index)"
              :disabled="index === form.providers.length - 1"
              title="Понизить приоритет"
              type="button"
              style="padding: 6px; width: 28px; height: 28px;"
            >
              <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="width: 14px; height: 14px;">
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
    <div class="card-title">Прокси-сервер (Proxy)</div>
    <div class="card-subtitle">Для обхода региональных ограничений при обращении к базам метаданных</div>

    <div class="form-group">
      <label class="form-label">Адрес прокси (HTTP / HTTPS / SOCKS5 / SOCKS5h):</label>
      <div class="input-row">
        <input
          type="text"
          v-model="form.proxy_url"
          placeholder="socks5://127.0.0.1:1080 или http://proxy.local:8080"
          style="flex: 1;"
        />
      </div>
      <p style="font-size: 0.8rem; color: var(--text-secondary); margin-top: 6px; line-height: 1.4;">
        Поддерживаются протоколы <code>socks5://</code>, <code>socks5h://</code> (с удаленным DNS-резолвом через прокси) и <code>http://</code>. Включается персонально для каждого провайдера выше.
      </p>
    </div>
  </div>

  <!-- Карточка: Языковые сопоставления (Language Mappings) -->
  <div class="card">
    <div class="card-title">Языковые сопоставления (Аудио и субтитры)</div>
    <div class="card-subtitle">Определение языковых суффиксов (.ru.mka, .en.ass) по ключевым словам в путях раздач</div>

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
          placeholder="Ключевое слово (RUS Sound, Dub, etc.)"
          style="flex: 2;"
        />
        <span style="color: var(--text-muted); font-size: 13px;">➔</span>
        <input
          type="text"
          v-model="mapping.val"
          placeholder="Код языка (ru, en)"
          style="flex: 1; max-width: 110px;"
        />
        <button
          class="btn-icon"
          @click="removeLanguageMapping(index)"
          title="Удалить сопоставление"
          type="button"
          style="border-color: rgba(239, 68, 68, 0.2); color: rgb(239, 68, 68);"
        >
          <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <polyline points="3 6 5 6 21 6"></polyline>
            <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path>
          </svg>
        </button>
      </div>

      <button
        class="btn btn-secondary"
        @click="addLanguageMapping"
        type="button"
        style="margin-top: 6px; font-size: 0.85rem; padding: 6px 12px;"
      >
        <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="width: 14px; height: 14px; margin-right: 4px;">
          <line x1="12" y1="5" x2="12" y2="19"></line>
          <line x1="5" y1="12" x2="19" y2="12"></line>
        </svg>
        Добавить язык
      </button>
    </div>
  </div>

  <!-- Карточка: Периодическая проверка -->
  <div class="card">
    <div class="card-title">Периодическая проверка</div>
    <div class="card-subtitle">Автоматическая фоновая работа сервиса</div>

    <div class="form-group">
      <label class="form-label">Интервал между автоматическими фоновыми синхронизациями:</label>
      <div class="input-row">
        <input
          type="number"
          v-model.number="form.sync_interval_minutes"
          min="0"
          max="1440"
          style="max-width: 120px;"
        />
        <span class="input-suffix">минут (0 для отключения фонового таймера)</span>
      </div>
    </div>
  </div>

  <!-- Кнопка сохранения -->
  <div class="btn-save-wrapper">
    <button class="btn btn-primary" @click="handleSave">
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
</template>
