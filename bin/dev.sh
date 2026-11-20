#!/usr/bin/env bash
# DESC: Main polyglot CLI router with declarative language registry for dev-utils
set -euo pipefail

# Resolve real repository root even when invoked through symlinks
resolve_repo_root() {
  local source="${BASH_SOURCE[0]}"
  local dir=""
  while [[ -L "$source" ]]; do
    dir="$(cd -P "$(dirname "$source")" && pwd)"
    source="$(readlink "$source")"
    [[ "$source" != /* ]] && source="$dir/$source"
  done
  cd -P "$(dirname "$source")/.." && pwd
}

REPO_ROOT="$(resolve_repo_root)"
COMMANDS_DIR="$REPO_ROOT/commands"
CACHE_DIR="${XDG_CACHE_HOME:-$HOME/.cache}/dev-utils/bin"
VERSION="0.3.0"

# Configure colors only for interactive TTYs when NO_COLOR is unset
if [[ -t 1 && -z "${NO_COLOR:-}" ]]; then
  BOLD=$'\033[1m'
  DIM=$'\033[2m'
  CYAN=$'\033[36m'
  GREEN=$'\033[32m'
  YELLOW=$'\033[33m'
  RED=$'\033[31m'
  RESET=$'\033[0m'
else
  BOLD="" DIM="" CYAN="" GREEN="" YELLOW="" RED="" RESET=""
fi

# ==============================================================================
# DECLARATIVE LANGUAGE REGISTRY
# ==============================================================================
# Schema:
#   register_language <name> <exts> <shebangs> <comment> <mode> <env_exports> <runners>
#
# Modes:
#   direct   -> Run directly if executable (+x), otherwise invoke first installed runner
#   compile  -> Build/cache native binary into {cache} when missing/stale, then exec
#   runner   -> Execute via first installed runner template (or direct if executable)
#
# Runner spec (semicolon-separated in priority order):
#   "bin|cmd_template; fallback_bin|fallback_template"
#   Placeholders: {file} = command source path, {cache} = compiled binary path
# ==============================================================================

LANGUAGES=()
declare -A LANG_EXTS=()
declare -A LANG_SHEBANGS=()
declare -A LANG_COMMENT=()
declare -A LANG_MODE=()
declare -A LANG_ENV=()
declare -A LANG_RUNNERS=()

IGNORE_FILES=("LICENSE" "README" "Makefile" "Dockerfile")
IGNORE_EXTS=("md" "txt" "rst" "json" "yml" "yaml" "toml" "pyc" "pyo" "bak" "tmp")
DOCTOR_SYSTEM_TOOLS=("docker" "ss" "awk" "ps" "jq" "fzf" "shellcheck" "ruff")

register_language() {
  local name="$1"
  local exts="$2"
  local shebangs="$3"
  local comment="$4"
  local mode="$5"
  local env_exports="$6"
  local runners="$7"

  LANGUAGES+=("$name")
  LANG_EXTS["$name"]="$exts"
  LANG_SHEBANGS["$name"]="$shebangs"
  LANG_COMMENT["$name"]="$comment"
  LANG_MODE["$name"]="$mode"
  LANG_ENV["$name"]="$env_exports"
  LANG_RUNNERS["$name"]="$runners"
}

register_language \
  "bash" \
  "sh bash" \
  "bash sh" \
  "#" \
  "direct" \
  "" \
  "bash|bash {file}; sh|sh {file}"

register_language \
  "python" \
  "py pyw" \
  "python python3" \
  "#" \
  "direct" \
  "PYTHONSAFEPATH=1" \
  "python3|python3 {file}; python|python {file}"

register_language \
  "go" \
  "go" \
  "" \
  "//" \
  "compile" \
  "" \
  "go|go build -o {cache} {file}"

register_language \
  "rust" \
  "rs" \
  "rust-script" \
  "//" \
  "compile" \
  "" \
  "rustc|rustc -O -o {cache} {file}"

register_language \
  "javascript" \
  "js mjs cjs" \
  "node bun deno" \
  "//" \
  "direct" \
  "" \
  "node|node {file}; bun|bun run {file}; deno|deno run -A {file}"

register_language \
  "typescript" \
  "ts mts cts" \
  "tsx ts-node bun deno" \
  "//" \
  "runner" \
  "" \
  "tsx|tsx {file}; bun|bun run {file}; ts-node|ts-node {file}; deno|deno run -A {file}"

register_language \
  "ruby" \
  "rb" \
  "ruby" \
  "#" \
  "direct" \
  "" \
  "ruby|ruby {file}"

# Optional repo-level language overrides/additions
if [[ -f "$REPO_ROOT/.dev-languages.sh" ]]; then
  # shellcheck source=/dev/null
  source "$REPO_ROOT/.dev-languages.sh"
fi

# ==============================================================================
# LANGUAGE DETECTION & TEMPLATE HELPERS
# ==============================================================================

trim() {
  local s="$1"
  s="${s#"${s%%[![:space:]]*}"}"
  s="${s%"${s##*[![:space:]]}"}"
  printf '%s' "$s"
}

is_ignored_command_file() {
  local filename="$1"
  [[ "$filename" == .* ]] && return 0

  local item
  for item in "${IGNORE_FILES[@]}"; do
    [[ "$filename" == "$item" ]] && return 0
  done

  if [[ "$filename" == *.* ]]; then
    local ext="${filename##*.}"
    ext="${ext,,}"
    for item in "${IGNORE_EXTS[@]}"; do
      [[ "$ext" == "$item" ]] && return 0
    done
  fi
  return 1
}

parse_shebang_interpreter() {
  local line="$1"
  line="${line#\#!}"
  line="${line//$'\r'/}"
  local -a tokens=()
  read -ra tokens <<<"$line"
  [[ ${#tokens[@]} -gt 0 ]] || return 1

  local bin
  bin="$(basename "${tokens[0]}")"
  if [[ "$bin" == "env" ]]; then
    local i
    for ((i = 1; i < ${#tokens[@]}; i++)); do
      case "${tokens[$i]}" in
      -* | *=*) continue ;;
      *)
        bin="$(basename "${tokens[$i]}")"
        break
        ;;
      esac
    done
  fi
  printf '%s' "$bin"
}

detect_file_language() {
  local file="$1"
  local filename
  filename="$(basename "$file")"

  # 1. Inspect line-1 shebang first (works with or without file extensions)
  if [[ -f "$file" && -s "$file" ]]; then
    local shebang=""
    IFS= read -r shebang <"$file" 2>/dev/null || true
    if [[ "$shebang" == "#!"* ]]; then
      local interp=""
      interp="$(parse_shebang_interpreter "$shebang" || true)"
      if [[ -n "$interp" ]]; then
        local lang pattern
        for lang in "${LANGUAGES[@]}"; do
          for pattern in ${LANG_SHEBANGS[$lang]}; do
            if [[ "$interp" == "$pattern" || "$interp" == "$pattern"[0-9.]* ]]; then
              printf '%s' "$lang"
              return 0
            fi
          done
        done
      fi
    fi
  fi

  # 2. Fall back to registered file extensions
  if [[ "$filename" == *.* && "$filename" != .* ]]; then
    local ext="${filename##*.}"
    ext="${ext,,}"
    local lang candidate
    for lang in "${LANGUAGES[@]}"; do
      for candidate in ${LANG_EXTS[$lang]}; do
        if [[ "$ext" == "$candidate" ]]; then
          printf '%s' "$lang"
          return 0
        fi
      done
    done
  fi

  printf 'unknown'
}

extract_runner_names() {
  local spec_list="$1"
  [[ -n "$spec_list" ]] || {
    printf '%s' "-"
    return
  }
  local -a entries=() names=()
  local entry bin
  IFS=';' read -ra entries <<<"$spec_list"
  for entry in "${entries[@]}"; do
    entry="$(trim "$entry")"
    [[ -n "$entry" ]] || continue
    bin="$(trim "${entry%%|*}")"
    names+=("$bin")
  done
  local joined
  joined="$(
    IFS=', '
    printf '%s' "${names[*]}"
  )"
  printf '%s' "${joined:-"-"}"
}

find_available_runner() {
  local spec_list="$1"
  [[ -n "$spec_list" ]] || return 1

  local -a entries=()
  local entry bin cmd_tpl
  IFS=';' read -ra entries <<<"$spec_list"
  for entry in "${entries[@]}"; do
    entry="$(trim "$entry")"
    [[ -n "$entry" ]] || continue
    IFS='|' read -r bin cmd_tpl <<<"$entry"
    bin="$(trim "$bin")"
    cmd_tpl="$(trim "$cmd_tpl")"
    if command -v "$bin" >/dev/null 2>&1; then
      printf '%s|%s\n' "$bin" "$cmd_tpl"
      return 0
    fi
  done
  return 1
}

run_template() {
  local template="$1"
  local file="$2"
  local cache="${3:-}"
  local -a raw_parts=() cmd=()
  read -ra raw_parts <<<"$template"
  local part
  for part in "${raw_parts[@]}"; do
    part="${part//\{file\}/$file}"
    part="${part//\{cache\}/$cache}"
    cmd+=("$part")
  done
  "${cmd[@]}"
}

exec_template() {
  local template="$1"
  local file="$2"
  local cache="$3"
  shift 3
  local -a raw_parts=() cmd=()
  read -ra raw_parts <<<"$template"
  local part
  for part in "${raw_parts[@]}"; do
    part="${part//\{file\}/$file}"
    part="${part//\{cache\}/$cache}"
    cmd+=("$part")
  done
  exec "${cmd[@]}" "$@"
}

# ==============================================================================
# COMMAND DISCOVERY & METADATA
# ==============================================================================

list_command_files() {
  [[ -d "$COMMANDS_DIR" ]] || return 0
  local file base
  while IFS= read -r -d '' file; do
    base="$(basename "$file")"
    if ! is_ignored_command_file "$base"; then
      printf '%s\n' "$file"
    fi
  done < <(find "$COMMANDS_DIR" -maxdepth 1 -type f -print0 | LC_ALL=C sort -z)
}

# Derive clean subcommand name by stripping any file extension (e.g., jwt.py -> jwt)
command_name_for_file() {
  local base
  base="$(basename "$1")"
  printf '%s\n' "${base%.*}"
}

# Extract 'DESC:' header using the language's registered comment prefix (or # / //)
extract_desc() {
  local file="$1"
  local lang="${2:-unknown}"
  local prefix="#"
  if [[ "$lang" != "unknown" && -n "${LANG_COMMENT[$lang]:-}" ]]; then
    prefix="${LANG_COMMENT[$lang]}"
  fi

  awk -v pfx="$prefix" '
    NR <= 10 {
      line = $0
      sub(/^[[:space:]]+/, "", line)
      if (index(line, pfx) == 1) {
        rest = substr(line, length(pfx) + 1)
        if (rest ~ /^[[:space:]]*DESC:[[:space:]]*/) {
          sub(/^[[:space:]]*DESC:[[:space:]]*/, "", rest)
          print rest
          exit
        }
      }
      if (line ~ /^(#|\/\/)[[:space:]]*DESC:[[:space:]]*/) {
        sub(/^(#|\/\/)[[:space:]]*DESC:[[:space:]]*/, "", line)
        print line
        exit
      }
    }
  ' "$file" 2>/dev/null || true
}

# Resolve a subcommand (with or without extension) to its file path in commands/
find_command_file() {
  local query="$1"
  local file base stem fallback=""

  while IFS= read -r file; do
    [[ -n "$file" ]] || continue
    base="$(basename "$file")"
    stem="${base%.*}"

    if [[ "$base" == "$query" ]]; then
      printf '%s\n' "$file"
      return 0
    elif [[ "$stem" == "$query" && -z "$fallback" ]]; then
      fallback="$file"
    fi
  done < <(list_command_files)

  if [[ -n "$fallback" ]]; then
    printf '%s\n' "$fallback"
    return 0
  fi
  return 1
}

# ==============================================================================
# CLI VIEWS & INTERACTIVE LAUNCHER
# ==============================================================================

show_help() {
  printf '%sdev%s %s(v%s)%s — Polyglot developer utility belt\n\n' "$BOLD" "$RESET" "$DIM" "$VERSION" "$RESET"
  printf '%sUsage:%s\n' "$BOLD" "$RESET"
  printf '  dev <command> [args...]\n'
  printf '  dev [--help | --version | --list | --langs | --doctor]\n\n'
  printf '%sAvailable commands:%s\n' "$BOLD" "$RESET"

  local found=0 file name lang badge desc
  while IFS= read -r file; do
    [[ -n "$file" ]] || continue
    found=1
    name="$(command_name_for_file "$file")"
    lang="$(detect_file_language "$file")"
    desc="$(extract_desc "$file" "$lang")"
    badge="[$lang]"
    [[ "$lang" == "unknown" ]] && badge="[bin]"
    printf '  %s%-15s%s %s%-12s%s %s\n' \
      "$CYAN" "$name" "$RESET" \
      "$DIM" "$badge" "$RESET" \
      "${desc:-${DIM}No description${RESET}}"
  done < <(list_command_files)

  if [[ "$found" -eq 0 ]]; then
    printf '  %s(No commands found in %s)%s\n' "$DIM" "$COMMANDS_DIR" "$RESET"
  fi

  printf '\n%sTip:%s Run %sdev <command> --help%s for subcommand options.\n' "$DIM" "$RESET" "$CYAN" "$RESET"
}

print_language_table() {
  printf '%s%-12s %-14s %-16s %-9s %-24s %s%s\n' \
    "$BOLD" "LANGUAGE" "EXTENSIONS" "SHEBANGS" "MODE" "RUNNERS / COMPILERS" "STATUS" "$RESET"

  local lang runners status runner_hit
  for lang in "${LANGUAGES[@]}"; do
    runners="$(extract_runner_names "${LANG_RUNNERS[$lang]}")"
    if runner_hit="$(find_available_runner "${LANG_RUNNERS[$lang]}")"; then
      status="${GREEN}ready (${runner_hit%%|*})${RESET}"
    else
      status="${YELLOW}not installed${RESET}"
    fi
    printf '%-12s %-14s %-16s %-9s %-24s %s\n' \
      "$lang" \
      "${LANG_EXTS[$lang]:--}" \
      "${LANG_SHEBANGS[$lang]:--}" \
      "${LANG_MODE[$lang]}" \
      "$runners" \
      "$status"
  done
}

# Suggest the closest matching subcommand on typos using Python's difflib if available
suggest_command() {
  local unknown="$1"
  command -v python3 >/dev/null 2>&1 || return 0

  local candidates=() file
  while IFS= read -r file; do
    [[ -n "$file" ]] || continue
    candidates+=("$(command_name_for_file "$file")")
  done < <(list_command_files)

  [[ ${#candidates[@]} -gt 0 ]] || return 0

  PYTHONSAFEPATH=1 python3 - "$unknown" "${candidates[@]}" <<'PY'
import difflib, sys
query, pool = sys.argv[1], sys.argv[2:]
matches = difflib.get_close_matches(query, pool, n=1, cutoff=0.55)
if matches:
    print(matches[0])
PY
}

# Interactive launcher when 'dev' is run with no args in an interactive terminal
interactive_launcher() {
  local items=() file name lang desc
  while IFS= read -r file; do
    [[ -n "$file" ]] || continue
    name="$(command_name_for_file "$file")"
    lang="$(detect_file_language "$file")"
    desc="$(extract_desc "$file" "$lang")"
    items+=("$(printf '%-15s [%-10s] %s' "$name" "$lang" "${desc:-No description}")")
  done < <(list_command_files)

  [[ ${#items[@]} -gt 0 ]] || {
    show_help
    return 0
  }

  if command -v fzf >/dev/null 2>&1; then
    local choice selected_cmd
    choice="$(printf '%s\n' "${items[@]}" | fzf --height=40% --reverse --prompt="dev > " || true)"
    [[ -n "$choice" ]] || return 0
    selected_cmd="${choice%% *}"
    exec_command "$selected_cmd"
  else
    printf '%sdev%s %s(v%s)%s — Interactive Launcher\n\n' "$BOLD" "$RESET" "$DIM" "$VERSION" "$RESET"
    local i
    for ((i = 0; i < ${#items[@]}; i++)); do
      printf '  %s%2d)%s %s\n' "$CYAN" "$((i + 1))" "$RESET" "${items[$i]}"
    done
    printf '\n%sSelect a command number to run (or Enter to exit):%s ' "$BOLD" "$RESET"
    local reply idx
    read -r reply || return 0
    if [[ "$reply" =~ ^[0-9]+$ ]] && ((reply >= 1 && reply <= ${#items[@]})); then
      idx=$((reply - 1))
      selected_cmd="${items[$idx]%% *}"
      exec_command "$selected_cmd"
    fi
  fi
}

run_doctor() {
  printf '%sdev-utils environment check%s\n' "$BOLD" "$RESET"
  printf '  Repo root    : %s\n' "$REPO_ROOT"
  printf '  Commands dir : %s\n' "$COMMANDS_DIR"
  printf '  Build cache  : %s\n\n' "$CACHE_DIR"

  printf '%sRegistered language runtimes:%s\n' "$BOLD" "$RESET"
  print_language_table

  printf '\n%sSystem & developer tools:%s\n' "$BOLD" "$RESET"
  local tool status
  for tool in "${DOCTOR_SYSTEM_TOOLS[@]}"; do
    if command -v "$tool" >/dev/null 2>&1; then
      status="${GREEN}installed${RESET} ($(command -v "$tool"))"
    else
      status="${YELLOW}missing${RESET}"
    fi
    printf '  %-12s : %s\n' "$tool" "$status"
  done
}

# ==============================================================================
# SUBCOMMAND EXECUTION ENGINE
# ==============================================================================

apply_language_env() {
  local env_list="$1"
  [[ -n "$env_list" ]] || return 0
  local pair
  for pair in $env_list; do
    # shellcheck disable=SC2163
    export "$pair"
  done
}

exec_command() {
  local subcommand="$1"
  shift

  local target
  if ! target="$(find_command_file "$subcommand")"; then
    printf '%sError:%s Unknown command '\''%s'\''\n' "$RED" "$RESET" "$subcommand" >&2
    local suggestion
    suggestion="$(suggest_command "$subcommand")"
    if [[ -n "$suggestion" ]]; then
      printf 'Did you mean: %sdev %s%s?\n' "$CYAN" "$suggestion" "$RESET" >&2
    fi
    printf 'Run '\''dev --help'\'' to list available utilities.\n' >&2
    exit 1
  fi

  local lang
  lang="$(detect_file_language "$target")"

  if [[ "$lang" != "unknown" ]]; then
    apply_language_env "${LANG_ENV[$lang]:-}"

    local mode="${LANG_MODE[$lang]:-direct}"
    local runners="${LANG_RUNNERS[$lang]:-}"
    local runner_spec="" runner_tpl=""

    case "$mode" in
    compile)
      if ! runner_spec="$(find_available_runner "$runners")"; then
        printf '%sError:%s No compiler installed for [%s] (expected one of: %s)\n' \
          "$RED" "$RESET" "$lang" "$(extract_runner_names "$runners")" >&2
        exit 1
      fi
      runner_tpl="${runner_spec#*|}"
      local cache_bin
      cache_bin="$CACHE_DIR/$(command_name_for_file "$target")"
      if [[ ! -x "$cache_bin" || "$target" -nt "$cache_bin" ]]; then
        mkdir -p "$CACHE_DIR"
        run_template "$runner_tpl" "$target" "$cache_bin"
      fi
      exec "$cache_bin" "$@"
      ;;

    runner)
      if [[ -x "$target" ]]; then
        exec "$target" "$@"
      fi
      if ! runner_spec="$(find_available_runner "$runners")"; then
        printf '%sError:%s No runtime installed for [%s] (expected one of: %s)\n' \
          "$RED" "$RESET" "$lang" "$(extract_runner_names "$runners")" >&2
        exit 1
      fi
      runner_tpl="${runner_spec#*|}"
      exec_template "$runner_tpl" "$target" "" "$@"
      ;;

    direct)
      if [[ ! -x "$target" ]]; then
        chmod +x "$target" 2>/dev/null || true
      fi
      if [[ -x "$target" ]]; then
        exec "$target" "$@"
      fi
      if runner_spec="$(find_available_runner "$runners")"; then
        runner_tpl="${runner_spec#*|}"
        exec_template "$runner_tpl" "$target" "" "$@"
      fi
      ;;
    esac
  fi

  # Fallback for unregistered/prebuilt binaries or generic shebang scripts
  if [[ ! -x "$target" ]]; then
    chmod +x "$target" 2>/dev/null || true
  fi
  if [[ -x "$target" ]]; then
    exec "$target" "$@"
  fi

  local shebang=""
  IFS= read -r shebang <"$target" || true
  if [[ "$shebang" == "#!"* ]]; then
    local interp="${shebang#\#!}"
    # shellcheck disable=SC2086
    exec $interp "$target" "$@"
  fi

  printf '%sError:%s Command '\''%s'\'' is not executable. Run: ./scripts/make-executable.sh\n' "$RED" "$RESET" "$target" >&2
  exit 126
}

if [[ $# -eq 0 ]]; then
  if [[ -t 0 && -t 1 ]]; then
    interactive_launcher
  else
    show_help
  fi
  exit 0
fi

case "$1" in
-h | --help)
  show_help
  exit 0
  ;;
-v | --version)
  printf 'dev-utils %s\n' "$VERSION"
  exit 0
  ;;
--list)
  while IFS= read -r file; do
    [[ -n "$file" ]] || continue
    command_name_for_file "$file"
  done < <(list_command_files)
  exit 0
  ;;
--langs)
  print_language_table
  exit 0
  ;;
--doctor)
  run_doctor
  exit 0
  ;;
*)
  exec_command "$@"
  ;;
esac
