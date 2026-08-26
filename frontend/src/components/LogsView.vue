<script setup>
import { ref, computed, watch, nextTick } from 'vue'

const props = defineProps({
  logs: {
    type: Array,
    default: () => []
  },
  syncing: {
    type: Boolean,
    default: false
  }
})

const emit = defineEmits(['sync', 'dry-run', 'clear'])

const logContainer = ref(null)
const filterTab = ref('all') // 'all' | 'changes' | 'errors'
const displayMode = ref('tree') // 'tree' | 'console'
const searchQuery = ref('')
const autoScroll = ref(true)
const copied = ref(false)
const activeTooltip = ref(null)
const sourceCopied = ref(false)

// Развернутые папки в интерактивном дереве (по умолчанию ВСЁ СВЕРНУТО)
const expandedShows = ref(new Set())
const expandedSeasons = ref(new Set())

function toggleShow(name) {
  const next = new Set(expandedShows.value)
  if (next.has(name)) {
    next.delete(name)
  } else {
    next.add(name)
  }
  expandedShows.value = next
}

function toggleSeason(showName, seasonName) {
  const key = `${showName}/${seasonName}`
  const next = new Set(expandedSeasons.value)
  if (next.has(key)) {
    next.delete(key)
  } else {
    next.add(key)
  }
  expandedSeasons.value = next
}

function expandAll() {
  const shows = new Set()
  const seasons = new Set()
  parsedTree.value.forEach(s => {
    shows.add(s.name)
    s.seasons.forEach(sn => seasons.add(`${s.name}/${sn.name}`))
  })
  expandedShows.value = shows
  expandedSeasons.value = seasons
}

function collapseAll() {
  expandedShows.value = new Set()
  expandedSeasons.value = new Set()
}

function toggleTooltip(target, source, event) {
  if (activeTooltip.value && activeTooltip.value.target === target) {
    activeTooltip.value = null
    return
  }

  const rect = event.currentTarget.getBoundingClientRect()
  const isMobile = window.innerWidth <= 768

  if (isMobile) {
    activeTooltip.value = {
      target,
      source,
      isMobile: true
    }
  } else {
    let x = rect.left
    let y = rect.bottom + 8
    if (x + 390 > window.innerWidth) {
      x = window.innerWidth - 400
    }
    if (y + 130 > window.innerHeight) {
      y = rect.top - 120
    }
    activeTooltip.value = {
      target,
      source,
      x: Math.max(10, x),
      y: Math.max(10, y),
      isMobile: false
    }
  }
}

function closeTooltip() {
  activeTooltip.value = null
}

// Универсальная функция копирования
async function copyText(text) {
  if (navigator.clipboard && window.isSecureContext) {
    try {
      await navigator.clipboard.writeText(text)
      return true
    } catch (err) {
      console.warn('navigator.clipboard error, fallback to execCommand:', err)
    }
  }

  try {
    const textarea = document.createElement('textarea')
    textarea.value = text
    textarea.style.position = 'fixed'
    textarea.style.left = '-999999px'
    textarea.style.top = '-999999px'
    textarea.setAttribute('readonly', '')
    document.body.appendChild(textarea)
    textarea.focus()
    textarea.select()
    const successful = document.execCommand('copy')
    document.body.removeChild(textarea)
    return successful
  } catch (err) {
    console.error('Ошибка копирования:', err)
    return false
  }
}

async function copySource(source) {
  const ok = await copyText(source)
  if (ok) {
    sourceCopied.value = true
    setTimeout(() => {
      sourceCopied.value = false
    }, 1500)
  }
}

// Парсинг строки лога
function parseLogLine(line) {
  const timeRegex = /^\[(\d{2}:\d{2}:\d{2})\]\s(.*)$/
  const match = line.match(timeRegex)

  let timestamp = ''
  let content = line

  if (match) {
    timestamp = match[1]
    content = match[2]
  }

  const trimmed = content.trim()

  // 1. Корень аниме в дереве: 📁 ShowName
  if (trimmed.startsWith('📁')) {
    return {
      raw: line,
      timestamp,
      content: trimmed,
      isLink: false,
      isTree: true,
      isTreeFile: false,
      treeLevel: 0,
      type: 'tree'
    }
  }

  // 2. Папка сезона в дереве: ├── 📂 SeasonName или └── 📂 SeasonName
  if (trimmed.includes('📂')) {
    return {
      raw: line,
      timestamp,
      content: trimmed,
      isLink: false,
      isTree: true,
      isTreeFile: false,
      treeLevel: 1,
      type: 'tree'
    }
  }

  // 3. Файл в дереве или операция связывания
  if (content.includes('➔ (из ') || content.includes('➔ (из')) {
    const parts = content.split(/➔\s*\(из\s*/)
    if (parts.length >= 2) {
      let left = parts[0].trim()
      let source = parts[1].replace(/\)\s*$/, '').trim()

      if (left.includes('├──') || left.includes('└──')) {
        let branch = left.includes('├──') ? '├──' : '└──'

        let icon = '🎬'
        if (left.includes('🎬')) icon = '🎬'
        else if (left.includes('🎵')) icon = '🎵'
        else if (left.includes('💬')) icon = '💬'
        else {
          const ext = left.substring(left.lastIndexOf('.')).toLowerCase()
          if (ext === '.mka' || ext === '.flac' || ext === '.ac3') icon = '🎵'
          else if (ext === '.ass' || ext === '.srt' || ext === '.vtt') icon = '💬'
        }

        let targetFile = left
          .replace(/^[│├└─\s]+/, '')
          .replace(/^(?:🎬|🎵|💬)\s*/, '')
          .trim()

        return {
          raw: line,
          timestamp,
          content: '',
          isLink: false,
          isTree: true,
          isTreeFile: true,
          treeLevel: 2,
          treeBranch: branch,
          treeIcon: icon,
          treeTarget: targetFile,
          treeSource: source,
          type: 'tree'
        }
      } else {
        let target = left.replace(/^➔\s*/, '').trim()
        return {
          raw: line,
          timestamp,
          content: '',
          isLink: true,
          isTree: false,
          isTreeFile: false,
          treeLevel: 0,
          target,
          source,
          type: 'success'
        }
      }
    }
  }

  // Определение типа для остальных строк
  let type = 'info'
  if (trimmed.startsWith('🚀') || trimmed.startsWith('🔍') || trimmed.startsWith('✓') || trimmed.startsWith('✨')) {
    type = 'session'
  } else if (content.includes('Ошибка') || content.includes('ERROR')) {
    type = 'error'
  } else if (content.includes('Предупреждение') || content.includes('Внимание')) {
    type = 'warning'
  } else if (content.includes('Создана ссылка') || content.includes('Успех') || content.includes('Удалено:')) {
    type = 'success'
  }

  return {
    raw: line,
    timestamp,
    content: trimmed,
    isLink: false,
    isTree: false,
    isTreeFile: false,
    treeLevel: 0,
    type
  }
}

