#!/bin/bash
set -e

TARGET="${1:-current}"

echo "=== Сборка Jellyfin Anime Linker (Target: $TARGET) ==="

# 1. Сборка фронтенда
if [ -d "frontend" ]; then
    echo "📦 Сборка фронтенда..."
    cd frontend
    if [ -d "node_modules" ]; then
        npm run build
    else
        echo "Установка зависимостей фронтенда и сборка..."
        npm install && npm run build
    fi
    cd ..
fi

mkdir -p bin

LDFLAGS="-s -w"

build_target() {
    local os=$1
    local arch=$2
    local ext=""
    if [ "$os" = "windows" ]; then
        ext=".exe"
    fi
    local output="bin/jelly-an-li-${os}-${arch}${ext}"
    echo "🔨 Компиляция под ${os}/${arch} -> ${output}..."
    CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -ldflags="$LDFLAGS" -o "$output" .
}

case "$TARGET" in
    windows)
        build_target "windows" "amd64"
        build_target "windows" "arm64"
        ;;
    linux)
        build_target "linux" "amd64"
        build_target "linux" "arm64"
        ;;
    darwin|macos)
        build_target "darwin" "arm64"
        build_target "darwin" "amd64"
        ;;
    all)
        build_target "windows" "amd64"
        build_target "windows" "arm64"
        build_target "linux" "amd64"
        build_target "linux" "arm64"
        build_target "darwin" "arm64"
        build_target "darwin" "amd64"
        ;;
    current|*)
        echo "🔨 Компиляция под текущую систему..."
        go build -ldflags="$LDFLAGS" -o bin/jelly-an-li .
        ;;
esac

echo ""
echo "=== ✨ Сборка завершена успешно ==="
ls -lh bin/
