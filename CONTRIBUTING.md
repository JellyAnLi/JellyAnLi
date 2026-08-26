# Руководство по участию в разработке (Contributing)

Спасибо за интерес к проекту **JellyAnLi (Jellyfin Anime Linker)**!

## Локальное окружение для разработки

### Требования
- **Go** 1.22+ (рекомендуется 1.24+)
- **Node.js** 20+ (рекомендуется 24+) и **npm**
- (Опционально) **Docker** и **docker compose**

---

### Запуск проекта локально

1. **Клонируйте репозиторий:**
   ```bash
   git clone https://github.com/JellyAnLi/JellyAnLi.git
   cd JellyAnLi
   ```

2. **Запуск Go-бэкенда:**
   ```bash
   make dev-backend
   # Запустит сервер с автоперезапуском через air (или go run . serve --port 37773)
   ```

3. **Запуск Vue 3 фронтенда (с Vite Hot Reload):**
   ```bash
   make dev-frontend
   # Запустит Vite-сервер на http://localhost:5173 с автопроксированием к бэкенду (:37773)
   ```

---

### Запуск тестов

Перед отправкой изменений убедитесь, что все тесты успешно проходят:
```bash
make test
# Или напрямую: go test -v -race ./...
```

---

### Сборка бинарников

* **Под текущую систему:**
  ```bash
  make build
  # Исполняемый файл появится в папке bin/jelly-an-li
  ```
* **Под Windows (.exe):**
  ```bash
  make build-windows
  # Соберёт bin/jelly-an-li-windows-amd64.exe и bin/jelly-an-li-windows-arm64.exe
  ```
* **Под Linux:**
  ```bash
  make build-linux
  ```
* **Под macOS:**
  ```bash
  make build-darwin
  ```
* **Под все платформы сразу:**
  ```bash
  make build-all
  ```

---

## Правила создания Pull Request

1. Создавайте отдельную ветку от `main` (для исправлений) или от `beta` (для нового функционала).
2. Один Pull Request — одна логическая задача или фикс.
3. Добавляйте unit-тесты для новых шаблонов названий или логики парсинга (`internal/parser/*_test.go`, `internal/linker/*_test.go`).
4. Используйте понятные сообщения коммитов (например: `feat: поддержка римских цифр в сезонах`, `fix: обработка пробелов в именах файлов`).
5. Заполните стандартный чек-лист [шаблона Pull Request](.github/PULL_REQUEST_TEMPLATE.md).