function splitTargetPath(target) {
  if (!target) return { folder: '', filename: '' }
  const parts = target.split('/')
  if (parts.length > 1) {
    const filename = parts.pop()
    const folder = parts.join('/') + '/'
    return { folder, filename }
  }
  return { folder: '', filename: target }
}

// Все распарсенные строки
const parsedLogs = computed(() => {
  return props.logs.map(line => parseLogLine(line))
})

// Подсчет статистики
const stats = computed(() => {
  let changes = 0
  let errors = 0
  for (const item of parsedLogs.value) {
    if (item.isLink || item.isTreeFile || item.content.includes('Создана ссылка') || item.content.includes('Удалено:') || item.content.includes('создано:')) {
      changes++
    }
    if (item.type === 'error' || item.type === 'warning') {
      errors++
    }
  }
  return {
    total: parsedLogs.value.length,
    changes,
    errors
  }
})

// Отфильтрованные логи
const filteredLogs = computed(() => {
  let result = parsedLogs.value

  if (filterTab.value === 'changes') {
    result = result.filter(item => 
      item.isLink || 
      item.isTreeFile ||
      item.content.includes('Создана ссылка') || 
      item.content.includes('Удалено:') || 
      item.type === 'session'
    )
  } else if (filterTab.value === 'errors') {
    result = result.filter(item => item.type === 'error' || item.type === 'warning')
  }

  if (searchQuery.value.trim() !== '') {
    const q = searchQuery.value.toLowerCase().trim()
    result = result.filter(item => {
      if (item.raw.toLowerCase().includes(q)) return true
      if (item.target && item.target.toLowerCase().includes(q)) return true
      if (item.treeTarget && item.treeTarget.toLowerCase().includes(q)) return true
      if (item.source && item.source.toLowerCase().includes(q)) return true
      if (item.treeSource && item.treeSource.toLowerCase().includes(q)) return true
      return false
    })
  }

  return result
})

// Построение интерактивного дерева папок (Collapsible Tree Model)
const parsedTree = computed(() => {
  const items = filteredLogs.value
  let startIndex = 0

  // Ищем начало последней сессии предпросмотра, чтобы дерево не дублировалось
  for (let i = items.length - 1; i >= 0; i--) {
    if (items[i].content.includes('Предпросмотр:')) {
      startIndex = i
      break
    }
  }

  const sessionItems = items.slice(startIndex)
  const shows = []
  let currentShow = null
  let currentSeason = null

  for (const item of sessionItems) {
    if (item.isTree && item.treeLevel === 0) {
      const showTitle = item.content.replace(/^📁\s*/, '').trim()
      currentShow = {
        name: showTitle,
        seasons: [],
        totalFiles: 0
      }
      shows.push(currentShow)
      currentSeason = null
    } else if (item.isTree && item.treeLevel === 1) {
      const seasonTitle = item.content.replace(/^[│├└─\s]*📂\s*/, '').trim()
      if (currentShow) {
        currentSeason = {
          name: seasonTitle,
          files: []
        }
        currentShow.seasons.push(currentSeason)
      }
    } else if (item.isTreeFile) {
      if (currentSeason) {
        currentSeason.files.push({
          icon: item.treeIcon,
          target: item.treeTarget,
          source: item.treeSource,
          branch: item.treeBranch
        })
        if (currentShow) currentShow.totalFiles++
      } else if (currentShow) {
        if (!currentShow.seasons.length) {
          currentSeason = { name: 'Корневые файлы', files: [] }
          currentShow.seasons.push(currentSeason)
        }
        currentShow.seasons[0].files.push({
          icon: item.treeIcon,
          target: item.treeTarget,
          source: item.treeSource,
          branch: item.treeBranch
        })
        currentShow.totalFiles++
      }
    }
  }

  return shows
})

