#!/usr/bin/env bash
# DESC: Table-driven polyglot formatter, syntax checker, and linter (shebang + extension aware)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
LANG_REGISTRY_FILE="$SCRIPT_DIR/.lint-languages.sh"

MODE="all" # all | format | check
STAGED_ONLY="false"
TARGET_FILES=()

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
GRAY='\033[0;90m'
BOLD='\033[1m'
NC='\033[0m'

ERRORS=0
WARNINGS=0
CHECKED=0
SKIPPED=0

timestamp() { date +"%H:%M:%S"; }

log_info() { printf '%b\n' "${GRAY}[$(timestamp)]${NC} ${BLUE}[INFO]${NC}   $1"; }
log_step() { printf '%b\n' "${GRAY}[$(timestamp)]${NC} ${CYAN}[STEP]${NC}   $1"; }
log_detail() { printf '%b\n' "${GRAY}[$(timestamp)]            -> $1${NC}"; }
log_ok() { printf '%b\n' "${GRAY}[$(timestamp)]${NC} ${GREEN}[OK]${NC}     $1"; }
log_warn() {
  printf '%b\n' "${GRAY}[$(timestamp)]${NC} ${YELLOW}[WARN]${NC}   $1"
  WARNINGS=$((WARNINGS + 1))
}
log_err() {
  printf '%b\n' "${GRAY}[$(timestamp)]${NC} ${RED}[ERR]${NC}    $1"
  ERRORS=$((ERRORS + 1))
}

# ==============================================================================
# DECLARATIVE LANGUAGE REGISTRY LOADER
# ==============================================================================

LANGUAGES=()
declare -A LANG_EXTS=()
declare -A LANG_SHEBANGS=()
declare -A LANG_FORMATTERS=()
declare -A LANG_CHECKERS=()
declare -A LANG_LINTERS=()

IGNORE_FILES=(
  "LICENSE" "CHANGELOG" "Makefile" "Dockerfile"
  ".gitignore" ".gitattributes" ".dockerignore" ".editorconfig" ".gitkeep"
)
IGNORE_EXTS=("txt" "pyc" "pyo" "lock" "svg" "png" "jpg" "jpeg" "gif" "ico" "pdf")

register_language() {
  local name="$1"
  local exts="$2"
  local shebangs="$3"
  local formatters="$4"
  local checkers="$5"
  local linters="$6"

  LANGUAGES+=("$name")
  LANG_EXTS["$name"]="$exts"
  LANG_SHEBANGS["$name"]="$shebangs"
  LANG_FORMATTERS["$name"]="$formatters"
  LANG_CHECKERS["$name"]="$checkers"
  LANG_LINTERS["$name"]="$linters"
}

