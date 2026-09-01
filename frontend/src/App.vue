<script setup>
import {ref, onMounted, onUnmounted, nextTick} from 'vue'
import SettingsView from './components/SettingsView.vue'
import LogsView from './components/LogsView.vue'
import { GetConfig, SaveConfig, RunSync, GetLogs, ClearLogs, GetVersion, ClearCache, subscribeEvents } from './api.js'

const activeTab = ref('settings')
const syncing = ref(false)
const serverOnline = ref(true)
const logs = ref([])
const versionInfo = ref({
  current_version: 'v1.0.0',
  latest_version: 'v1.0.0',
  has_update: false,
  release_url: 'https://github.com/JellyAnLi/JellyAnLi/releases'
})
const config = ref({
  torrent_dirs: [],
  library_dir: '',
  sync_interval_minutes: 5,
  use_shikimori: true,
  use_relative_symlinks: true,
  language_mapping: {}
})

const toast = ref(null)
let toastTimeout = null

function showToast(message, type = 'success') {
  if (toastTimeout) clearTimeout(toastTimeout)
  toast.value = {message, type}
  toastTimeout = setTimeout(() => {
    toast.value = null
  }, 3000)
}

// Загрузка конфигурации из Go
async function loadConfig() {
  try {
    const cfg = await GetConfig()
    if (cfg) {
      config.value = cfg
    }
  } catch (e) {
    console.error('Ошибка загрузки конфигурации:', e)
  }
}

// Загрузка накопленных логов
async function loadLogs() {
  try {
    const existingLogs = await GetLogs()
    if (existingLogs && existingLogs.length) {
      logs.value = existingLogs
    }
  } catch (e) {
    console.error('Ошибка загрузки логов:', e)
  }
}

// Загрузка информации о версии и обновлениях
async function loadVersion() {
  try {
    const info = await GetVersion()
    if (info) {
      versionInfo.value = info
    }
  } catch (e) {
    /* ignore offline */
  }
}

// Сохранение настроек
async function handleSave(cfg) {
  if (!serverOnline.value) {
    showToast('Сервер недоступен. Дождитесь переподключения.', 'error')
    return
  }
  try {
    await SaveConfig(cfg)
    config.value = {...cfg}
    showToast('Настройки успешно сохранены!', 'success')
  } catch (e) {
    showToast('Ошибка сохранения: ' + e, 'error')
  }
}

// Запуск синхронизации
async function handleSync(dryRun = false) {
  if (!serverOnline.value) {
    showToast('Сервер недоступен. Дождитесь переподключения.', 'error')
    return
  }
  if (syncing.value) return
  try {
    syncing.value = true
    await RunSync(dryRun)
    showToast(dryRun ? 'Запущен предпросмотр...' : 'Синхронизация запущена!', 'success')
  } catch (e) {
    syncing.value = false
    console.error('Ошибка синхронизации:', e)
    showToast('Ошибка запуска синхронизации: ' + e, 'error')
  }
}

// Очистка логов
async function handleClearLogs() {
  logs.value = []
  try {
    await ClearLogs()
  } catch (e) { /* ignore */
  }
}

// Сброс кэша и пересинхронизация
async function handleResyncClear(dryRun = false) {
  if (!serverOnline.value) {
    showToast('Сервер недоступен. Дождитесь переподключения.', 'error')
    return
  }
  if (syncing.value) return
  try {
    syncing.value = true
    await ClearCache({ clear_metadata: true, clear_state: true, resync: true, dry_run: dryRun })
    showToast(dryRun ? 'Кэш сброшен. Запущен предпросмотр...' : 'Кэш сброшен. Запущена полная пересинхронизация!', 'success')
    activeTab.value = 'logs'
  } catch (e) {
    syncing.value = false
    showToast('Ошибка сброса кэша: ' + e, 'error')
  }
}

const themeMode = ref('dark') // 'dark' | 'light' | 'auto'

function applyTheme(mode) {
  const isLight = mode === 'light' || (mode === 'auto' && window.matchMedia && !window.matchMedia('(prefers-color-scheme: dark)').matches)
  if (isLight) {
    document.documentElement.classList.add('light-theme')
    document.body.classList.add('light-theme')
  } else {
    document.documentElement.classList.remove('light-theme')
    document.body.classList.remove('light-theme')
  }
}

function setTheme(mode) {
  themeMode.value = mode
  localStorage.setItem('theme-mode', mode)
  applyTheme(mode)
}

function toggleTheme() {
  if (themeMode.value === 'dark') {
    setTheme('light')
  } else if (themeMode.value === 'light') {
    setTheme('auto')
  } else {
    setTheme('dark')
  }
}

let unsubscribeEvents = null

