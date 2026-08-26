<script setup>
import { ref, watch, onMounted } from 'vue'
import { Browse } from '../api.js'

const props = defineProps({
  show: {
    type: Boolean,
    required: true
  },
  initialPath: {
    type: String,
    default: ''
  },
  title: {
    type: String,
    default: 'Выбор папки'
  }
})

const emit = defineEmits(['select', 'close'])

const currentPath = ref('')
const parentPath = ref('')
const directories = ref([])
const loading = ref(false)
const error = ref('')

async function loadDirectory(path) {
  loading.value = true
  error.value = ''
  try {
    const data = await Browse(path)
    currentPath.value = data.current_path
    parentPath.value = data.parent_path
    directories.value = data.directories || []
  } catch (e) {
    error.value = 'Ошибка загрузки папки: ' + e.message
  } finally {
    loading.value = false
  }
}

// Загружаем при первом показе
watch(() => props.show, (newVal) => {
  if (newVal) {
    loadDirectory(props.initialPath)
  }
})

function selectFolder() {
  emit('select', currentPath.value)
  emit('close')
}

function handleOverlayClick(e) {
  if (e.target.classList.contains('modal-overlay')) {
    emit('close')
  }
}
</script>

<template>
  <Transition name="modal-fade">
    <div v-if="show" class="modal-overlay" @click="handleOverlayClick">
      <div class="modal-window">
        <!-- Заголовок -->
        <div class="modal-header">
          <div class="modal-title">{{ title }}</div>
          <button class="btn-close-modal" @click="emit('close')">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="width: 20px; height: 20px;">
              <line x1="18" y1="6" x2="6" y2="18"></line>
              <line x1="6" y1="6" x2="18" y2="18"></line>
            </svg>
          </button>
        </div>

        <!-- Контент -->
        <div class="modal-body">
          <!-- Текущий путь -->
          <div class="current-path-row">
            <span class="path-label">Путь:</span>
            <input 
              type="text" 
              v-model="currentPath" 
              class="path-input" 
              @keyup.enter="loadDirectory(currentPath)" 
              placeholder="/абсолютный/путь"
            />
            <button class="btn btn-secondary" @click="loadDirectory(currentPath)" style="padding: 8px 12px; font-size: 0.85rem;">
              Перейти
            </button>
          </div>

          <!-- Список папок -->
          <div class="folder-list-container">
            <div v-if="loading" class="folder-list-message">
              <span class="spinner" style="margin-right: 8px;"></span>
              Загрузка файловой системы...
            </div>
            
            <div v-else-if="error" class="folder-list-error">
              {{ error }}
            </div>
            
            <div v-else class="folder-list">
              <!-- Кнопка "Назад" -->
              <div 
                v-if="parentPath" 
                class="folder-item parent-link" 
                @click="loadDirectory(parentPath)"
              >
                <div class="folder-icon">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" style="width: 16px; height: 16px; transform: scaleX(-1);">
                    <path d="M9 17l6-6-6-6"></path>
                  </svg>
                </div>
                <span class="folder-name">.. (Наверх)</span>
              </div>

              <!-- Список папок -->
              <div 
                v-for="dir in directories" 
                :key="dir" 
                class="folder-item"
                @click="loadDirectory(currentPath + '/' + dir)"
              >
                <div class="folder-icon">
                  <svg viewBox="0 0 24 24" fill="currentColor" stroke="none" style="width: 16px; height: 16px; color: var(--accent);">
                    <path d="M10 4H4c-1.11 0-1.99.89-1.99 2L2 18c0 1.11.89 2 2 2h16c1.11 0 2-.89 2-2V8c0-1.11-.89-2-2-2h-8l-2-2z"></path>
                  </svg>
                </div>
                <span class="folder-name">{{ dir }}</span>
              </div>

              <div v-if="directories.length === 0 && !parentPath" class="folder-list-empty">
                Папки не найдены
              </div>
            </div>
          </div>
        </div>

        <!-- Подвал с кнопками действия -->
        <div class="modal-footer">
          <button class="btn btn-secondary" @click="emit('close')">
            Отмена
          </button>
          <button class="btn btn-primary" @click="selectFolder">
            Выбрать эту папку
          </button>
        </div>
      </div>
    </div>
  </Transition>
