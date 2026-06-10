# Flagmate — презентация

## Слайд 1: Что такое Attack-Defence CTF?
Attack-Defence (AD) — это командный формат соревнований по кибербезопасности.
- Каждая команда получает одинаковый набор уязвимых сервисов.
- Задача: защищать свои сервисы и атаковать чужие.
- За каждую украденную команда получает флаг — строку вида `flag{...}`.
- Побеждает тот, кто собрал больше флагов и чьи сервисы дольше работали.

---

## Слайд 2: Проблема
В AD CTF критически важна скорость реакции:
- Атака может начаться в любой момент.
- Нужно мгновенно увидеть подозрительный трафик, понять вектор атаки и заблокировать его.
- Firegex пробивается DoS, если поставить слишком много или слишком широких regex правил.
- Packmate не умеет блокировать трафик и не имеет inline gate.
- Нет готового решения «все в одном» для AD.
- Нет готового решения «из коробки» для CTF AD.

---

## Слайд 3: Flagmate — обзор
Flagmate — это одно-командный деплояемый инструмент для анализа трафика в Attack-Defence CTF.
```
git clone https://github.com/Serebr1k-code/flagmate.git && cd flagmate && docker compose up -d
```
- Всё работает сразу: фронтенд, бэкенд, inline HTTP gate, Suricata.
- Никаких сторонних зависимостей и сложной настройки.

---

## Слайд 4: Ключевые возможности
- **Inline HTTP gate** — перехватывает и модифицирует ответы в реальном времени (замена на femboy media или рандомный флаг).
- **Suricata ingest** — сбор трафика через Unix-socket в реальном времени.
- **Service-scoped bans** — блокировка по содержимому, с разделением на режимы B/C/S.
- **Marks** — автоподсветка подозрительных паттернов (SQLi, RCE, SSRF, path traversal и т.д.).
- **Mirroring** — автоматическое повторение трафика на указанные цели, включая WebSocket-фреймы.

---

## Слайд 5: UI / Мониторинг
- **Flows** — лента трафика с бесконечной подгрузкой, группировкой по hash и свёрткой дубликатов.
- **Flow Detail** — split-pane просмотр с подсветкой marks, WebSocket frames, diff между попытками.
- **Stats** — граф атак (attacker → service endpoint → result), статистика украденных флагов, attack sessions.
- **Bans** — per-service управление правилами с детектором конфликтов.
- **Marks** — drag-reorder, colors, per-mark counts.

---

## Слайд 6: В реальном времени
- WebSocket live refresh: при появлении нового flow или изменении banned — все открытые клиенты обновляются мгновенно.
- Compromise alerts: уведомления сверху страницы при первом обнаружении утечки флага с сервиса.
- Фоновый пересчёт ban правил без блокировки UI.

---

## Слайд 7: Tech Stack
- **Backend**: Go (REST API, SQLite, gorilla/websocket, chi router)
- **Frontend**: Vue 3 + TypeScript + Vite
- **Suricata**: реальный NIDS, `unix_stream` socket
- **Deploy**: Docker Compose (backend, frontend, testservice, suricata)
- **One command**: `git clone + docker compose up`

---

## Слайд 8: Ссылки
- GitHub: https://github.com/Serebr1k-code/flagmate
- Документация: открыть Issues на GitHub для обратной связи
- Лицензия: MIT
- Контакты: https://t.me/serebr0k