const hasTreeData = computed(() => parsedTree.value.length > 0)

const totalTreeFiles = computed(() => {
  return parsedTree.value.reduce((acc, s) => acc + s.totalFiles, 0)
})

// Автоскролл
watch(() => props.logs.length, async () => {
  if (autoScroll.value && displayMode.value === 'console') {
    await nextTick()
    scrollToBottom()
  }
})

function scrollToBottom() {
  if (logContainer.value) {
    logContainer.value.scrollTop = logContainer.value.scrollHeight
  }
}

function scrollToTop() {
  if (logContainer.value) {
    logContainer.value.scrollTop = 0
  }
}

// Копирование логов в буфер
async function copyLogs() {
  const textToCopy = filteredLogs.value.map(item => item.raw).join('\n')
  const ok = await copyText(textToCopy)
  if (ok) {
    copied.value = true
    setTimeout(() => {
      copied.value = false
    }, 2000)
  }
}

function formatShows(count) {
  const mod10 = count % 10
  const mod100 = count % 100
  if (mod100 >= 11 && mod100 <= 19) return `${count} тайтлов`
  if (mod10 === 1) return `${count} тайтл`
  if (mod10 >= 2 && mod10 <= 4) return `${count} тайтла`
  return `${count} тайтлов`
}

function formatSeasons(count) {
  const mod10 = count % 10
  const mod100 = count % 100
  if (mod100 >= 11 && mod100 <= 19) return `${count} сезонов`
  if (mod10 === 1) return `${count} сезон`
  if (mod10 >= 2 && mod10 <= 4) return `${count} сезона`
  return `${count} сезонов`
}

function formatFiles(count) {
  const mod10 = count % 10
  const mod100 = count % 100
  if (mod100 >= 11 && mod100 <= 19) return `${count} файлов`
  if (mod10 === 1) return `${count} файл`
  if (mod10 >= 2 && mod10 <= 4) return `${count} файла`
  return `${count} файлов`
}
</script>

