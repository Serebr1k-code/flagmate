# FlagMate — CTF Attack-Defence Platform

## One-command deploy

```bash
git clone https://github.com/Serebr1k-code/flagmate.git && cd flagmate && docker compose up -d
```

Open `http://<your-ip>:3000` — login: `admin`

## Что внутри

- **Backend** — Go, Suricata EVE JSON, PostgreSQL, Redis, NATS
- **Frontend** — Vue 3 + TypeScript, 11 dark themes
- **Flow tracking** — real-time WebSocket, stable flow detection
- **Ban system** — regex patterns, word picker для выбора слов из трафика
- **Mirroring** — TCP JSON forwarding + raw packet mirroring (nfqueue)

## Services

Добавь сервисы через UI → Services tab. Каждый сервис = порт который мониторит Suricata.

## Architecture

```
Suricata → EVE JSON → Backend → PostgreSQL/Redis/NATS → Frontend (Vue 3)
```
