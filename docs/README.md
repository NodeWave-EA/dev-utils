# Development Utilities Documentation

Reference guides for the CLI router, subcommands, and repository scripts in `dev-utils`.

- **Repository Overview:** [`README.md`](../README.md)
- **Repository Structure:** [`STRUCTURE.md`](../STRUCTURE.md)
- **Repository Roadmap:** [`ROADMAP.md`](../ROADMAP.md)

---

## Core Infrastructure & Scripts

| Component | Documentation | Source | Description |
| :--- | :--- | :--- | :--- |
| **`dev` CLI Router** | [`docs/dev.md`](dev.md) | `bin/dev.sh` | Polyglot command discovery, declarative language registry, build caching & diagnostics. |
| **Lint & Format** | [`docs/lint-and-format.md`](lint-and-format.md) | `scripts/lint-and-format.sh` | Table-driven polyglot formatter, syntax checker & linter backed by `scripts/.lint-languages.sh`. |
| **Permissions Helper** | [`docs/make-executable.md`](make-executable.md) | `scripts/make-executable.sh` | Applies `+x` executable permissions across entrypoints, commands & scripts. |

---

## Subcommands

| Command | Documentation | Source | Runtime | Summary |
| :--- | :--- | :--- | :--- | :--- |
| **`dev docker`** | [`docs/docker.md`](docker.md) | `commands/docker.go` | Go | Simplified Docker & Docker Compose manager with interactive pickers and full CLI pass-through. |
| **`dev http`** | [`docs/http.md`](http.md) | `commands/http.go` | Go | Local HTTP webhook/echo inspector, mock/chaos server, static/SPA file server & request replayer. |
| **`dev jwt`** | [`docs/jwt.md`](jwt.md) | `commands/jwt.py` | Python 3 | Offline JWT decoder, UTC/local claim timestamp inspector & expiration checker. |
| **`dev port`** | [`docs/port.md`](port.md) | `commands/port.sh` | Bash (`ss`) | Linux listening TCP/UDP port inspector & graceful `SIGTERM` $\rightarrow$ `SIGKILL` process terminator. |
| **`dev uuid`** | [`docs/uuid.md`](uuid.md) | `commands/uuid.py` | Python 3 | Cryptographic UUIDv4 & RFC 9562 UUIDv7 generator, clipboard helper & UUID metadata inspector. |
