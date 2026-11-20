#!/usr/bin/env bash
# DESC: Make all CLI entrypoints, commands, and helper scripts executable
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

echo "Updating executable permissions in $REPO_ROOT..."

# 1. Core CLI entrypoint (supports bin/dev or bin/dev.sh)
for entry in "$REPO_ROOT/bin/dev" "$REPO_ROOT/bin/dev.sh"; do
  if [[ -f "$entry" ]]; then
    chmod +x "$entry"
    echo "  [OK] ${entry#"$REPO_ROOT/"}"
  fi
done

# 2. All utilities inside commands/ (ignoring docs and hidden files)
if [[ -d "$REPO_ROOT/commands" ]]; then
  while IFS= read -r -d '' cmd; do
    chmod +x "$cmd"
    echo "  [OK] commands/$(basename "$cmd")"
  done < <(find "$REPO_ROOT/commands" -maxdepth 1 -type f \
    ! -name ".*" ! -name "*.md" ! -name "*.txt" ! -name "*.json" -print0 | sort -z)
fi

# 3. Root installer and helper scripts
for script in "$REPO_ROOT/install" "$REPO_ROOT/install.sh" "$REPO_ROOT/scripts/"*.sh; do
  if [[ -f "$script" ]]; then
    chmod +x "$script"
    echo "  [OK] ${script#"$REPO_ROOT/"}"
  fi
done

echo "Done! All scripts are now executable."
