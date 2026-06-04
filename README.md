# Flagmate

Flagmate is a lightweight CTF attack-defense traffic UI and inline response gate.
It ingests Suricata HTTP events in real time, shows live flows in the browser, and lets you ban service-specific words, regexes, or endpoint paths.

## One-command deploy

```bash
git clone https://github.com/Serebr1k-code/flagmate.git && cd flagmate && docker compose up -d
```

Open `http://<your-ip>:3000` and log in with `admin` unless you changed `UI_PASSWORD` in `.env`.

## What is included

- **Backend**: Go API, WebSocket live updates, SQLite storage, Suricata Unix socket ingest.
- **Frontend**: Vue 3 + TypeScript dashboard for flows, services, bans, groups, and mirroring.
- **Suricata**: bundled Compose service with `unix_stream` EVE output into the backend socket.
- **Inline HTTP gate**: proxies protected HTTP services and can replace banned responses before clients receive them.
- **Service-scoped bans**: every service has its own banned words, regexes, and endpoint/path rules.
- **Realtime flow history**: flow details load lazily in pages of 100 records for large repeated flows.
- **Poison responses**: browser-like clients get random images, non-browser clients get one fake flag-like line.

## Runtime ports

- `3000`: frontend UI
- `8080`: backend API
- `18080`: default inline HTTP gate port for the bundled test service

## Suricata in Docker

`docker compose up -d` starts Suricata too. The Suricata container uses host networking and captures `${SURICATA_INTERFACE:-lo}` by default.
On Linux attack-defense boxes this means the one-command deploy includes the UI, backend, gate, test service, and Suricata.

If your protected traffic is not on loopback, set the interface before starting:

```bash
SURICATA_INTERFACE=eth0 docker compose up -d
```

Suricata needs Linux capabilities (`NET_ADMIN`, `NET_RAW`, `SYS_NICE`) and host networking, so this setup is intended for Linux hosts.

## Suricata ingest

Flagmate does not poll `eve.json`. The backend creates a Unix socket listener and expects Suricata EVE events via `unix_stream`:

```text
/var/run/suricata/eve.sock
```

When `AUTO_HOOK_SURICATA=true`, the backend tries to reconcile `suricata.yaml` so an `eve-log` `unix_stream` output exists.

## Inline HTTP gate

With `GATE_ENABLED=true`, the backend listens on `GATE_LISTEN` and proxies to `GATE_UPSTREAM`.
If active bans for that service match the request path/body/headers or response body/status, Flagmate does not return the original upstream response.

Browser-like requests receive a random image from `assets/femboy` with a per-client limit of 10 images per minute. Non-browser requests receive one fake `flag{...}` line.

## Managing bans

Use the `Bans` tab:

1. Select a service.
2. Add a word, regex, or endpoint path rule.
3. Existing service-specific rules can be enabled, disabled, or deleted.

From a flow detail panel, `Ban Words` opens a picker where endpoint path fragments are pinned at the top and highlighted in blue.

## Environment

Important `.env` options:

```env
UI_PASSWORD=admin
JWT_SECRET=supersecretjwtkey
EVE_SOCKET=/var/run/suricata/eve.sock
DB_PATH=/data/flagmate.db
AUTO_HOOK_SURICATA=true
GATE_ENABLED=true
GATE_LISTEN=:18080
GATE_UPSTREAM=http://testservice:18080
POISON_IMAGE_DIR=/app/poison-images
SURICATA_INTERFACE=lo
```

## Architecture

```text
Client -> Flagmate HTTP gate -> Protected service
                    |
                    v
             ban decision engine -> poisoned response or original response

Suricata -> EVE unix_stream socket -> Flagmate backend -> SQLite -> Frontend WebSocket
```

## Development

```bash
docker compose up -d --build
```

The repository keeps the default docker-compose setup self-contained so the one-command deploy path remains valid.