<template>
  <div class="log-header">
    <div class="log-header-top">
      <div>
        <h2>Журнал работы</h2>
        <p>Отчеты о созданных симлинках, структуре библиотеки и ошибках</p>
      </div>

      <div class="btn-group">
        <button class="btn btn-primary" :disabled="syncing" @click="emit('sync')">
          <template v-if="syncing">
            <span class="spinner"></span>
            Синхронизация...
          </template>
          <template v-else>
            <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <polygon points="5 3 19 12 5 21 5 3"/>
            </svg>
            Синхронизировать
          </template>
        </button>

        <button class="btn btn-secondary" :disabled="syncing" @click="emit('dry-run')">
          <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="11" cy="11" r="8"/>
            <line x1="21" y1="21" x2="16.65" y2="16.65"/>
          </svg>
          Dry Run
        </button>

        <button class="btn btn-danger" @click="emit('clear')" title="Очистить лог">
          <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <polyline points="3 6 5 6 21 6"/>
            <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/>
          </svg>
          Очистить
        </button>
      </div>
    </div>

    <!-- Панель фильтров, поиска и переключения режимов -->
    <div class="log-controls-bar">
      <!-- Строка 1: Табы фильтрации -->
      <div class="filter-pills">
        <button 
          class="pill-btn" 
          :class="{ active: filterTab === 'all' }" 
          @click="filterTab = 'all'"
        >
          Все
          <span class="pill-badge">{{ stats.total }}</span>
        </button>

        <button 
          class="pill-btn" 
          :class="{ active: filterTab === 'changes' }" 
          @click="filterTab = 'changes'"
        >
          Изменения
          <span class="pill-badge badge-success">{{ stats.changes }}</span>
        </button>

        <button 
          class="pill-btn" 
          :class="{ active: filterTab === 'errors' }" 
          @click="filterTab = 'errors'"
        >
          Ошибки
          <span class="pill-badge" :class="stats.errors > 0 ? 'badge-error' : ''">{{ stats.errors }}</span>
        </button>
      </div>

      <!-- Строка 2: Кнопки режимов и действий -->
      <div class="log-actions-group">
        <!-- Переключатель: Дерево папок ⇄ Консоль -->
        <div v-if="hasTreeData" class="mode-switch-group">
          <button 
            class="mode-btn" 
            :class="{ active: displayMode === 'tree' }"
            @click="displayMode = 'tree'"
            title="Интерактивное дерево с раскрывающимися папками"
          >
            <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"></path>
            </svg>
            <span>Дерево</span>
          </button>
          <button 
            class="mode-btn" 
            :class="{ active: displayMode === 'console' }"
            @click="displayMode = 'console'"
            title="Консольный текстовый вывод"
          >
            <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <polyline points="4 17 10 11 4 5"></polyline>
              <line x1="12" y1="19" x2="20" y2="19"></line>
            </svg>
            <span>Консоль</span>
          </button>
        </div>

        <button 
          v-if="displayMode === 'console'"
          class="tool-btn" 
          :class="{ active: autoScroll }" 
          @click="autoScroll = !autoScroll"
          :title="autoScroll ? 'Автоскролл включен' : 'Автоскролл выключен'"
        >
          <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M12 5v14M19 12l-7 7-7-7"/>
          </svg>
          <span class="btn-text">Автоскролл</span>
        </button>

        <button class="tool-btn" @click="copyLogs" title="Скопировать журнал">
          <svg v-if="!copied" class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <rect x="9" y="9" width="13" height="13" rx="2" ry="2"/>
            <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>
          </svg>
          <svg v-else class="icon icon-success" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <polyline points="20 6 9 17 4 12"/>
          </svg>
          <span class="btn-text">{{ copied ? 'Скопировано!' : 'Копия' }}</span>
        </button>
      </div>

      <!-- Строка 3: Полноразмерный поиск -->
      <div class="search-input-wrapper">
        <svg class="search-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="11" cy="11" r="8"/>
          <line x1="21" y1="21" x2="16.65" y2="16.65"/>
        </svg>
        <input 
          type="text" 
          v-model="searchQuery" 
          placeholder="Поиск тайтла или файла..." 
          class="search-input"
        />
        <button v-if="searchQuery" class="clear-search-btn" @click="searchQuery = ''">✕</button>
      </div>
    </div>
  </div>

  <!-- РЕЖИМ 1: Интерактивное дерево с раскрывающимися папками (Tree Explorer) -->
  <div v-if="hasTreeData && displayMode === 'tree'" class="tree-explorer-container">
    <div class="tree-explorer-toolbar">
      <div class="tree-summary-group">
        <span class="summary-badge">📁 {{ formatShows(parsedTree.length) }}</span>
        <span class="summary-badge-files">🎬 {{ formatFiles(totalTreeFiles) }}</span>
      </div>
      <div class="tree-action-btns">
        <button class="tree-mini-btn" @click="expandAll">Развернуть всё</button>
        <button class="tree-mini-btn" @click="collapseAll">Свернуть всё</button>
      </div>
    </div>

    <div class="tree-cards-list">
      <div 
        v-for="show in parsedTree" 
        :key="show.name" 
        class="show-tree-card"
      >
        <!-- Заголовок тайтла (раскрывается по клику, по умолчанию свёрнут) -->
        <div class="show-tree-header" @click="toggleShow(show.name)">
          <div class="show-title-left">
            <span class="tree-chevron" :class="{ 'is-expanded': expandedShows.has(show.name) }">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="9 18 15 12 9 6"></polyline></svg>
            </span>
            <span class="show-folder-icon">📁</span>
            <span class="show-name-text">{{ show.name }}</span>
          </div>
          <div class="show-meta-pill">
            <span v-if="show.seasons.length > 1">{{ formatSeasons(show.seasons.length) }} • </span>
            <span>{{ formatFiles(show.totalFiles) }}</span>
          </div>
        </div>

        <!-- Тело тайтла с сезонами -->
        <div v-show="expandedShows.has(show.name)" class="show-tree-body">
          <div 
            v-for="season in show.seasons" 
            :key="season.name"
            class="season-tree-block"
          >
            <!-- Заголовок сезона (раскрывается по клику) -->
            <div class="season-tree-header" @click="toggleSeason(show.name, season.name)">
              <div class="season-title-left">
                <span class="season-chevron" :class="{ 'is-expanded': expandedSeasons.has(`${show.name}/${season.name}`) }">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="9 18 15 12 9 6"></polyline></svg>
                </span>
                <span class="season-folder-icon">📂</span>
                <span class="season-name-text">{{ season.name }}</span>
              </div>
              <span class="season-meta-pill">{{ formatFiles(season.files.length) }}</span>
            </div>

            <!-- Список файлов сезона -->
            <div v-show="expandedSeasons.has(`${show.name}/${season.name}`)" class="season-files-grid">
              <div 
                v-for="file in season.files" 
                :key="file.target"
                class="tree-file-row"
                :class="{ 'is-active': activeTooltip && activeTooltip.target === file.target }"
                @click.stop="toggleTooltip(file.target, file.source, $event)"
              >
                <span class="tree-file-icon">{{ file.icon }}</span>
                <span class="tree-file-title">{{ file.target }}</span>
                <span class="tree-file-info-icon">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <circle cx="12" cy="12" r="10"></circle>
                    <line x1="12" y1="16" x2="12" y2="12"></line>
                    <line x1="12" y1="8" x2="12.01" y2="8"></line>
                  </svg>
                </span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>

  <!-- РЕЖИМ 2: Классический терминал логов (с оптимизацией Safari content-visibility) -->
  <div v-else class="log-area" ref="logContainer">
    <template v-if="filteredLogs.length > 0">
      <div
        v-for="(line, i) in filteredLogs"
        :key="i"
        class="log-line"
        :class="line.type"
      >
        <template v-if="line.timestamp && !line.isTree && !line.isTreeFile">
          <span class="timestamp">[{{ line.timestamp }}]</span>
          <span class="log-spacer"> </span>
        </template>

        <!-- Tree File line -->
        <template v-if="line.isTreeFile">
          <span class="tree-line-row level-2">
            <span class="tree-branch">{{ line.treeBranch }}</span>
            <span 
              class="log-tree-file-wrapper" 
              :class="{ 'is-active': activeTooltip && activeTooltip.target === line.treeTarget }"
              @click.stop="toggleTooltip(line.treeTarget, line.treeSource, $event)"
            >
              <span class="log-tree-icon">{{ line.treeIcon }}</span>
              <span class="log-tree-filename">{{ line.treeTarget }}</span>
              <span class="log-tree-source-tag">
                <svg class="source-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <circle cx="12" cy="12" r="10"></circle>
                  <line x1="12" y1="16" x2="12" y2="12"></line>
                  <line x1="12" y1="8" x2="12.01" y2="8"></line>
                </svg>
              </span>
            </span>
          </span>
        </template>

        <!-- Standard Link line -->
        <template v-else-if="line.isLink">
          <span class="log-link-badge">LINK</span>
          <span v-if="splitTargetPath(line.target).folder" class="log-link-folder">
            📁 {{ splitTargetPath(line.target).folder }}
          </span>
          <span 
            class="log-tree-file-wrapper"
            :class="{ 'is-active': activeTooltip && activeTooltip.target === splitTargetPath(line.target).filename }"
            @click.stop="toggleTooltip(splitTargetPath(line.target).filename, line.source, $event)"
          >
            <span class="log-tree-filename">{{ splitTargetPath(line.target).filename }}</span>
            <span class="log-tree-source-tag">
              <svg class="source-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <circle cx="12" cy="12" r="10"></circle>
                <line x1="12" y1="16" x2="12" y2="12"></line>
                <line x1="12" y1="8" x2="12.01" y2="8"></line>
              </svg>
            </span>
          </span>
        </template>

        <!-- Tree Show root or Season folder line -->
        <template v-else-if="line.isTree">
          <span 
            class="tree-line-row" 
            :class="{ 
              'level-0': line.treeLevel === 0, 
              'level-1': line.treeLevel === 1,
              'tree-header': line.content.includes('Иерархия')
            }"
          >
            <span>{{ line.content }}</span>
          </span>
        </template>

        <!-- Other lines -->
        <template v-else>
          <span>{{ line.content }}</span>
        </template>
      </div>
    </template>
    <div v-else class="log-empty">
      <template v-if="logs.length === 0">
        Журнал пуст. Запустите синхронизацию, чтобы увидеть результаты.
      </template>
      <template v-else>
        По текущему фильтру или поисковому запросу ничего не найдено.
      </template>
    </div>
  </div>

  <!-- Всплывающий Tippy Tooltip по клику -->
  <Transition name="tooltip-pop">
    <div 
      v-if="activeTooltip" 
      class="tippy-tooltip"
      :class="{ 'mobile-sheet': activeTooltip.isMobile }"
      :style="!activeTooltip.isMobile ? { top: activeTooltip.y + 'px', left: activeTooltip.x + 'px' } : {}"
      @click.stop
    >
      <div class="tippy-header">
        <div class="tippy-tag">
          <svg class="tippy-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"/>
            <path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"/>
          </svg>
          Симлинк Jellyfin
        </div>
        <button class="tippy-close-btn" @click="closeTooltip" title="Закрыть">✕</button>
      </div>

      <div class="tippy-target">{{ activeTooltip.target }}</div>
      
      <div class="tippy-divider"></div>
      
      <div class="tippy-source-box">
        <div class="tippy-source-header">
          <span class="tippy-source-label">Исходный файл раздачи:</span>
          <button class="tippy-copy-btn" @click="copySource(activeTooltip.source)" title="Скопировать исходный путь">
            <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <rect x="9" y="9" width="13" height="13" rx="2" ry="2"/>
              <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>
            </svg>
            <span>{{ sourceCopied ? 'Скопировано!' : 'Копировать' }}</span>
          </button>
        </div>
        <div class="tippy-source-val">{{ activeTooltip.source }}</div>
      </div>
    </div>
  </Transition>

  <!-- Затемнение / прозрачный фон для закрытия по клику вне -->
  <div 
    v-if="activeTooltip" 
    :class="activeTooltip.isMobile ? 'tippy-mobile-overlay' : 'tippy-backdrop'" 
    @click="closeTooltip"
  ></div>