onMounted(async () => {
  const savedMode = localStorage.getItem('theme-mode') || 'dark'
  themeMode.value = savedMode
  applyTheme(savedMode)

  try {
    window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
      if (themeMode.value === 'auto') {
        applyTheme('auto')
      }
    })
  } catch (e) {
    /* ignore */
  }

  await loadConfig()
  await loadLogs()
  await loadVersion()

  // Real-time стриминг логов и статуса через единый Server-Sent Events канал
  unsubscribeEvents = subscribeEvents({
    onConnected: async ({ isReconnect }) => {
      const wasOffline = !serverOnline.value
      serverOnline.value = true
      if (isReconnect || wasOffline) {
        showToast('🟢 Связь с сервером восстановлена, данные обновлены!', 'success')
        // Тихо обновляем данные без перезагрузки всей страницы
        await loadConfig()
        await loadLogs()
        await loadVersion()
      }
    },
    onDisconnected: () => {
      serverOnline.value = false
    },
    onLog: (newLog) => {
      logs.value.push(newLog)
      if (logs.value.length > 5000) {
        logs.value = logs.value.slice(logs.value.length - 5000)
      }
    },
    onStatus: (isSyncing) => {
      syncing.value = isSyncing
    },
    onReset: async () => {
      await loadLogs()
    }
  })
})

onUnmounted(() => {
  if (unsubscribeEvents) {
    unsubscribeEvents()
  }
})
</script>