load_language_registry() {
  if [[ ! -f "$LANG_REGISTRY_FILE" ]]; then
    log_err "Language registry file not found: $LANG_REGISTRY_FILE"
    exit 1
  fi

  # shellcheck source=/dev/null
  source "$LANG_REGISTRY_FILE"

  if [[ ${#LANGUAGES[@]} -eq 0 ]]; then
    log_err "No languages registered in $LANG_REGISTRY_FILE"
    exit 1
  fi
}

# ==============================================================================
# DETECTION & EXECUTION ENGINE
# ==============================================================================

trim() {
  local s="$1"
  s="${s#"${s%%[![:space:]]*}"}"
  s="${s%"${s##*[![:space:]]}"}"
  printf '%s' "$s"
}

show_usage() {
  cat <<EOF
Usage: ./scripts/lint-and-format.sh [all | format | check] [--staged] [--list-langs] [file...]

Modes:
  all           Format files in place and run syntax/lint checks (default)
  format        Auto-format files in place only
  check         Verify formatting, syntax, and linters without modifying files (CI mode)

Options:
  --staged      Only inspect Git staged files (ideal for pre-commit hooks)
  --list-langs  Print the configured language -> formatter/checker/linter map
  -h, --help    Show this help message

Registry:
  Loaded from $LANG_REGISTRY_FILE
EOF
}

print_language_table() {
  printf '%b%-12s %-16s %-16s %-20s %-18s %s%b\n' \
    "$BOLD" "LANGUAGE" "EXTENSIONS" "SHEBANGS" "FORMATTERS" "CHECKERS" "LINTERS" "$NC"
  local lang
  for lang in "${LANGUAGES[@]}"; do
    printf '%-12s %-16s %-16s %-20s %-18s %s\n' \
      "$lang" \
      "${LANG_EXTS[$lang]:--}" \
      "${LANG_SHEBANGS[$lang]:--}" \
      "$(extract_tool_names "${LANG_FORMATTERS[$lang]}")" \
      "$(extract_tool_names "${LANG_CHECKERS[$lang]}")" \
      "$(extract_tool_names "${LANG_LINTERS[$lang]}")"
  done
}

extract_tool_names() {
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

get_extension() {
  local filename="$1"
  local stripped="${filename#.}"
  if [[ "$stripped" == *.* ]]; then
    local ext="${stripped##*.}"
    printf '%s' "${ext,,}"
  fi
}

is_ignored_file() {
  local filename="$1"
  local item
  for item in "${IGNORE_FILES[@]}"; do
    [[ "$filename" == "$item" ]] && return 0
  done

  local ext
  ext="$(get_extension "$filename")"
  if [[ -n "$ext" ]]; then
    for item in "${IGNORE_EXTS[@]}"; do
      [[ "$ext" == "$item" ]] && return 0
    done
  fi
  return 1
}

# Extract target interpreter name from line 1 shebang (supports /usr/bin/env & flags)
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

match_language_by_shebang() {
  local interp="$1"
  local lang pattern
  for lang in "${LANGUAGES[@]}"; do
    for pattern in ${LANG_SHEBANGS[$lang]}; do
      if [[ "$interp" == "$pattern" || "$interp" == "$pattern"[0-9.]* ]]; then
        printf '%s' "$lang"
        return 0
      fi
    done
  done
  return 1
}

match_language_by_extension() {
  local ext="$1"
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
  return 1
}

# Returns "language|detection_reason"
# Checks shebang first (works whether file has an extension or not), then extension.
detect_language() {
  local file="$1"
  local filename
  filename="$(basename "$file")"

  if is_ignored_file "$filename"; then
    printf 'ignore|ignored metadata/text file\n'
    return
  fi

  local ext="" ext_lang=""
  ext="$(get_extension "$filename")"
  if [[ -n "$ext" ]]; then
    ext_lang="$(match_language_by_extension "$ext" || true)"
  fi

  local shebang="" interp="" shebang_lang=""
  if [[ -f "$file" && -s "$file" ]]; then
    IFS= read -r shebang <"$file" 2>/dev/null || true
    if [[ "$shebang" == "#!"* ]]; then
      interp="$(parse_shebang_interpreter "$shebang" || true)"
      if [[ -n "$interp" ]]; then
        shebang_lang="$(match_language_by_shebang "$interp" || true)"
      fi
    fi
  fi

  if [[ -n "$shebang_lang" ]]; then
    if [[ -n "$ext_lang" && "$ext_lang" == "$shebang_lang" ]]; then
      printf '%s|shebang (%s) + extension (.%s)\n' "$shebang_lang" "$shebang" "$ext"
    elif [[ -n "$ext_lang" && "$ext_lang" != "$shebang_lang" ]]; then
      printf '%s|shebang (%s) overriding extension (.%s)\n' "$shebang_lang" "$shebang" "$ext"
    elif [[ -n "$ext" ]]; then
      printf '%s|shebang (%s) with custom extension (.%s)\n' "$shebang_lang" "$shebang" "$ext"
    else
      printf '%s|shebang (%s)\n' "$shebang_lang" "$shebang"
    fi
    return
  fi

  if [[ -n "$ext_lang" ]]; then
    printf '%s|extension (.%s)\n' "$ext_lang" "$ext"
    return
  fi

  printf 'unknown|no matching extension or shebang\n'
}

# Execute a command template safely without eval, substituting {file}
run_template() {
  local template="$1"
  local file="$2"
  local -a raw_parts=() cmd=()
  read -ra raw_parts <<<"$template"
  local part
  for part in "${raw_parts[@]}"; do
    cmd+=("${part//\{file\}/$file}")
  done
  "${cmd[@]}"
}

run_check_template() {
  local spec="$1"
  local file="$2"
  if [[ "$spec" == stdout_empty:* ]]; then
    local actual="${spec#stdout_empty:}"
    local out
    out="$(run_template "$actual" "$file")"
    [[ -z "$out" ]]
  else
    run_template "$spec" "$file" >/dev/null
  fi
}

run_language_pipeline() {
  local lang="$1"
  local file="$2"
  local rel="$3"

  local fmt_specs="${LANG_FORMATTERS[$lang]}"
  local chk_specs="${LANG_CHECKERS[$lang]}"
  local lnt_specs="${LANG_LINTERS[$lang]}"

  # 1. Formatting stage (uses first healthy installed formatter in priority order)
  if [[ -n "$fmt_specs" ]]; then
    local -a fmt_entries=()
    local entry bin fmt_cmd chk_cmd health_cmd ran_fmt="false"
    IFS=';' read -ra fmt_entries <<<"$fmt_specs"

    for entry in "${fmt_entries[@]}"; do
      entry="$(trim "$entry")"
      [[ -n "$entry" ]] || continue
      IFS='|' read -r bin fmt_cmd chk_cmd health_cmd <<<"$entry"
      bin="$(trim "$bin")"
      fmt_cmd="$(trim "$fmt_cmd")"
      chk_cmd="$(trim "$chk_cmd")"
      health_cmd="$(trim "$health_cmd")"

      command -v "$bin" >/dev/null 2>&1 || continue

      if [[ -n "$health_cmd" ]] && ! run_template "$health_cmd" "$file" >/dev/null 2>&1; then
        log_warn "$rel -> Skipping '$bin' formatter (health check '$health_cmd' failed)"
        continue
      fi

      ran_fmt="true"
      if [[ "$MODE" == "all" || "$MODE" == "format" ]]; then
        if [[ -n "$fmt_cmd" ]]; then
          log_detail "Formatting with '$fmt_cmd'..."
          if ! run_template "$fmt_cmd" "$file"; then
            log_err "$rel -> Formatter '$bin' failed"
            return
          fi
        fi
      elif [[ "$MODE" == "check" && -n "$chk_cmd" ]]; then
        log_detail "Checking formatting with '$chk_cmd'..."
        if ! run_check_template "$chk_cmd" "$file"; then
          log_err "$rel -> $lang formatting mismatch (run with 'format')"
          return
        fi
      fi
      break
    done

    if [[ "$ran_fmt" == "false" ]]; then
      log_detail "Skipping format (no healthy formatter from: $(extract_tool_names "$fmt_specs"))"
    fi
  fi

  [[ "$MODE" == "format" ]] && {
    log_ok "$rel -> Formatted $lang file"
    return
  }

  # 2. Syntax / static check stage (runs all healthy installed checkers)
  if [[ -n "$chk_specs" ]]; then
    local -a chk_entries=()
    local entry bin chk_cmd health_cmd
    IFS=';' read -ra chk_entries <<<"$chk_specs"

    for entry in "${chk_entries[@]}"; do
      entry="$(trim "$entry")"
      [[ -n "$entry" ]] || continue
      IFS='|' read -r bin chk_cmd health_cmd <<<"$entry"
      bin="$(trim "$bin")"
      chk_cmd="$(trim "$chk_cmd")"
      health_cmd="$(trim "$health_cmd")"

      command -v "$bin" >/dev/null 2>&1 || continue

      if [[ -n "$health_cmd" ]] && ! run_template "$health_cmd" "$file" >/dev/null 2>&1; then
        log_warn "$rel -> Skipping '$bin' check (runtime health check '$health_cmd' failed)"
        continue
      fi

      log_detail "Running check '$chk_cmd'..."
      if ! run_check_template "$chk_cmd" "$file"; then
        log_err "$rel -> $lang check failed ($bin)"
        return
      fi
    done
  fi

  # 3. Linter stage (runs all healthy installed linters)
  if [[ -n "$lnt_specs" ]]; then
    local -a lnt_entries=()
    local entry bin lnt_cmd health_cmd ran_lint="false"
    IFS=';' read -ra lnt_entries <<<"$lnt_specs"

    for entry in "${lnt_entries[@]}"; do
      entry="$(trim "$entry")"
      [[ -n "$entry" ]] || continue
      IFS='|' read -r bin lnt_cmd health_cmd <<<"$entry"
      bin="$(trim "$bin")"
      lnt_cmd="$(trim "$lnt_cmd")"
      health_cmd="$(trim "$health_cmd")"

      command -v "$bin" >/dev/null 2>&1 || continue

      if [[ -n "$health_cmd" ]] && ! run_template "$health_cmd" "$file" >/dev/null 2>&1; then
        log_warn "$rel -> Skipping '$bin' linter (runtime health check '$health_cmd' failed)"
        continue
      fi

      ran_lint="true"
      log_detail "Running linter '$lnt_cmd'..."
      if ! run_template "$lnt_cmd" "$file"; then
        log_err "$rel -> $bin reported issues"
        return
      fi
    done

    if [[ "$ran_lint" == "false" ]]; then
      log_detail "Skipping deep lint (no healthy linter from: $(extract_tool_names "$lnt_specs"))"
    fi
  fi

  log_ok "$rel -> Passed $lang checks"
}

process_file() {
  local file="$1"
  [[ -f "$file" ]] || return 0
  local rel="${file#"$REPO_ROOT/"}"

  local detection lang reason
  detection="$(detect_language "$file")"
  lang="${detection%%|*}"
  reason="${detection#*|}"

  if [[ "$lang" == "ignore" ]]; then
    log_detail "Ignoring '$rel' ($reason)"
    SKIPPED=$((SKIPPED + 1))
    return
  fi

  CHECKED=$((CHECKED + 1))
  log_step "Processing '$rel' -> detected as [$lang] via $reason"

  if [[ "$lang" == "unknown" ]]; then
    log_warn "$rel -> Unknown file type ($reason)"
    return
  fi

  run_language_pipeline "$lang" "$file" "$rel"
}

load_language_registry

while [[ $# -gt 0 ]]; do
  case "$1" in
  all | format | check)
    MODE="$1"
    shift
    ;;
  --staged)
    STAGED_ONLY="true"
    shift
    ;;
  --list-langs)
    print_language_table
    exit 0
    ;;
  -h | --help)
    show_usage
    exit 0
    ;;
  *)
    TARGET_FILES+=("$1")
    shift
    ;;
  esac
done

log_info "Starting lint & format run"
log_detail "Repository root : $REPO_ROOT"
log_detail "Language config : $LANG_REGISTRY_FILE"
log_detail "Execution mode  : $MODE"
echo ""

if [[ ${#TARGET_FILES[@]} -gt 0 ]]; then
  for f in "${TARGET_FILES[@]}"; do
    [[ "$f" != /* ]] && f="$REPO_ROOT/$f"
    process_file "$f"
    echo ""
  done
elif [[ "$STAGED_ONLY" == "true" ]]; then
  while IFS= read -r -d '' rel_path; do
    process_file "$REPO_ROOT/$rel_path"
    echo ""
  done < <(git -C "$REPO_ROOT" diff --cached --name-only --diff-filter=ACMR -z)
else
  while IFS= read -r -d '' file; do
    process_file "$file"
    echo ""
  done < <(find "$REPO_ROOT" \
    \( -name ".git" \
    -o -name "node_modules" \
    -o -name ".venv" \
    -o -name "venv" \
    -o -name "__pycache__" \
    -o -name ".ruff_cache" \
    -o -name ".pytest_cache" \
    -o -name ".mypy_cache" \) -prune -o \
    -type f -print0 | LC_ALL=C sort -z)
fi

log_info "================ SUMMARY ================"
log_detail "Files processed : $CHECKED"
log_detail "Files ignored   : $SKIPPED"
log_detail "Warnings        : $WARNINGS"
log_detail "Errors          : $ERRORS"
log_info "========================================="

if [[ $ERRORS -gt 0 ]]; then
  log_err "Run failed with $ERRORS error(s)."
  exit 1
else
  log_ok "All checks and formatting completed cleanly!"
fi