</template>

<style scoped>
.log-header-top {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
  margin-bottom: 12px;
  flex-wrap: wrap;
}

.log-header h2 {
  font-size: 18px;
  font-weight: 700;
  color: var(--text-primary);
  margin-bottom: 2px;
}

.log-header p {
  font-size: 12px;
  color: var(--text-muted);
}

/* Панель фильтров */
.log-controls-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  margin-bottom: 12px;
  flex-wrap: wrap;
}

.controls-top-row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.filter-pills {
  display: flex;
  gap: 6px;
  align-items: center;
}

.pill-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 5px 10px;
  border-radius: 20px;
  background: var(--bg-card-solid);
  border: 1px solid var(--border-color);
  color: var(--text-secondary);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  transition: var(--transition);
  user-select: none;
}

.pill-btn:hover {
  background: var(--bg-hover);
  color: var(--text-primary);
}

.pill-btn.active {
  background: var(--bg-active);
  border-color: var(--accent);
  color: var(--text-accent);
}

.pill-badge {
  font-size: 10px;
  font-weight: 600;
  padding: 1px 6px;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.1);
  color: var(--text-secondary);
}

.pill-btn.active .pill-badge {
  background: var(--accent);
  color: white;
}

.pill-badge.badge-success {
  background: rgba(34, 197, 94, 0.2);
  color: var(--success);
}

