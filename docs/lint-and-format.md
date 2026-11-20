# `scripts/lint-and-format.sh` — Polyglot Linter & Formatter

Table-driven code quality runner for `dev-utils`. Detects repository files by `#!` shebang or file extension and runs the appropriate formatter, syntax checker, and linter defined in `scripts/.lint-languages.sh`.

- **Source:** `scripts/lint-and-format.sh`
- **Language Registry:** `scripts/.lint-languages.sh`
- **Runtime:** Bash (`bash` 4.0+)

---

## Usage

```bash
./scripts/lint-and-format.sh                     # Format in place + run syntax & lint checks (default)
./scripts/lint-and-format.sh format              # Auto-format files in place only
./scripts/lint-and-format.sh check               # Non-mutating CI verification mode
./scripts/lint-and-format.sh --staged            # Inspect only Git staged files (pre-commit hook)
./scripts/lint-and-format.sh --list-langs        # Print configured language -> tool mapping table
./scripts/lint-and-format.sh [file1 file2 ...]   # Target specific files
```

---

## Execution Modes & Flags

| Argument / Flag | Description |
| :--- | :--- |
| `all` (default) | Formats files in place using the first healthy formatter, then runs all syntax checkers and linters. |
| `format` | Auto-formats files in place without running syntax checks or linters. |
| `check` | Verifies formatting, syntax, and lint rules without modifying files. Exits with code `1` on any mismatch or lint error (designed for CI). |
| `--staged` | Queries `git diff --cached --name-only --diff-filter=ACMR` so only staged files are checked/formatted. |
| `--list-langs` | Displays the table of registered languages, extensions, shebangs, formatters, checkers, and linters. |
| `-h, --help` | Displays CLI usage help. |

---

## Pipeline Stages

For every non-ignored file, the script executes three stages:

1. **Language Detection:** Inspects line 1 for a `#!` shebang first (handling `/usr/bin/env`, flags, and versioned binaries like `python3`), then falls back to file extension (including dotfiles with extensions such as `.lint-languages.sh`).
2. **Formatting:** Selects the **first** installed and healthy formatter from `LANG_FORMATTERS` (`bin|format_cmd|check_cmd|health_cmd`). If a tool's optional `health_cmd` fails (for example, a broken shared library), it logs a warning and falls back to the next formatter.
3. **Checking & Linting:** Runs **all** healthy installed syntax checkers (`LANG_CHECKERS`) and linters (`LANG_LINTERS`).

---

## Language Registry (`scripts/.lint-languages.sh`)

All language rules live in `scripts/.lint-languages.sh` using:

```bash
register_language "<lang>" "<exts>" "<shebangs>" "<formatters>" "<checkers>" "<linters>"
```

| Language | Extensions | Shebangs | Formatters | Checkers | Linters |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **`bash`** | `sh bash` | `bash sh` | `shfmt` | `bash -n` | `shellcheck` |
| **`python`** | `py pyw` | `python python3` | `ruff`, `black` | `py_compile` | `ruff`, `flake8` |
| **`javascript`** | `js mjs cjs` | `node bun deno` | `prettier` | `node --check` | `eslint` |
| **`typescript`** | `ts tsx mts cts` | `tsx ts-node` | `prettier` | — | `eslint` |
| **`go`** | `go` | — | `gofmt` | `go vet` | `golangci-lint` |
| **`rust`** | `rs` | `rust-script` | `rustfmt` | — | — |
| **`ruby`** | `rb` | `ruby` | `rubocop` | `ruby -c` | `rubocop` |
| **`json`** | `json jsonc` | — | `prettier` | `json.tool` | — |
| **`yaml`** | `yml yaml` | — | `prettier` | — | `yamllint` |
| **`markdown`** | `md markdown` | — | `prettier`, `mdl` | — | `markdownlint`, `mdl` |

### Git Pre-Commit Hook Setup

```bash
cat <<'EOF' > .git/hooks/pre-commit
#!/usr/bin/env bash
./scripts/lint-and-format.sh check --staged
EOF
chmod +x .git/hooks/pre-commit
```
