# FlagMate — CTF Attack-Defence Suricata UI

Real-time traffic analysis UI for CTF attack-defence competitions. Integrates with Suricata's EVE JSON output to capture, analyse, and manage network flows.

## Features

- **Real-time flow capture** — View all network streams live via WebSocket
- **Stable flow detection** — Automatically groups similar flows (tokens/IDs masked)
- **Checker labeling** — Mark flows as checker or non-checker
- **Ban flagging** — Flag suspicious flows, copy iptables commands for manual action
- **Pattern matching** — Define regex patterns to auto-detect banned flows
- **Service management** — Add/remove monitored services dynamically with Suricata rule reload
- **Traffic mirroring** — Forward EVE JSON to other teams via TCP + raw packet mirroring via nfqueue
- **11 dark themes** — Midnight, Cyberpunk, Matrix, Dracula, Monokai, Nord, Tokyo Night, Gruvbox, Catppuccin, Oceanic, Solarized
- **Prometheus metrics** — `/metrics` endpoint for monitoring
- **NATS message bus** — Scalable real-time flow distribution
- **OpenAPI docs** — Full REST API specification

## Quick Start

```bash
# Set environment variables
cp .env.example .env
# Edit .env with your passwords

# Start everything
docker compose up -d

# Access UI
open http://localhost:3000
# API
open http://localhost:8080
# Metrics
open http://localhost:8080/metrics
# OpenAPI spec
open http://localhost:8080/openapi.json
```

## Architecture

```
Suricata → EVE JSON (Unix socket) → Go Backend → PostgreSQL + Redis
                                                → NATS → WebSocket → Vue Frontend
                                                → TCP Mirror → Other teams
                                                → nfqueue → Raw packet mirror
                                                → Prometheus /metrics
```

## Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `UI_PASSWORD` | Login password for the UI | `admin` |
| `JWT_SECRET` | JWT signing key | `supersecretjwtkey` |
| `POSTGRES_PASSWORD` | PostgreSQL password | `flagmate_pass` |
| `STABLE_THRESHOLD` | Occurrences to mark flow as stable | `5` |
| `EVE_SOCKET` | Path to Suricata EVE Unix socket | `/tmp/eve.sock` |
| `SURICATA_RULES_PATH` | Path to Suricata rules file | `/etc/suricata/rules/ctf.rules` |
| `SURICATA_PID` | PID of running Suricata for hot-reload | `0` (disabled) |
| `NATS_URL` | NATS server URL | `nats://nats:4222` |

## API Endpoints

See `openapi.json` for full specification. Key endpoints:

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/login` | Authenticate (password from env) |
| `GET` | `/services` | List monitored services |
| `POST` | `/services` | Add service (auto-generates Suricata rule) |
| `DELETE` | `/services/{id}` | Remove service |
| `GET` | `/flows` | List flows (paginated, searchable) |
| `GET` | `/flows/{id}` | Full flow details |
| `POST` | `/flows/{id}/label` | Toggle checker label |
| `POST` | `/flows/{id}/ban` | Flag as banned (only if response=200) |
| `GET` | `/patterns` | List ban patterns |
| `POST` | `/patterns` | Add regex pattern |
| `DELETE` | `/patterns/{id}` | Remove pattern |
| `GET` | `/flow-groups` | Get most frequent flow groups |
| `POST` | `/mirroring` | Update TCP mirror targets |
| `GET` | `/mirroring` | Get mirror config |
| `GET` | `/ws` | WebSocket for real-time flows |
| `GET` | `/metrics` | Prometheus metrics |

## Tech Stack

### Backend
- **Go 1.26** — High-performance concurrent backend
- **PostgreSQL** — Persistent flow storage (GORM ORM)
- **Redis** — Fast cache, pub/sub, hash counters
- **NATS** — Message bus for flow distribution
- **WebSocket** — Real-time UI updates
- **Prometheus** — Metrics collection

### Frontend
- **Vue 3 + TypeScript** — Reactive UI
- **Vite** — Fast build tool
- **Pinia** — State management
- **11 custom dark themes** — CSS custom properties
- **shadcn-style components** — Modern dark UI

### Deployment
- **Docker Compose** — Single-command local setup
- **Multi-stage builds** — Optimized container images

## Development

### Backend

```bash
cd backend
go mod tidy
go build ./cmd/server
go test ./... -v
```

### Frontend

```bash
cd frontend
npm install
npm run dev
npm run build
npx vitest run
```

## Project Structure

```
flagmate/
├── backend/
│   ├── cmd/server/main.go          # Entry point
│   ├── internal/
│   │   ├── api/                    # REST handlers + auth
│   │   ├── ban/                    # Ban evaluation logic
│   │   ├── eve/                    # Suricata EVE JSON reader
│   │   ├── flow/                   # Flow assembly + normalisation
│   │   ├── metrics/                # Prometheus metrics
│   │   ├── mirror/                 # TCP JSON mirroring
│   │   ├── models/                 # Data models
│   │   ├── natsbus/                # NATS message bus
│   │   ├── normaliser/             # Token masking regexes
│   │   ├── packetmirror/           # Raw packet mirroring (nfqueue)
│   │   ├── stable/                 # Stable flow detection
│   │   ├── store/                  # PostgreSQL + Redis
│   │   └── suricata/               # Rule management + reload
│   └── Dockerfile
├── frontend/
│   ├── src/
│   │   ├── components/             # Vue components
│   │   ├── composables/            # Vue composables
│   │   ├── pages/                  # Route pages
│   │   ├── stores/                 # Pinia stores
│   │   ├── themes/                 # 11 dark themes
│   │   ├── types/                  # TypeScript types
│   │   └── utils/                  # API client
│   └── Dockerfile
├── rules/
│   └── ctf.rules                   # Suricata rules
├── docker-compose.yml
├── openapi.json
└── README.md
```

## Mirroring

### TCP JSON Mirroring
Forwards raw EVE JSON lines to configured targets over plain TCP. Targets can be added/removed via the UI.

### Raw Packet Mirroring
Uses iptables/nfqueue to duplicate raw packets to secondary interfaces. Configure queue number via `PACKET_MIRROR_QUEUE` env var.

## Ban Workflow

1. Flow arrives from Suricata
2. Backend normalises payload (masks tokens, timestamps, UUIDs)
3. Checks if flow belongs to a stable group (hash frequency ≥ threshold)
4. If stable or manually marked as checker → evaluates against ban patterns
5. **Only if response_code = 200** → flags as banned
6. UI shows ban panel with "Copy iptables" button for manual action

## Themes

All 11 themes are dark-mode only, designed for long CTF sessions:

| Theme | Primary | Accent |
|-------|---------|--------|
| Midnight | `#6366f1` | `#22d3ee` |
| Cyberpunk | `#ff00ff` | `#00ffff` |
| Matrix | `#00ff41` | `#00ffaa` |
| Dracula | `#bd93f9` | `#ff79c6` |
| Monokai | `#a6e22e` | `#f92672` |
| Nord | `#88c0d0` | `#bf616a` |
| Tokyo Night | `#7aa2f7` | `#bb9af7` |
| Gruvbox Dark | `#b8bb26` | `#fe8019` |
| Catppuccin Mocha | `#cba6f7` | `#f5c2e7` |
| Oceanic Next | `#6699cc` | `#5fb3b3` |
| Solarized Dark | `#268bd2` | `#b58900` |