.pill-badge.badge-error {
  background: rgba(239, 68, 68, 0.2);
  color: var(--error);
}

/* Действия и режимы */
.log-actions-group {
  display: flex;
  align-items: center;
  gap: 8px;
}

.mode-switch-group {
  display: inline-flex;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-sm);
  background: var(--bg-card-solid);
  overflow: hidden;
}

.mode-btn {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 4px 9px;
  height: 30px;
  background: none;
  border: none;
  color: var(--text-secondary);
  font-size: 11.5px;
  font-weight: 500;
  cursor: pointer;
  transition: var(--transition);
}

.mode-btn:hover {
  color: var(--text-primary);
  background: var(--bg-hover);
}

.mode-btn.active {
  background: var(--accent);
  color: #ffffff;
}

.mode-btn .icon {
  width: 13px;
  height: 13px;
}

.search-input-wrapper {
  position: relative;
  display: flex;
  align-items: center;
  width: 220px;
}

.search-icon {
  position: absolute;
  left: 10px;
  width: 14px;
  height: 14px;
  color: var(--text-muted);
  pointer-events: none;
}

.search-input {
  width: 100%;
  padding: 5px 24px 5px 30px !important;
  font-size: 12px !important;
  height: 30px !important;
  min-height: 30px !important;
  border-radius: 16px !important;
}

.clear-search-btn {
  position: absolute;
  right: 8px;
  background: none;
  border: none;
  color: var(--text-muted);
  font-size: 10px;
  cursor: pointer;
}

.tool-btn {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 5px 9px;
  height: 30px;
  border-radius: var(--radius-sm);
  background: var(--bg-card-solid);
  border: 1px solid var(--border-color);
  color: var(--text-secondary);
  font-size: 11.5px;
  font-weight: 500;
  cursor: pointer;
  transition: var(--transition);
}

.tool-btn:hover {
  background: var(--bg-hover);
  color: var(--text-primary);
}

.tool-btn.active {
  color: var(--text-accent);
  border-color: var(--border-focus);
  background: var(--bg-active);
}

.tool-btn .icon {
  width: 14px;
  height: 14px;
}

.icon-success {
  color: var(--success);
}

/* =======================================================
   ИНТЕРАКТИВНОЕ ДЕРЕВО ПАПОК (Tree Explorer Cards)
======================================================= */
.tree-explorer-container {
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-height: 250px;
}

.tree-explorer-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 12px;
  background: var(--bg-card-solid);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  gap: 8px;
}

.tree-summary-group {
  display: flex;
  align-items: center;
  gap: 6px;
}

.summary-badge {
  font-size: 12px;
  font-weight: 600;
  color: #38bdf8;
  background: rgba(56, 189, 248, 0.1);
  padding: 3px 8px;
  border-radius: 6px;
}

.summary-badge-files {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-accent);
  background: rgba(59, 130, 246, 0.1);
  padding: 3px 8px;
  border-radius: 6px;
}

.tree-action-btns {
  display: flex;
  gap: 6px;
}

.tree-mini-btn {
  background: var(--bg-hover);
  border: 1px solid var(--border-color);
  color: var(--text-primary);
  font-size: 11.5px;
  font-weight: 500;
  padding: 4px 10px;
  border-radius: var(--radius-sm);
  cursor: pointer;
  transition: var(--transition);
  white-space: nowrap;
}

.tree-mini-btn:hover {
  background: var(--bg-active);
  border-color: var(--border-focus);
  color: var(--text-accent);
}

.tree-cards-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.show-tree-card {
  background: var(--bg-card-solid);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  overflow: hidden;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.15);
  transition: border-color 0.2s ease;
}

.show-tree-card:hover {
  border-color: var(--border-focus);
}

.show-tree-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 9px 12px;
  background: rgba(255, 255, 255, 0.02);
  cursor: pointer;
  user-select: none;
  transition: background 0.15s ease;
  gap: 8px;
}

.show-tree-header:hover {
  background: var(--bg-hover);
}

.show-title-left {
  display: flex;
  align-items: center;
  gap: 7px;
  flex: 1 1 auto;
  min-width: 0;
}

.tree-chevron {
  width: 14px;
  height: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-muted);
  flex-shrink: 0;
  transition: transform 0.2s ease;
}

.tree-chevron svg {
  width: 12px;
  height: 12px;
}

.tree-chevron.is-expanded {
  transform: rotate(90deg);
}

.show-folder-icon {
  font-size: 15px;
  flex-shrink: 0;
}

.show-name-text {
  font-size: 13px;
  font-weight: 700;
  color: #38bdf8;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.show-meta-pill {
  flex-shrink: 0;
  font-size: 11px;
  font-weight: 600;
  color: var(--text-secondary);
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.08);
  padding: 3px 8px;
  border-radius: 12px;
  white-space: nowrap;
}

.show-tree-body {
  padding: 8px 12px 10px 12px;
  border-top: 1px solid var(--border-color);
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.season-tree-block {
  border-left: 2px solid rgba(192, 132, 252, 0.3);
  padding-left: 8px;
  margin-left: 4px;
}

.season-tree-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 5px 8px;
  border-radius: var(--radius-sm);
  cursor: pointer;
  user-select: none;
  transition: background 0.15s ease;
  gap: 6px;
}