</template>

<style scoped>
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  background: rgba(15, 23, 42, 0.6);
  backdrop-filter: blur(4px);
  z-index: 1000;
  display: flex;
  justify-content: center;
  align-items: center;
}

.modal-window {
  width: 600px;
  max-width: 90%;
  max-height: 80vh;
  background: var(--bg-card);
  border: 1px solid var(--border-color);
  border-radius: 12px;
  display: flex;
  flex-direction: column;
  box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.3), 0 10px 10px -5px rgba(0, 0, 0, 0.2);
  overflow: hidden;
}

.modal-header {
  padding: 16px 20px;
  border-bottom: 1px solid var(--border-color);
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.modal-title {
  font-weight: 600;
  font-size: 1.1rem;
  color: var(--text-primary);
}

.btn-close-modal {
  background: none;
  border: none;
  color: var(--text-secondary);
  cursor: pointer;
  padding: 4px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s;
}

.btn-close-modal:hover {
  background: rgba(255, 255, 255, 0.08);
  color: var(--text-primary);
}

.modal-body {
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 16px;
  flex: 1;
  overflow: hidden;
}

.current-path-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.path-label {
  font-size: 0.9rem;
  color: var(--text-secondary);
  font-weight: 500;
}

.path-input {
  flex: 1;
  background: var(--bg-app);
  border: 1px solid var(--border-color);
  border-radius: 6px;
  padding: 8px 12px;
  color: var(--text-primary);
  font-size: 0.9rem;
  font-family: monospace;
}

.path-input:focus {
  outline: none;
  border-color: var(--accent);
}

.folder-list-container {
  border: 1px solid var(--border-color);
  background: var(--bg-app);
  border-radius: 8px;
  height: 300px;
  overflow-y: auto;
  position: relative;
}

.folder-list-message,
.folder-list-error,
.folder-list-empty {
  padding: 40px 20px;
  text-align: center;
  color: var(--text-secondary);
  font-size: 0.9rem;
}

.folder-list-error {
  color: #ef4444;
}

.folder-list {
  display: flex;
  flex-direction: column;
}

.folder-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 16px;
  cursor: pointer;
  transition: background 0.15s;
  border-bottom: 1px solid rgba(255, 255, 255, 0.03);
}

.folder-item:hover {
  background: rgba(255, 255, 255, 0.05);
}

.parent-link {
  color: var(--text-secondary);
  font-style: italic;
}

.folder-icon {
  display: flex;
  align-items: center;
  justify-content: center;
}

.folder-name {
  font-size: 0.9rem;
  color: var(--text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.modal-footer {
  padding: 16px 20px;
  border-top: 1px solid var(--border-color);
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}

/* Анимация */
.modal-fade-enter-active,
.modal-fade-leave-active {
  transition: opacity 0.25s ease;
}

.modal-fade-enter-from,
.modal-fade-leave-to {
  opacity: 0;
}

.modal-fade-enter-active .modal-window {
  animation: scale-up 0.25s ease;
}

.modal-fade-leave-active .modal-window {
  animation: scale-down 0.2s ease;
}

@keyframes scale-down {
  from { transform: scale(1); opacity: 1; }
  to { transform: scale(0.95); opacity: 0; }
}

@media (max-width: 600px) {
  .modal-window {
    width: 95%;
    max-width: 95%;
    max-height: 90dvh;
    border-radius: 10px;
  }

  .modal-header {
    padding: 12px 14px;
  }

  .modal-title {
    font-size: 1rem;
  }

  .modal-body {
    padding: 12px;
    gap: 10px;
  }

  .current-path-row {
    flex-wrap: wrap;
  }

  .current-path-row .path-label {
    display: none;
  }

  .path-input {
    width: 100%;
    font-size: 12px;
  }

  .folder-list-container {
    height: 220px;
    max-height: 45dvh;
  }

  .folder-item {
    padding: 9px 12px;
  }

  .folder-name {
    font-size: 0.85rem;
  }

  .modal-footer {
    padding: 12px 14px;
    gap: 8px;
  }

  .modal-footer .btn {
    flex: 1;
    padding: 8px 12px;
  }
}
</style>