<template>
  <div class="app-layout">
    <!-- Global Disconnected Warning Banner -->
    <Transition name="banner-slide">
      <div v-if="!serverOnline" class="server-offline-banner">
        <span class="offline-pulse-dot"></span>
        <span class="offline-text">Сервер JellyAnLi недоступен. Ожидание переподключения...</span>
      </div>
    </Transition>
    <!-- Mobile Header (visible only on mobile) -->
    <header class="mobile-header">
      <div class="mobile-logo">
        <img src="/logo.png" alt="JellyAnLi" class="app-brand-icon" />
        <span class="logo-text">JellyAnLi</span>
      </div>

      <div class="mobile-header-actions">
        <!-- Status indicator badge -->
        <div 
          class="mobile-status-badge" 
          :class="{ syncing: syncing && serverOnline, offline: !serverOnline }"
          :title="!serverOnline ? 'Сервер отключен' : (syncing ? 'Синхронизация...' : 'Сервер активен')"
        >
          <div class="status-dot" :class="{ syncing: syncing && serverOnline, offline: !serverOnline }"></div>
          <span class="mobile-status-text">{{ !serverOnline ? 'Офлайн' : (syncing ? 'Синхр...' : 'Активен') }}</span>
        </div>

        <!-- Theme toggle icon button -->
        <button class="btn-icon mobile-theme-btn" @click="toggleTheme" title="Сменить тему">
          <svg v-if="themeMode === 'dark'" class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor"
               stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/>
          </svg>
          <svg v-else-if="themeMode === 'light'" class="icon" viewBox="0 0 24 24" fill="none"
               stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="12" r="5"/>
            <line x1="12" y1="1" x2="12" y2="3"/>
            <line x1="12" y1="21" x2="12" y2="23"/>
            <line x1="4.22" y1="4.22" x2="5.64" y2="5.64"/>
            <line x1="18.36" y1="18.36" x2="19.78" y2="19.78"/>
            <line x1="1" y1="12" x2="3" y2="12"/>
            <line x1="21" y1="12" x2="23" y2="12"/>
            <line x1="4.22" y1="19.78" x2="5.64" y2="18.36"/>
            <line x1="18.36" y1="5.64" x2="19.78" y2="4.22"/>
          </svg>
          <svg v-else class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"
               stroke-linecap="round" stroke-linejoin="round">
            <rect x="2" y="3" width="20" height="14" rx="2" ry="2"/>
            <line x1="8" y1="21" x2="16" y2="21"/>
            <line x1="12" y1="17" x2="12" y2="21"/>
          </svg>
        </button>
      </div>
    </header>

    <!-- Desktop Sidebar (hidden on mobile) -->
    <aside class="sidebar">
      <div class="sidebar-header">
        <div class="sidebar-logo">
          <img src="/logo.png" alt="JellyAnLi" class="app-brand-icon" />
          <div class="brand-text-col">
            <span class="logo-text">JellyAnLi</span>
            <span class="logo-tagline">Anime Linker</span>
          </div>
        </div>
      </div>

      <nav class="sidebar-nav">
        <span class="sidebar-section-label">Разделы</span>

        <button
          class="nav-item"
          :class="{ active: activeTab === 'settings' }"
          @click="activeTab = 'settings'"
        >
          <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"
               stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="12" r="3"/>
            <path
              d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"/>
          </svg>
          Настройки
        </button>

        <button
          class="nav-item"
          :class="{ active: activeTab === 'logs' }"
          @click="activeTab = 'logs'"
        >
          <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"
               stroke-linecap="round" stroke-linejoin="round">
            <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
            <polyline points="14 2 14 8 20 8"/>
            <line x1="16" y1="13" x2="8" y2="13"/>
            <line x1="16" y1="17" x2="8" y2="17"/>
            <polyline points="10 9 9 9 8 9"/>
          </svg>
          Журнал
        </button>
      </nav>

      <div class="sidebar-footer">
        <!-- Status Badge -->
        <div class="sidebar-status" :class="{ offline: !serverOnline }">
          <div class="status-dot" :class="{ syncing: syncing && serverOnline, offline: !serverOnline }"></div>
          <span class="status-text">
            {{ !serverOnline ? 'Сервер отключен' : (syncing ? 'Синхронизация...' : 'Служба активна') }}
          </span>
        </div>

        <!-- Segmented Theme Switcher -->
        <div class="sidebar-theme-switch" title="Выбор темы оформления">
          <button
            class="theme-switch-btn"
            :class="{ active: themeMode === 'dark' }"
            @click="setTheme('dark')"
            title="Тёмная тема"
            type="button"
          >
            <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/>
            </svg>
          </button>
          <button
            class="theme-switch-btn"
            :class="{ active: themeMode === 'light' }"
            @click="setTheme('light')"
            title="Светлая тема"
            type="button"
          >
            <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <circle cx="12" cy="12" r="5"/>
              <line x1="12" y1="1" x2="12" y2="3"/>
              <line x1="12" y1="21" x2="12" y2="23"/>
              <line x1="4.22" y1="4.22" x2="5.64" y2="5.64"/>
              <line x1="18.36" y1="18.36" x2="19.78" y2="19.78"/>
              <line x1="1" y1="12" x2="3" y2="12"/>
              <line x1="21" y1="12" x2="23" y2="12"/>
              <line x1="4.22" y1="19.78" x2="5.64" y2="18.36"/>
              <line x1="18.36" y1="5.64" x2="19.78" y2="4.22"/>
            </svg>
          </button>
          <button
            class="theme-switch-btn"
            :class="{ active: themeMode === 'auto' }"
            @click="setTheme('auto')"
            title="Системная тема"
            type="button"
          >
            <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <rect x="2" y="3" width="20" height="14" rx="2" ry="2"/>
              <line x1="8" y1="21" x2="16" y2="21"/>
              <line x1="12" y1="17" x2="12" y2="21"/>
            </svg>
          </button>
        </div>
        
        <div class="sidebar-footer-links">
          <a
            :href="versionInfo.has_update ? versionInfo.release_url : 'https://github.com/JellyAnLi/JellyAnLi'"
            target="_blank"
            rel="noopener noreferrer"
            class="footer-github-link"
            :class="{ 'has-update': versionInfo.has_update }"
            :title="versionInfo.has_update ? `Доступна новая версия ${versionInfo.latest_version}! Нажмите, чтобы открыть релиз` : 'Репозиторий проекта на GitHub'"
          >
            <div v-if="versionInfo.has_update" class="update-dot"></div>
            <svg v-else class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M9 19c-5 1.5-5-2.5-7-3m14 6v-3.87a3.37 3.37 0 0 0-.94-2.61c3.14-.35 6.44-1.54 6.44-7A5.44 5.44 0 0 0 20 4.77 5.07 5.07 0 0 0 19.91 1S18.73.65 16 2.48a13.38 13.38 0 0 0-7 0C6.27.65 5.09 1 5.09 1A5.07 5.07 0 0 0 5 4.77a5.44 5.44 0 0 0-1.5 3.78c0 5.42 3.3 6.61 6.44 7A3.37 3.37 0 0 0 9 18.13V22"></path>
            </svg>
            <span v-if="versionInfo.has_update">
              {{ versionInfo.current_version }} → <b>{{ versionInfo.latest_version }} 🔥</b>
            </span>
            <span v-else>
              GitHub {{ versionInfo.current_version }}
            </span>
          </a>
        </div>
      </div>
    </aside>

    <!-- Main Content -->
    <main class="content">
      <SettingsView
        v-if="activeTab === 'settings'"
        :config="config"
        :syncing="syncing"
        :server-online="serverOnline"
        @save="handleSave"
        @sync="handleSync"
        @toast="showToast"
      />
      <LogsView
        v-else-if="activeTab === 'logs'"
        :logs="logs"
        :syncing="syncing"
        @sync="handleSync(false)"
        @dry-run="handleSync(true)"
        @clear="handleClearLogs"
        @resync-clear="handleResyncClear(false)"
      />
    </main>

    <!-- Mobile Bottom Navigation (visible only on mobile) -->
    <nav class="mobile-bottom-nav">
      <button
        class="mobile-nav-item"
        :class="{ active: activeTab === 'settings' }"
        @click="activeTab = 'settings'"
      >
        <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"
             stroke-linecap="round" stroke-linejoin="round">
          <circle cx="12" cy="12" r="3"/>
          <path
            d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"/>
        </svg>
        <span>Настройки</span>
      </button>

      <button
        class="mobile-nav-item"
        :class="{ active: activeTab === 'logs' }"
        @click="activeTab = 'logs'"
      >
        <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"
             stroke-linecap="round" stroke-linejoin="round">
          <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
          <polyline points="14 2 14 8 20 8"/>
          <line x1="16" y1="13" x2="8" y2="13"/>
          <line x1="16" y1="17" x2="8" y2="17"/>
          <polyline points="10 9 9 9 8 9"/>
        </svg>
        <span>Журнал</span>
      </button>
    </nav>
  </div>

  <!-- Toast Notification -->
  <Transition name="fade">
    <div v-if="toast" class="toast" :class="toast.type">
      {{ toast.message }}
    </div>
  </Transition>
</template>
