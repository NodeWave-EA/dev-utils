# Repository Roadmap

This roadmap tracks shipped utilities and planned additions for `dev-utils`. Each planned command includes a recommended implementation runtime aligned with our zero-dependency polyglot architecture (see [`STRUCTURE.md`](STRUCTURE.md)).

---

## Shipped Utilities

| Command | Runtime | Documentation | Capabilities |
| :--- | :--- | :--- | :--- |
| **`dev docker`** | Go | [`docs/docker.md`](docs/docker.md) | Simplified Docker & Compose lifecycle (`up`, `down`, `rebuild`, `ps`, `sh`, `logs`, `env`, `ip`, `clean`, `ctx`) + full CLI pass-through. |
| **`dev http`** | Go | [`docs/http.md`](docs/http.md) | Local HTTP webhook/echo inspector, mock & chaos fault server, static/SPA file server, and offline request replayer. |
| **`dev jwt`** | Python 3 | [`docs/jwt.md`](docs/jwt.md) | Offline JWT header/payload decoder, UTC/local claim timestamp formatter, `--json` output, and `--check` expiration exit codes. |
| **`dev port`** | Bash | [`docs/port.md`](docs/port.md) | Linux listening TCP/UDP socket inspector (`ss`) with interactive port browser and graceful `SIGTERM` $\rightarrow$ `SIGKILL` termination. |
| **`dev uuid`** | Python 3 | [`docs/uuid.md`](docs/uuid.md) | Cryptographic UUIDv4 and RFC 9562 time-ordered UUIDv7 generator, batch/clipboard support, and UUID metadata inspector. |

---

## Planned Utilities

### 1. Core Infrastructure & Developer Experience

- [ ] **`dev update`** *(Bash)* — Self-update helper that resolves `REPO_ROOT`, pulls latest changes (`git pull --ff-only`), re-applies `scripts/make-executable.sh`, invalidates stale compiled binaries in `~/.cache/dev-utils/bin/`, and prints newly added commands.
- [ ] **Zsh & Fish Completions** *(Bash / Shell)* — Extend `install.sh` to generate native tab-completion scripts for Zsh and Fish alongside Bash.
- [ ] **GitHub Actions CI Workflow** *(YAML)* — Run `./scripts/lint-and-format.sh check` on every pull request.

---

### 2. Security, Crypto & Encoding

- [ ] **`dev secret`** *(Python 3)* — Generate high-entropy API keys, URL-safe tokens, random passwords, hex strings, or HMAC secrets with optional clipboard copy.
- [ ] **`dev hash`** *(Python 3 / Go)* — Compute MD5, SHA-1, SHA-256, or SHA-512 checksums for strings, stdin streams, or files, plus checksum verification (`--check`).
- [ ] **`dev b64`** *(Python 3)* — Standard and URL-safe Base64 encoder/decoder with automatic padding normalization and stdin pipe support.
- [ ] **`dev url`** *(Python 3)* — URL-encode/decode strings and decompose full URLs into scheme, host, port, path, and sorted query parameters.
- [ ] **`dev ssl`** *(Go)* — Inspect remote TLS/SSL certificate chains (`domain[:port]`), expiration countdowns, SANs, issuer details, and negotiated TLS versions via `crypto/tls`.
- [ ] **`dev ssh-key`** *(Bash)* — Generate Ed25519 SSH keypairs, display key fingerprints, and copy public keys to clipboard.
- [ ] **`dev totp`** *(Python 3)* — Generate RFC 6238 2FA/TOTP codes locally from Base32 secrets with live countdown timers for testing authentication flows.

---

### 3. Data Formats & Serialization

- [ ] **`dev json`** *(Python 3)* — Pretty-print, minify, validate (with exact line/column error reporting), query dot-paths, and structurally diff two JSON payloads (`--diff`).
- [ ] **`dev yaml`** *(Python 3)* — Convert between JSON, YAML, and TOML payloads (`tomllib` stdlib + lightweight YAML support) and validate syntax.
- [ ] **`dev csv`** *(Python 3)* — Render CSV/TSV files as aligned terminal tables with column filtering or convert rows to JSON.
- [ ] **`dev mock`** *(Python 3)* — Generate realistic fake user profiles, emails, UUIDs, timestamps, and JSON arrays for local seeding and load tests.
- [ ] **`dev regex`** *(Python 3)* — Test regular expressions against strings or files with color-highlighted matches and numbered/named capture groups.

