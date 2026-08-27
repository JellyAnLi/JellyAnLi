# JellyAnLi (Jellyfin Anime Linker)

[Docker](#запуск-в-docker) | [Бинарник](#запуск-бинарником) | [Настройки](#настройки-configjson)

Автоматически наводит порядок в аниме-библиотеке **Jellyfin** с помощью симлинков, не ломая раздачу в торрентах.

---

### Зачем это нужно?

Торрент-раздачи аниме часто устроены хаотично: видео лежит отдельно, русские озвучки — в папках `RUS Sound/StudioBand/*.mka`, сабы — в `Subs/`, а сезоны разбиты по странным названиям.

**JellyAnLi** сканирует папки с торрентами, находит серии, подтягивает к ним внешние дорожки/субтитры, определяет правильные сезоны через базу Shikimori и делает симлинки в формате, который сразу понимает Jellyfin.

#### Пример:
```text
Торрент:
Downloads/Sousou no Frieren TV-2 [1080p]/
├── Sousou no Frieren TV-2 01.mkv
└── RUS Sound/StudioBand/01.mka

Jellyfin (симлинки):
Media/Аниме/Провожающая в последний путь Фрирен/
└── Season 02/
    ├── Sousou no Frieren S02E01.mkv
    └── Sousou no Frieren S02E01.StudioBand.ru.mka
```

---

### Что умеет

* **Привязка озвучек и сабов**: находит внешние аудио (`.mka`, `.flac`, `.ac3`) и субтитры (`.ass`, `.srt`), сопоставляет их с нужными сериями и проставляет языковые коды (`.ru.mka`, `.en.ass`).
* **Гибкие названия папок**: поддержка русских названий (*«Клинок, рассекающий демонов»*), официальных названий на Ромадзи (*«Kimetsu no Yaiba»*) или исходных имен из раздач.
* **Связка сезонов и франшиз**: сам понимает, к какой франшизе и какому сезону относится раздача.
* **Поддержка Proxy**: возможность работы через HTTP/HTTPS и SOCKS5/SOCKS5h (с DNS-пробросом) для обхода региональных ограничений.
* **Выравнивание сквозной нумерации**: если во 2-м сезоне серии названы как `13..24` или разбит на части `Part 2`, перенумерует их в `S02E01..S02E12`.
* **Относительные симлинки**: ссылки не ломаются при пробросе папок в Docker на Synology, Unraid, TrueNAS, CasaOS/ZimaOS или Linux.
* **Автоочистка**: когда торрент удаляется, JellyAnLi сам удаляет мертвые ссылки и пустые папки.
* **Удобный Web UI**: просмотр логов в реальном времени, предпросмотр изменений (Dry-Run), выбор папок и запуск по кнопке.

---

### Запуск в Docker

Самый удобный способ для сервера или NAS.

`docker-compose.yml`:
```yaml
services:
  jellyanli:
    image: ghcr.io/jellyanli/jellyanli:latest
    container_name: jellyanli
    restart: unless-stopped
    ports:
      - "37773:37773"
    environment:
      - TZ=Europe/Moscow
    volumes:
      - ./data:/config
      - /path/to/downloads:/torrents:ro
      - /path/to/jellyfin/anime:/media
```

Запуск:
```bash
docker compose up -d
```
Панель управления будет доступна по адресу: `http://IP-сервера:37773`.

---

### Запуск бинарником

Скачайте готовый файл для вашей системы со страницы [Releases](https://github.com/JellyAnLi/JellyAnLi/releases).

```bash
# Веб-интерфейс и фоновая синхронизация по таймеру
./jelly-an-li serve --port 37773

# Разовая синхронизация (например, для Cron)
./jelly-an-li sync

# Безопасный предпросмотр (без создания файлов)
./jelly-an-li sync --dry-run
```

---

### Настройки (`config.json`)

Настройки сохраняются в `config.json` (можно менять прямо из веб-интерфейса):

```json
{
  "torrent_dirs": [
    "/torrents/Anime"
  ],
  "library_dir": "/media/Anime",
  "sync_interval_minutes": 5,
  "folder_naming_mode": "russian",
  "proxy_url": "",
  "use_relative_symlinks": true,
  "use_shikimori": true,
  "language_mapping": {
    "RUS Sound": "ru",
    "RUS Subs": "ru",
    "Rus Dub": "ru",
    "Rus Sub": "ru",
    "ENG Sound": "en",
    "ENG Subs": "en"
  }
}
```

* `torrent_dirs` — список папок, куда качаются торренты.
* `library_dir` — папка аниме-медиатеки в Jellyfin (сериалы раскладываются по `Season 01`, `Season 02`, а фильмы и спешлы — в `Season 00` с файлами `.nfo`).
* `sync_interval_minutes` — интервал фоновой проверки новых раздач (в минутах).
* `folder_naming_mode` — стиль названий папок: `"russian"` (русские), `"romaji"` (ромадзи/английские) или `"original"` (исходные из раздачи).
* `proxy_url` — прокси для запросов к базам метаданных (`socks5://127.0.0.1:1080`, `socks5h://...`, `http://...`).
* `use_relative_symlinks` — создавать относительные симлинки (рекомендуется для Docker/NAS).
* `use_shikimori` — определять метаданные и структуру сезонов через базу Shikimori.
* `language_mapping` — соответствие папок с озвучками/сабами языковым тегам.

---

### Сборка из исходников

Нужен **Go 1.22+** и **Node.js 24+**:

```bash
git clone https://github.com/JellyAnLi/JellyAnLi.git
cd JellyAnLi
make build
```
Готовый бинарник со встроенным фронтендом появится в папке `bin/`.

---

### Лицензия

MIT