.season-tree-header:hover {
  background: var(--bg-hover);
}

.season-title-left {
  display: flex;
  align-items: center;
  gap: 6px;
  flex: 1 1 auto;
  min-width: 0;
}

.season-chevron {
  width: 12px;
  height: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-muted);
  flex-shrink: 0;
  transition: transform 0.2s ease;
}

.season-chevron svg {
  width: 10px;
  height: 10px;
}

.season-chevron.is-expanded {
  transform: rotate(90deg);
}

.season-folder-icon {
  font-size: 13px;
  flex-shrink: 0;
}

.season-name-text {
  font-size: 12px;
  font-weight: 600;
  color: #c084fc;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.season-meta-pill {
  flex-shrink: 0;
  font-size: 10.5px;
  color: var(--text-muted);
  background: rgba(255, 255, 255, 0.04);
  padding: 2px 6px;
  border-radius: 8px;
  white-space: nowrap;
}

.season-files-grid {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 4px 0 4px 14px;
}

.tree-file-row {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 3px 8px;
  border-radius: 4px;
  cursor: pointer;
  transition: background 0.12s ease;
  user-select: none;
  content-visibility: auto;
  contain-intrinsic-size: 24px;
}

.tree-file-row:hover {
  background: rgba(59, 130, 246, 0.15);
}

.tree-file-row.is-active {
  background: rgba(59, 130, 246, 0.25);
}

.tree-file-icon {
  font-size: 13px;
  flex-shrink: 0;
}

.tree-file-title {
  font-size: 11.5px;
  color: var(--text-primary);
  font-weight: 500;
  font-family: var(--font-mono);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.tree-file-row:hover .tree-file-title {
  color: var(--text-accent);
}

.tree-file-info-icon {
  margin-left: auto;
  opacity: 0.4;
  color: var(--text-muted);
  display: flex;
  align-items: center;
  flex-shrink: 0;
}

.tree-file-info-icon svg {
  width: 13px;
  height: 13px;
}

.tree-file-row:hover .tree-file-info-icon {
  opacity: 1;
  color: var(--text-accent);
}

/* =======================================================
   КОНСОЛЬНЫЕ СТИЛИ (Terminal Console Styles)
======================================================= */
.log-line {
  content-visibility: auto;
  contain-intrinsic-size: 22px;
}

.tree-line-row {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  line-height: 1.6;
  white-space: nowrap;
}

.tree-line-row.level-0 {
  padding-left: 0;
  margin-top: 4px;
  font-size: 13px;
  font-weight: 700;
  color: #38bdf8;
}

.tree-line-row.level-1 {
  padding-left: 14px;
  font-weight: 600;
  color: #c084fc;
}

.tree-line-row.level-2 {
  padding-left: 28px;
}

.tree-line-row.tree-header {
  color: var(--text-accent);
  font-weight: 700;
  margin: 4px 0 2px 0;
}

.tree-branch {
  color: var(--text-muted);
  font-family: var(--font-mono);
  user-select: none;
  opacity: 0.7;
}

.log-tree-file-wrapper {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  cursor: pointer;
  padding: 1px 6px;
  border-radius: 4px;
  background-color: transparent;
  transition: background-color 0.15s ease;
  vertical-align: middle;
  user-select: none;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
  transform: translateZ(0);
}

.log-tree-file-wrapper:hover {
  background-color: rgba(59, 130, 246, 0.15);
}

.log-tree-icon {
  font-size: 13px;
  line-height: 1;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  width: 16px;
}

.log-tree-filename {
  color: var(--text-primary);
  font-weight: 500;
  line-height: 1.4;
  white-space: nowrap;
}

.log-tree-source-tag {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: var(--text-muted);
  opacity: 0.5;
  flex-shrink: 0;
  width: 14px;
  height: 14px;
  min-width: 14px;
  min-height: 14px;
  pointer-events: none;
  transition: opacity 0.15s ease, color 0.15s ease;
}

.log-tree-file-wrapper:hover .log-tree-source-tag {
  opacity: 1;
  color: var(--text-accent);
}

.source-icon {
  width: 14px;
  height: 14px;
  min-width: 14px;
  min-height: 14px;
  display: block;
  flex-shrink: 0;
  pointer-events: none;
}

/* Tippy Tooltip Popup */
.tippy-tooltip {
  position: fixed;
  z-index: 10000;
  width: 380px;
  max-width: calc(100vw - 24px);
  background: var(--bg-card-solid);
  border: 1px solid var(--border-focus);
  border-radius: var(--radius-md);
  padding: 12px 14px;
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.55), 0 0 15px var(--accent-glow);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  pointer-events: auto;
}

.tippy-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 6px;
}

.tippy-tag {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: var(--text-accent);
}

.tippy-icon {
  width: 12px;
  height: 12px;
}

.tippy-close-btn {
  background: none;
  border: none;
  color: var(--text-muted);
  font-size: 14px;
  cursor: pointer;
  padding: 2px 6px;
  border-radius: 4px;
}

.tippy-close-btn:hover {
  color: var(--text-primary);
}

.tippy-target {
  font-size: 12.5px;
  font-weight: 600;
  color: var(--text-primary);
  word-break: break-word;
  line-height: 1.4;
}