---

### 4. Networking & HTTP Debugging

- [x] **`dev http`** *(Go)* — Shipped as [`dev http`](docs/http.md).
- [x] **`dev serve`** *(Go)* — Shipped as [`dev http --serve <dir>`](docs/http.md).
- [ ] **`dev ip`** *(Bash / Go)* — Display local network interfaces, LAN IPv4/IPv6 addresses, default gateway, and public IP/ASN metadata.
- [ ] **`dev dns`** *(Go / Bash)* — Query `A`, `AAAA`, `CNAME`, `MX`, `NS`, and `TXT` records and compare propagation across public resolvers (`1.1.1.1`, `8.8.8.8`, `9.9.9.9`).
- [ ] **`dev ping-port`** *(Go)* — Concurrent TCP port reachability and connection latency tester (`host:port`) without requiring `telnet` or `nc`.
- [ ] **`dev speed`** *(Go)* — Quick terminal download/upload throughput and latency check.

---

### 5. Time & Date Utilities

- [ ] **`dev time`** *(Python 3)* — Display current timestamps across UTC, local timezone, ISO-8601, RFC 2822, and Unix epoch seconds/milliseconds/nanoseconds.
- [ ] **`dev epoch`** *(Python 3)* — Auto-detect epoch unit (`s`, `ms`, `us`, `ns`) and convert any Unix timestamp into human-readable UTC, local time, and relative offset.
- [ ] **`dev cron`** *(Python 3)* — Translate 5-field cron expressions (`*/15 * * * *`) into plain English and preview the next $N$ scheduled execution times.

---

### 6. Linux System & Process Helpers

- [ ] **`dev sys`** *(Bash)* — Compact terminal dashboard of Linux CPU load averages, RAM/swap utilization, disk usage, kernel version, and uptime.
- [ ] **`dev top-mem`** *(Bash)* — Display top memory- and CPU-consuming processes in a clean, human-readable table.
- [ ] **`dev path`** *(Bash)* — Print `$PATH` entries line-by-line, flagging missing directories, non-executable paths, and duplicates.
- [ ] **`dev env-check`** *(Python 3)* — Compare `.env` against `.env.example` to catch missing required keys, undocumented variables, or empty values.
- [ ] **`dev extract`** *(Bash)* — Universal archive extractor (`tar.gz`, `tgz`, `tar.bz2`, `tar.xz`, `tar.zst`, `zip`, `7z`, `gz`) with single-command ergonomics.

---

### 7. Git & Developer Workflow

- [ ] **`dev git-clean`** *(Bash)* — Interactively prune local branches already merged into `main`/`master` or whose upstream tracking branch is `[gone]`.
- [ ] **`dev git-undo`** *(Bash)* — Safe interactive helper to unstage files, amend the last commit message, or soft-reset the latest commit.
- [ ] **`dev git-wt`** *(Bash)* — Create, list, and clean up temporary `git worktree` directories for parallel PR reviews.
- [ ] **`dev git-stats`** *(Bash)* — Summarize repository commit frequency, top contributors, and high-churn files.
- [ ] **`dev ignore`** *(Python 3 / Go)* — Fetch or generate `.gitignore` templates (`python`, `go`, `node`, `linux`, `vscode`, `jetbrains`) directly from the terminal.

---

### 8. Containers & Cloud Helpers

- [x] **`dev docker-clean`** *(Go)* — Shipped as [`dev docker clean [--safe | --deep]`](docs/docker.md).
- [x] **`dev docker-ip`** *(Go)* — Shipped as [`dev docker ip [target]`](docs/docker.md).
- [ ] **`dev k8s-ctx`** *(Bash)* — Fast `fzf`/numbered switcher for `kubectl` contexts and namespaces.

---

## Contributing a Roadmap Utility

1. Pick an unchecked item above (or propose a new utility via issue/PR).
2. Follow the conventions in [`STRUCTURE.md`](STRUCTURE.md) (`commands/<name>.<ext>`, `DESC:` header, zero external runtime dependencies).
3. Add a matching reference guide in `docs/<name>.md`, check off the item in `ROADMAP.md`, and verify with `./scripts/lint-and-format.sh check`.
