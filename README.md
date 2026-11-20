# Development Utilities

A centralized, project-agnostic command-line toolkit for Linux developers. Built around a lightweight polyglot Bash router (`dev`) backed by a declarative language registry, it lets engineers write and run everyday utilities in the best language for each job with zero third-party runtime dependencies.

---

## Installation (Linux)

1. **Clone the repository:**

   ```bash
   git clone <your-org-repo-url>/dev-utils.git
   cd dev-utils
   ```

2. **Run the installer:**

   ```bash
   # System-wide install (/usr/local/bin/dev) + Bash tab-completion
   ./install.sh

   # Or rootless user install (~/.local/bin/dev)
   ./install.sh --user
   ```

   *(To remove symlinks later, run `./install.sh --uninstall`.)*

3. **Verify your environment and available runtimes:**

   ```bash
   dev --doctor
   dev --help
   ```

---

## Available Commands

Full usage, flags, and examples for every command are documented in [`docs/`](docs/README.md):

| Command | Usage | Description | Runtime | Docs |
| :--- | :--- | :--- | :--- | :--- |
| **`dev docker`** | `dev docker [subcommand] [args]` | Simplified Docker & Docker Compose manager with interactive pickers and full `docker` pass-through. | Go | [`docs/docker.md`](docs/docker.md) |
| **`dev http`** | `dev http [port \| dir] [options]` | Local HTTP webhook/echo inspector, mock/chaos server, static/SPA server, and request replayer. | Go | [`docs/http.md`](docs/http.md) |
| **`dev jwt`** | `dev jwt [token] [--json] [--check]` | Decodes JWT headers/payloads offline, formats UTC/local timestamps, and checks expiration. | Python 3 | [`docs/jwt.md`](docs/jwt.md) |
| **`dev port`** | `dev port [port] [-l] [-k] [-f]` | Inspects listening TCP/UDP ports or terminates bound processes (`SIGTERM` $\rightarrow$ `SIGKILL`). | Bash (`ss`) | [`docs/port.md`](docs/port.md) |
| **`dev uuid`** | `dev uuid [--v4 \| --v7] [-n N] [-c]` | Generates UUIDv4 or time-ordered UUIDv7 IDs, copies to clipboard, or inspects existing UUIDs. | Python 3 | [`docs/uuid.md`](docs/uuid.md) |

---

## CLI Router & Repository Scripts

| Component | Path | Purpose | Docs |
| :--- | :--- | :--- | :--- |
| **CLI Router** | [`bin/dev.sh`](bin/dev.sh) | Discovers commands, detects languages via shebang/extension, auto-compiles Go/Rust, and provides interactive `fzf` selection, typo suggestions, and `--doctor` diagnostics. | [`docs/dev.md`](docs/dev.md) |
| **Lint & Format** | [`scripts/lint-and-format.sh`](scripts/lint-and-format.sh) | Runs formatters, syntax checkers, and linters across all repo languages configured in [`scripts/.lint-languages.sh`](scripts/.lint-languages.sh). | [`docs/lint-and-format.md`](docs/lint-and-format.md) |
| **Permissions Helper** | [`scripts/make-executable.sh`](scripts/make-executable.sh) | Restores `+x` executable permissions on entrypoints, commands, and scripts. | [`docs/make-executable.md`](docs/make-executable.md) |

---

## Project Guides

* **Documentation Index:** Browse all command and script reference guides in [`docs/README.md`](docs/README.md).
* **Repository Layout & Architecture:** See [`STRUCTURE.md`](STRUCTURE.md) for the directory tree, runtime execution lifecycle (`direct`, `compile`, `runner`), and conventions for adding new utilities.
* **Product & Engineering Roadmap:** See [`ROADMAP.md`](ROADMAP.md) for shipped tools and planned utilities across security, data formats, networking, Linux diagnostics, Git, and containers.

---

## Contributing

1. Fork or branch off `main`.
2. Add your command under `commands/` following the conventions in [`STRUCTURE.md`](STRUCTURE.md).
3. Add its reference guide under `docs/<command>.md` and update `README.md`, [`docs/README.md`](docs/README.md), and [`ROADMAP.md`](ROADMAP.md).
4. Run `./scripts/make-executable.sh` and `./scripts/lint-and-format.sh check`.
5. Submit a pull request.
