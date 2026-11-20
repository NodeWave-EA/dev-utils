# dev http

Zero-dependency local HTTP webhook & echo inspector, mock/chaos latency server, static/SPA file server, and captured request replayer.

- **Source:** `commands/http.go`
- **Runtime:** Go (auto-compiled & cached by `bin/dev.sh`)

---

## Overview

`dev http` provides four local HTTP workflows in a single command:

1. **Webhook & Echo Inspector (Default):** Listens on `0.0.0.0:8080` (automatically incrementing the port if busy), prints incoming methods, paths, query parameters, headers, and syntax-highlighted JSON/form bodies, and echoes structured request metadata back to the caller.
2. **Mock & Chaos Fault Simulator:** Injects custom HTTP status codes, response bodies/files, custom headers, fixed latency, random jitter, and probabilistic error rates (`--fail-rate`) to test client retry and error handling.
3. **Static File & SPA Server:** Serves any local directory over HTTP with permissive CORS headers, optional Single Page App (`--spa`) fallback to `index.html`, `Cache-Control: no-store`, and HTTP Basic Auth.
4. **Offline Capture & Replay:** Saves live incoming requests as JSON files (`--save <dir>`) and replays them later against any local or staging URL (`dev http replay`).

---

## Usage

```bash
dev http [port | dir] [options]
dev http replay <captured-request.json> <target-base-url>
```

---

## Options

### Server & Network

| Flag | Default | Description |
| :--- | :--- | :--- |
| `-p, --port <port>` | `8080` | Port to listen on. Auto-increments (up to +20) if busy unless `--strict-port` is set. |
| `--host <addr>` | `0.0.0.0` | Network interface address to bind. |
| `--local` | — | Shorthand for `--host 127.0.0.1` (loopback only). |
| `--strict-port` | `false` | Fail immediately instead of trying the next port if `<port>` is already in use. |
| `--open` | `false` | Open the local server URL in the default browser on startup. |

### Static File & SPA Server

| Flag | Default | Description |
| :--- | :--- | :--- |
| `-s, --serve <dir>` | — | Serve static files from `<dir>` (can also be passed as a positional directory path). |
| `--spa` | `false` | Fallback unknown routes to `<dir>/index.html` for client-side routers. |
| `--cors` / `--no-cors` | `--cors` | Enable or disable `Access-Control-Allow-Origin: *` and automatic `OPTIONS` preflight responses. |
| `--no-cache` | `false` | Send `Cache-Control: no-store, no-cache, must-revalidate` on all responses. |
| `--auth <user:pass>` | — | Require HTTP Basic Authentication on all requests. |

### Mocking, Latency & Chaos Injection

| Flag | Default | Description |
| :--- | :--- | :--- |
| `--status <code>` | `200` | HTTP status code returned in echo/mock mode. |
| `--body <text>` | — | Custom response body string (auto-detects JSON `Content-Type`). |
| `--body-file <path>` | — | Serve response body from `<path>` (re-read per request so edits apply live). |
| `-H, --header 'K: V'` | — | Add a custom response header (repeatable). |
| `--delay <duration>` | `0s` | Fixed artificial delay before responding (e.g., `250ms`, `2s`). |
| `--jitter <duration>` | `0s` | Random additional delay between `0` and `<duration>`. |
| `--fail-rate <float>` | `0.0` | Fraction of requests (`0.0`–`1.0`) to randomly fail with `--fail-status`. |
| `--fail-status <code>` | `503` | HTTP status code returned when a simulated chaos failure triggers. |

### Inspection & Capture

| Flag | Default | Description |
| :--- | :--- | :--- |
| `--save <dir>` | — | Persist every incoming request to `<dir>/req-NNNN-YYYYMMDDTHHMMSSZ-METHOD.json`. |
| `--headers` / `--no-headers` | Auto | Show or hide request headers in terminal output (shown by default in echo mode). |
| `--compact` | `false` | Print single-line access logs instead of full multi-line request dumps. |
| `--max-body <bytes>` | `65536` | Maximum body bytes printed to terminal (`0` = unlimited). |
| `--no-color` | `false` | Disable ANSI colors (also respects `NO_COLOR`). |

---

## Built-In Endpoints

- **`GET /__health`**: In echo/mock mode (when neither `--body` nor `--body-file` is set), returns `200 OK` with `{"status":"ok"}` for readiness checks.

---

## Examples

```bash
# Start webhook inspector on port 8080 (displays localhost + LAN IP URLs)
dev http

# Listen on port 9000 and save every incoming webhook payload to ./webhooks/
dev http 9000 --save ./webhooks

# Replay a captured webhook against a local backend server
dev http replay ./webhooks/req-0001-20261119T095300Z-POST.json http://localhost:3000

# Serve a frontend build directory with SPA routing and disabled caching
dev http ./dist --spa --no-cache

# Simulate a rate-limited endpoint with latency jitter
dev http 8080 --status 429 --delay 500ms --jitter 300ms --body '{"error":"rate_limited"}'

# Inject 25% random 503 Service Unavailable failures
dev http 8080 --fail-rate 0.25 --fail-status 503

# Share a directory over LAN protected with HTTP Basic Auth
dev http ./artifacts --auth team:secret123
```
