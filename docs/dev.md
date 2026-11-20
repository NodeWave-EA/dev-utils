# `bin/dev.sh` — Polyglot CLI Router

Main entrypoint (`dev`) for `dev-utils`. Resolves symlinks to locate the repository root, discovers subcommands in `commands/`, detects each utility's language via a declarative registry, compiles or invokes runtimes as needed, and provides interactive selection and environment diagnostics.

- **Source:** `bin/dev.sh`
- **Runtime:** Bash (`bash` 4.0+ for associative arrays)

---

## Usage

```bash
dev                             # Open interactive command launcher (in TTY) or show help
dev <command> [args...]         # Execute a subcommand from commands/
dev --sudo <command> [args...]  # Re-execute a subcommand with sudo (alias: dev sudo)
dev --help | -h                 # List discovered commands with runtime badges & descriptions
dev --version | -v              # Print dev-utils version
dev --list                      # Output raw subcommand names (used by tab-completion)
dev --langs                     # Print the active declarative language registry table
dev --doctor                    # Audit repository paths, language runtimes & system tools
```

---

## Privilege Elevation (`--sudo`)

When `dev-utils` is installed with `./install.sh --user` into `~/.local/bin/dev`, running `sudo dev <command>` fails with `sudo: dev: command not found` because `sudo` resets `$PATH` to `secure_path` (which excludes user home directories).

Use `dev --sudo <command> [args...]` (or `dev sudo <command> [args...]`) instead:

1. **Direct Path Resolution:** Resolves `$REPO_ROOT/bin/dev.sh` before invoking `sudo`, bypassing `secure_path` lookups.
2. **Cache Isolation:** When running under `sudo` (`SUDO_USER` is set), compiled Go/Rust utilities reuse up-to-date binaries from `~/.cache/dev-utils/bin/` if present, or compile into `${TMPDIR:-/tmp}/dev-utils-root-bin` so root never creates root-owned files inside your user cache directory.

---

## Command Discovery & Resolution

1. **Symlink Resolution:** Follows symlinks (`readlink`) from `/usr/local/bin/dev` or `~/.local/bin/dev` back to the real repository root so `commands/` is always found regardless of working directory.
2. **Extension Stripping:** Scans `commands/` (top-level files only) and strips file extensions so `commands/docker.go`, `commands/jwt.py`, and `commands/port.sh` are invoked as `dev docker`, `dev jwt`, and `dev port`. Exact filenames (e.g., `dev jwt.py`) also resolve.
3. **Ignored Files:** Automatically skips hidden dotfiles and non-executable metadata extensions (`.md`, `.txt`, `.rst`, `.json`, `.yml`, `.yaml`, `.toml`, `.pyc`, `.pyo`, `.bak`, `.tmp`, `LICENSE`, `README`, `Makefile`, `Dockerfile`).
4. **Metadata Extraction:** Reads the first 10 lines of each command for `DESC:` using the language's registered comment prefix (`# DESC:` or `// DESC:`) to populate `dev --help` and the interactive picker.
5. **Typo Suggestions:** When an unknown subcommand is entered and `python3` is installed, `difflib.get_close_matches` suggests the closest valid command (`Did you mean: dev <match>?`).

---

## Declarative Language Registry

Languages are registered via `register_language`:

```bash
register_language "<name>" "<exts>" "<shebangs>" "<comment>" "<mode>" "<env_exports>" "<runners>"
```

### Built-In Languages

| Language | Extensions | Shebangs | Mode | Env Exports | Runners / Compilers (Priority Order) |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **`bash`** | `sh bash` | `bash sh` | `direct` | — | `bash`, `sh` |
| **`python`** | `py pyw` | `python python3` | `direct` | `PYTHONSAFEPATH=1` | `python3`, `python` |
| **`go`** | `go` | — | `compile` | — | `go build -o {cache} {file}` |
| **`rust`** | `rs` | `rust-script` | `compile` | — | `rustc -O -o {cache} {file}` |
| **`javascript`** | `js mjs cjs` | `node bun deno` | `direct` | — | `node`, `bun`, `deno` |
| **`typescript`** | `ts mts cts` | `tsx ts-node bun deno` | `runner` | — | `tsx`, `bun`, `ts-node`, `deno` |
| **`ruby`** | `rb` | `ruby` | `direct` | — | `ruby` |

### Execution Modes

- **`direct`:** Attempts `chmod +x` if needed and `exec`s the file directly; falls back to the first installed interpreter in `<runners>` (useful on `noexec` mounts). Sets `PYTHONSAFEPATH=1` for Python so commands like `uuid.py` never shadow standard library modules.
- **`compile`:** Compiles single-file source into `${XDG_CACHE_HOME:-$HOME/.cache}/dev-utils/bin/<command>` when the binary is missing or the source file is newer (`-nt`), then `exec`s the cached binary.
- **`runner`:** Executes directly if `+x` is set, otherwise runs via the first available runtime template (`tsx`, `bun`, `ts-node`, `deno`).

### Customizing Languages

Create an optional `.dev-languages.sh` file in the repository root to register additional languages or override defaults:

```bash
# .dev-languages.sh
register_language \
  "lua" \
  "lua" \
  "lua luajit" \
  "#" \
  "direct" \
  "" \
  "luajit|luajit {file}; lua|lua {file}"
```