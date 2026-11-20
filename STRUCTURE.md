# Repository Structure

`dev-utils` is organized as a modular, polyglot CLI monorepo. A single Bash entrypoint (`bin/dev.sh`) discovers standalone utilities in `commands/`, detects their language via a declarative registry, and executes or compiles them on demand.

---

## Directory Layout

```text
dev-utils/
├── bin/
│   └── dev.sh                  # Main polyglot CLI router & declarative language registry
├── commands/
│   ├── docker.go               # Simplified Docker & Docker Compose manager (Go)
│   ├── http.go                 # Local HTTP webhook inspector, mock & static server (Go)
│   ├── jwt.py                  # Offline JWT decoder, timestamp inspector & checker (Python 3)
│   ├── port.sh                 # Linux port inspector & graceful process terminator (Bash + ss)
│   └── uuid.py                 # UUIDv4 & time-ordered UUIDv7 generator/inspector (Python 3)
├── docs/
│   ├── README.md               # Documentation index
│   ├── dev.md                  # CLI router & language registry reference
│   ├── docker.md               # dev docker reference
│   ├── http.md                 # dev http reference
│   ├── jwt.md                  # dev jwt reference
│   ├── lint-and-format.md      # Polyglot linter & formatter reference
│   ├── make-executable.md      # Permissions helper reference
│   ├── port.md                 # dev port reference
│   └── uuid.md                 # dev uuid reference
├── scripts/
│   ├── .lint-languages.sh      # Declarative language -> formatter/checker/linter registry
│   ├── lint-and-format.sh      # Table-driven polyglot formatter, syntax checker & linter
│   └── make-executable.sh      # Applies chmod +x to entrypoints, commands & helper scripts
├── install.sh                  # Symlinks dev into /usr/local/bin or ~/.local/bin + completion
├── LICENSE                     # Project license
├── README.md                   # Quickstart, command index & links to docs
├── ROADMAP.md                  # Shipped utilities & planned feature backlog
└── STRUCTURE.md                # Repository layout, runtime lifecycle & conventions
```

*(Optional local file: `.dev-languages.sh` at the repository root is sourced by `bin/dev.sh` if present to register custom languages or runner overrides without modifying core router code.)*

---

## Component Responsibilities

### Root Entrypoints & Top-Level Docs

| Path | Role |
| :--- | :--- |
| **`install.sh`** | Creates the `dev` symlink in `/usr/local/bin/dev` (system-wide) or `~/.local/bin/dev` (`--user`), sets executable permissions, and installs Bash tab-completion. Supports `--uninstall`. |
| **`README.md`** | Concise landing page with installation steps, command index table, and links to `docs/`, `STRUCTURE.md`, and `ROADMAP.md`. |
| **`STRUCTURE.md`** | Architectural reference defining directory responsibilities, runtime execution modes, and contribution conventions. |
| **`ROADMAP.md`** | Tracks shipped utilities and upcoming backlog items grouped by engineering domain. |

### `bin/` — CLI Router

- **`bin/dev.sh`**: The sole entrypoint exposed on `$PATH` as `dev`.
  - Resolves symlinks (`readlink`) back to the real repository root so `commands/` is always located regardless of working directory.
  - Discovers top-level files in `commands/`, stripping file extensions so `jwt.py` and `docker.go` become `dev jwt` and `dev docker`.
  - Reads the first 10 lines of each file for `# DESC:` or `// DESC:` metadata to populate `dev --help` and the interactive `fzf` / numbered launcher.
  - Provides built-in introspection flags (`--help`, `--version`, `--list`, `--langs`, `--doctor`) and `difflib`-powered typo suggestions.

### `commands/` — Polyglot Subcommands

Each file in `commands/` is a self-contained CLI utility written in the language best suited to the task:

- **Bash (`*.sh` or extensionless):** Best for Linux process/network orchestration (`ss`, `ps`, `kill`, `git`).
- **Python 3 (`*.py`):** Best for text/data parsing, crypto/encoding, JSON/YAML/CSV manipulation, and standard-library tooling (`PYTHONSAFEPATH=1` is automatically exported by `bin/dev.sh` so filenames like `uuid.py` never shadow stdlib modules).
- **Go (`*.go`):** Best for concurrency, HTTP servers, TLS inspection, and external CLI orchestration (`docker`), auto-compiled to native binaries on first run.
- **Rust / Node.js / TypeScript / Ruby:** Supported out of the box via the language registry in `bin/dev.sh`.

### `scripts/` — Repository Maintenance & CI

- **`scripts/.lint-languages.sh`**: Declarative mapping of languages (`bash`, `python`, `javascript`, `typescript`, `go`, `rust`, `ruby`, `json`, `yaml`, `markdown`) to their formatters, syntax checkers, and linters.
- **`scripts/lint-and-format.sh`**: Reads `scripts/.lint-languages.sh`, detects file types via `#!` shebang first and file extension second, and runs formatting (`format`), non-mutating verification (`check`), or full pipelines (`all` / `--staged`).
- **`scripts/make-executable.sh`**: Applies `chmod +x` across `install.sh`, `bin/dev.sh`, `commands/*`, and `scripts/*`.

### `docs/` — Per-Command & Script Reference Guides

Every utility in `commands/` and core script in `bin/` or `scripts/` maintains a dedicated Markdown manual under `docs/<name>.md` covering overview, flags, exit codes, and real-world examples.

---

## Runtime Execution Lifecycle

When a user runs `dev <subcommand> [args...]`, `bin/dev.sh` executes the following pipeline:

```text
dev <subcommand> [args...]
  │
  ├─ 1. Resolve symlink -> REPO_ROOT/commands/<subcommand>[.<ext>]
  ├─ 2. Detect language via line-1 '#!' shebang, falling back to file extension
  ├─ 3. Export language-specific env vars (e.g. PYTHONSAFEPATH=1)
  └─ 4. Dispatch by registered execution mode:
        ├── direct  (bash, python, js, ruby) -> chmod +x if needed -> exec file (or interpreter)
        ├── compile (go, rust)               -> build to ~/.cache/dev-utils/bin/<cmd> if missing/stale -> exec binary
        └── runner  (typescript)             -> exec via first installed runner (tsx, bun, ts-node, deno)
```

### Build Cache Location

Compiled single-file commands (`go`, `rust`) store their binaries in:

```text
${XDG_CACHE_HOME:-$HOME/.cache}/dev-utils/bin/<command-name>
```

The binary is automatically rebuilt whenever the source file's modification timestamp is newer than the cached artifact (`-nt`).

---

## Conventions for Adding New Utilities

1. **Single-File Placement:** Place new commands directly in `commands/<name>.<ext>` (avoid nested subdirectories inside `commands/`, as discovery scans `-maxdepth 1`).
2. **Metadata Header:** Include a `#!` shebang on line 1 (for interpreted languages) and a `DESC:` summary within the first 10 lines (`# DESC: ...` or `// DESC: ...`).
3. **Zero External Runtime Dependencies:** Prefer standard libraries (`Python 3` stdlib, `Go` stdlib, standard Linux coreutils/iproute2) so commands work immediately on any developer machine without `pip install` or `npm install`.
4. **1-to-1 Documentation:** Create `docs/<name>.md` alongside every new command and register it in `docs/README.md`, `README.md`, and `ROADMAP.md`.
5. **Quality Gate:** Run `./scripts/make-executable.sh` and `./scripts/lint-and-format.sh check` before opening a pull request.
