# dev docker

Simplified Docker and Docker Compose manager with interactive pickers, fuzzy container/service resolution, and transparent pass-through to native `docker` and `docker compose` commands.

- **Source:** `commands/docker.go`
- **Runtime:** Go (auto-compiled & cached by `bin/dev.sh`)

---

## Overview

`dev docker` reduces repetitive multi-flag Docker and Docker Compose commands into short, intuitive subcommands while preserving full access to the underlying Docker CLI:

- **Automatic Compose Detection:** Walks up from the current working directory to locate `compose.yaml`, `compose.yml`, `docker-compose.yaml`, or `docker-compose.yml` and automatically prefers `docker compose` (v2) with fallback to `docker-compose` (v1).
- **Fuzzy Target Resolution:** Accepts partial container names, Compose service names, container ID prefixes, or image names (e.g., `dev docker sh api` resolves `myproject-api-1`).
- **Interactive Pickers:** Uses `fzf` when installed (or a built-in numbered menu fallback) whenever a target container/service is omitted in an interactive terminal.
- **Transparent Pass-Through:** Any unrecognized subcommand or flag forwards directly to `docker` (and `dev docker dc ...` forwards directly to `docker compose`).

---

## Usage

```bash
dev docker                          # Open interactive dashboard & quick launcher
dev docker <shortcut> [args...]     # Run a simplified container or Compose workflow
dev docker dc <args...>             # Direct pass-through to 'docker compose'
dev docker <docker-cmd> [args...]   # Direct pass-through to native 'docker'
```

---

## Subcommands

### Compose Shortcuts

| Subcommand | Arguments | Description |
| :--- | :--- | :--- |
| **`up`** | `[services...] [--fg\|--attach] [-b]` | Starts the Compose stack in detached mode (`-d`) by default. Pass `--fg` to attach to stdout, or `-b` to rebuild images. |
| **`down`** | `[-v\|--volumes] [-y\|--yes]` | Stops and removes the Compose stack. Prompts for confirmation before deleting volumes (`-v`) unless `-y` is passed. |
| **`rebuild`** | `[services...]` | Rebuilds images and force-recreates containers (`docker compose up -d --build --force-recreate`). |
| **`build`** | `[services...]` | Runs `docker compose build` inside a Compose project, or builds `<dir-name>:latest` from `.` outside a Compose project. |
| **`dc`** / **`compose`** | `<args...>` | Passes all arguments directly to `docker compose` with `-f <detected-compose-file>` injected automatically. |

### Container & Debugging Shortcuts

| Subcommand | Aliases | Arguments | Description |
| :--- | :--- | :--- | :--- |
| **`ps`** | `ls`, `status` | `[-a\|--all] [--json]` | Displays a clean table of containers, Compose service names, images, statuses, and ports. |
| **`sh`** | `shell`, `bash`, `enter` | `[target] [cmd...]` | Opens an interactive shell (auto-detecting `/bin/bash`, `/bin/zsh`, `/bin/ash`, `/bin/sh`) or executes a command inside a container. |
| **`exec`** | — | `[target] <cmd...>` | Runs a command inside a fuzzy-matched container with `-it` attached in TTYs. |
| **`logs`** | `log` | `[target] [-f] [-n N] [--no-follow]` | Tails and follows logs (`--tail 100 -f` in TTYs). Supports `[ALL]` in Compose projects. |
| **`restart`** | — | `[targets...]` | Restarts a container, Compose service, or entire Compose stack. |
| **`stop`** | — | `[targets...]` | Stops a running container, Compose service, or Compose stack. |
| **`start`** | — | `[targets...]` | Starts a stopped container, Compose service, or Compose stack. |
| **`stats`** | — | `[targets...] [--no-stream]` | Streams live (or one-shot) CPU, memory, and network I/O metrics. |
| **`env`** | — | `[target] [--reveal]` | Prints sorted environment variables, masking secrets (`SECRET`, `PASSWORD`, `TOKEN`, `API_KEY`, etc.) unless `--reveal` is passed. |
| **`ip`** | `net` | `[target]` | Lists container bridge networks, internal IPv4 addresses, and published ports. |
| **`clean`** | `prune` | `[--safe \| --deep] [-y]` | Displays `docker system df` and prunes unused resources (`--safe` = dangling/stopped/build cache; `--deep` = all unused images & volumes). |
| **`ctx`** | `context` | `[name \| ls]` | Lists or interactively switches Docker contexts (`docker context use`). |

---

## Examples

```bash
# Launch the interactive control center
dev docker

# Start Compose stack in the background, then follow logs for 'api'
dev docker up
dev docker logs api

# Rebuild only the 'worker' service after dependency changes
dev docker rebuild worker

# Open a shell in a container (auto-selects bash/ash/sh)
dev docker sh postgres

# Inspect masked environment variables, then reveal plaintext values
dev docker env api
dev docker env api --reveal

# Reclaim disk space safely without removing tagged images or volumes
dev docker clean --safe

# Forward native Docker commands transparently
dev docker volume ls
dev docker inspect redis
```
