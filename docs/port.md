# dev port

Linux listening TCP/UDP port inspector and graceful process terminator.

- **Source:** `commands/port.sh`
- **Runtime:** Bash (`ss`, `awk`, `ps`)

---

## Overview

`dev port` inspects active network sockets using Linux `ss` and maps listening ports to process details (`PID`, `USER`, `ELAPSED`, and command line):

- **Interactive Port Browser:** Running `dev port` without arguments in a terminal lists all listening TCP/UDP ports and lets you pick a port to inspect or terminate.
- **IPv4, IPv6 & Multi-Worker Support:** Parses all socket families and resolves every PID bound to a port (such as multi-worker Gunicorn, Nginx, or Node servers).
- **Privilege Awareness:** Clearly indicates `(requires sudo)` when the Linux kernel hides process metadata for root-owned sockets instead of reporting the port as free.
- **Graceful Termination Escalation:** `--kill` sends `SIGTERM` (`-15`) first, polls for up to 2 seconds for clean shutdown, and escalates to `SIGKILL` (`-9`) only if the process refuses to exit.

---

## Usage

```bash
dev port                        # Interactive port browser & kill menu (in TTY)
dev port <port_number> [options]
```

---

## Options

| Flag | Description |
| :--- | :--- |
| `-l, --list` | List all listening TCP and UDP ports across the system. |
| `-k, --kill` | Terminate process(es) listening on `<port_number>` (`SIGTERM` $\rightarrow$ `SIGKILL`). |
| `-f, --force` | Send `SIGKILL` (`-9`) immediately without waiting for graceful `SIGTERM` exit (implies `--kill`). |
| `-y, --yes` | Skip the interactive `[y/N]` confirmation prompt when terminating processes. |
| `-h, --help` | Show help message and exit. |

---

## Examples

```bash
# Open interactive port list and optionally kill a selected port
dev port

# List all listening TCP/UDP sockets
dev port --list

# Inspect which process is occupying port 8080
dev port 8080

# Gracefully stop the process on port 3000 (with confirmation prompt)
dev port 3000 --kill

# Force-kill immediately without prompting (useful in scripts)
dev port 3000 --force --yes

# Inspect or kill privileged ports (<1024) owned by root
sudo dev port 80
```