.tippy-divider {
  height: 1px;
  background: var(--border-color);
  margin: 8px 0;
}

.tippy-source-box {
  background: var(--bg-input);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-sm);
  padding: 8px 10px;
}

.tippy-source-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 5px;
}

.tippy-source-label {
  font-size: 10.5px;
  color: var(--text-muted);
  font-weight: 500;
}

.tippy-copy-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  background: var(--bg-card-solid);
  border: 1px solid var(--border-color);
  border-radius: 4px;
  padding: 2px 7px;
  font-size: 10.5px;
  color: var(--text-secondary);
  cursor: pointer;
  transition: var(--transition);
}

.tippy-copy-btn:hover {
  background: var(--bg-hover);
  color: var(--text-primary);
  border-color: var(--border-focus);
}

.tippy-copy-btn .icon {
  width: 11px;
  height: 11px;
}

.tippy-source-val {
  font-family: var(--font-mono);
  font-size: 11px;
  color: var(--text-secondary);
  word-break: break-all;
  line-height: 1.4;
  user-select: text;
}

.tippy-backdrop {
  position: fixed;
  inset: 0;
  z-index: 9999;
  background: transparent;
}

.log-tree-file-wrapper.is-active {
  background: rgba(59, 130, 246, 0.2);
  border-radius: 4px;
}

/* Mobile Bottom Sheet Tooltip */
.tippy-tooltip.mobile-sheet {
  left: 12px !important;
  right: 12px !important;
  bottom: calc(68px + env(safe-area-inset-bottom, 12px)) !important;
  top: auto !important;
  width: auto !important;
  max-width: none !important;
  border-radius: var(--radius-lg);
  box-shadow: 0 -4px 30px rgba(0, 0, 0, 0.6);
  z-index: 10001;
}

.tippy-mobile-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.4);
  backdrop-filter: blur(2px);
  z-index: 10000;
}

.tooltip-pop-enter-active, .tooltip-pop-leave-active {
  transition: all 0.15s ease;
}
.tooltip-pop-enter-from, .tooltip-pop-leave-to {
  opacity: 0;
  transform: translateY(4px) scale(0.98);
}

@media (max-width: 768px) {
  .timestamp {
    display: none;
  }

  .log-spacer {
    display: none;
  }

  .tree-line-row.level-1 {
    padding-left: 8px;
  }

  .tree-line-row.level-2 {
    padding-left: 14px;
    gap: 4px;
  }

  .log-tree-filename {
    font-size: 11px;
  }

  .log-header-top {
    flex-direction: column;
    gap: 10px;
  }

  .log-controls-bar {
    flex-direction: column;
    align-items: stretch;
    gap: 8px;
  }

  /* Строка 1: Табы фильтрации (на всю ширину без скролла) */
  .filter-pills {
    display: flex;
    width: 100%;
    gap: 6px;
    overflow: hidden;
  }

  .pill-btn {
    flex: 1;
    justify-content: center;
    padding: 6px 4px;
    font-size: 11.5px;
    border-radius: var(--radius-sm);
    gap: 4px;
  }

  /* Строка 2: Режимы и утилиты (на всю ширину без скролла) */
  .log-actions-group {
    display: flex;
    justify-content: space-between;
    align-items: center;
    width: 100%;
    gap: 8px;
  }

  .mode-switch-group {
    display: flex;
    flex: 1;
  }

  .mode-btn {
    flex: 1;
    justify-content: center;
    height: 32px;
    font-size: 12px;
    padding: 0 8px;
  }

  .tool-btn {
    height: 32px;
    padding: 0 10px;
    font-size: 12px;
    flex-shrink: 0;
  }

  .tool-btn .btn-text {
    display: inline;
    font-size: 11.5px;
  }

  /* Строка 3: Полноразмерный поиск (100% ширины) */
  .search-input-wrapper {
    width: 100%;
    min-width: 100%;
  }

  .search-input {
    width: 100%;
    height: 34px !important;
    min-height: 34px !important;
    font-size: 13px !important;
  }

  /* Tree Toolbar on Mobile */
  .tree-explorer-toolbar {
    flex-direction: column;
    align-items: stretch;
    padding: 8px 10px;
    gap: 8px;
  }

  .tree-summary-group {
    display: flex;
    justify-content: flex-start;
    align-items: center;
    gap: 6px;
    width: 100%;
  }

  .summary-badge, .summary-badge-files {
    font-size: 11.5px;
    padding: 3px 8px;
    flex: 1;
    text-align: center;
  }

  .tree-action-btns {
    display: flex;
    width: 100%;
    gap: 6px;
  }

  .tree-mini-btn {
    flex: 1;
    text-align: center;
    justify-content: center;
    font-size: 11.5px;
    padding: 5px 8px;
    height: 30px;
    display: inline-flex;
    align-items: center;
  }

  .show-tree-header {
    padding: 8px 10px;
    gap: 6px;
  }

  .show-name-text {
    font-size: 12px;
  }

  .show-meta-pill {
    font-size: 10.5px;
    padding: 2px 7px;
  }

  .season-tree-header {
    padding: 4px 6px;
  }

  .season-name-text {
    font-size: 11.5px;
  }

  .season-meta-pill {
    font-size: 10px;
  }

  .tree-file-title {
    font-size: 11px;
  }
}
</style>
