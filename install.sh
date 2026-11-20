#!/usr/bin/env bash
# DESC: Install dev-utils symlink and shell completions
set -euo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MODE="system" # system (/usr/local/bin) | user (~/.local/bin) | uninstall

while [[ $# -gt 0 ]]; do
  case "$1" in
  --user)
    MODE="user"
    shift
    ;;
  --system)
    MODE="system"
    shift
    ;;
  --uninstall)
    MODE="uninstall"
    shift
    ;;
  -h | --help)
    echo "Usage: ./install [--system | --user | --uninstall]"
    exit 0
    ;;
  *)
    echo "Error: Unknown option '$1'" >&2
    exit 1
    ;;
  esac
done

# Locate entrypoint whether named bin/dev or bin/dev.sh
ENTRYPOINT=""
for candidate in "$REPO_DIR/bin/dev" "$REPO_DIR/bin/dev.sh"; do
  if [[ -f "$candidate" ]]; then
    ENTRYPOINT="$candidate"
    break
  fi
done

if [[ -z "$ENTRYPOINT" ]]; then
  echo "Error: Could not find bin/dev or bin/dev.sh in $REPO_DIR" >&2
  exit 1
fi

if [[ "$MODE" == "uninstall" ]]; then
  for target in "/usr/local/bin/dev" "$HOME/.local/bin/dev"; do
    if [[ -L "$target" || -f "$target" ]]; then
      if [[ -w "$(dirname "$target")" ]]; then
        rm -f "$target"
      else
        sudo rm -f "$target"
      fi
      echo "Removed $target"
    fi
  done
  exit 0
fi

# Ensure all scripts are executable
if [[ -f "$REPO_DIR/scripts/make-executable.sh" ]]; then
  bash "$REPO_DIR/scripts/make-executable.sh"
else
  chmod +x "$ENTRYPOINT" "$REPO_DIR/commands/"*
fi

if [[ "$MODE" == "user" ]]; then
  TARGET_DIR="$HOME/.local/bin"
  mkdir -p "$TARGET_DIR"
  ln -sf "$ENTRYPOINT" "$TARGET_DIR/dev"
  echo "Installed symlink at $TARGET_DIR/dev"
  if [[ ":$PATH:" != *":$TARGET_DIR:"* ]]; then
    echo "Warning: $TARGET_DIR is not in your \$PATH."
  fi
else
  TARGET_DIR="/usr/local/bin"
  echo "Creating symlink in $TARGET_DIR/dev..."
  if [[ -w "$TARGET_DIR" ]]; then
    ln -sf "$ENTRYPOINT" "$TARGET_DIR/dev"
  else
    sudo ln -sf "$ENTRYPOINT" "$TARGET_DIR/dev"
  fi
fi

# Install dynamic Bash completion if user completion directory exists
BASH_COMP_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/bash-completion/completions"
mkdir -p "$BASH_COMP_DIR"
cat >"$BASH_COMP_DIR/dev" <<'EOF'
_dev_completions() {
  local cur="${COMP_WORDS[COMP_CWORD]}"
  if [[ $COMP_CWORD -eq 1 ]]; then
    local cmds
    cmds="$(dev --list 2>/dev/null) --help --version --list --doctor"
    mapfile -t COMPREPLY < <(compgen -W "$cmds" -- "$cur")
  fi
}
complete -F _dev_completions dev
EOF

echo ""
echo "Installation complete! Verifying:"
dev --version
