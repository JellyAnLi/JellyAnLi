# Changelog

All notable changes to this project will be documented in this file.
The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2026-08-23

### Added
- Multi-source torrent directory scanning.
- Automated anime franchise linking and episode alignment.
- Separate audio (`.mka`) and subtitle (`.ass`, `.srt`) association with automatic language tag injection.
- Shikimori API integration for Russian anime titles and multi-season franchise detection.
- Relative symlink mode support for Docker, Synology NAS, Unraid, and CasaOS/ZimaOS.
- Real-time Web UI with interactive collapsible tree view, live SSE log streaming, and directory browser.
- Background periodic sync daemon with configurable interval.
- Safe Dry-Run preview mode via Web UI and CLI.
- Docker multi-arch support (`linux/amd64`, `linux/arm64`).
- Comprehensive unit test suite with offline mock caches.
