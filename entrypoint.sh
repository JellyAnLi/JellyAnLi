#!/bin/sh
set -e

PUID=${PUID:-1000}
PGID=${PGID:-1000}

# Создаем группу, если ее нет
if ! getent group jellygroup >/dev/null 2>&1; then
    addgroup -g "$PGID" jellygroup 2>/dev/null || true
fi

# Создаем пользователя, если его нет
if ! getent passwd jellyuser >/dev/null 2>&1; then
    adduser -u "$PUID" -G jellygroup -h /config -s /bin/sh -D jellyuser 2>/dev/null || true
fi

# Назначаем владельца папки конфигов
chown -R "$PUID:$PGID" /config 2>/dev/null || true

# Передаем управление приложению от имени нужного PUID:PGID
exec su-exec jellyuser "$@"
