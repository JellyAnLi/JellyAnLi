.PHONY: build build-all build-windows build-linux build-darwin build-frontend test dev-frontend dev-backend clean

# Сборка под текущую ОС (бинарник со встроенным фронтендом)
build:
	./build.sh current

# Сборка фронтенда
build-frontend:
	cd frontend && npm run build

# Кросс-компиляция под все платформы (Windows, Linux, macOS)
build-all:
	./build.sh all

# Сборка под Windows (x64 и ARM64 .exe)
build-windows:
	./build.sh windows

# Сборка под Linux (x64 и ARM64)
build-linux:
	./build.sh linux

# Сборка под macOS (Apple Silicon и Intel)
build-darwin:
	./build.sh darwin

# Запуск тестов
test:
	go test ./...

# Dev режим: фронтенд с Vite Hot Reload
dev-frontend:
	cd frontend && npm run dev

# Dev режим: Go бэкенд (с Air или обычный запуск)
dev-backend:
	@if command -v air > /dev/null; then \
		air; \
	else \
		echo "=== [WARN] Утилита 'air' не найдена для Hot Reload бэкенда ==="; \
		echo "Для автоматического перезапуска при изменении .go кода установите её:"; \
		echo "  go install github.com/air-verse/air@latest"; \
		echo "Запускаем в обычном режиме..."; \
		go run . serve; \
	fi

# Очистка собранных бинарников
clean:
	rm -rf bin/
