#!/usr/bin/env bash
# DESC: Inspect listening Linux ports or terminate processes bound to a port
set -euo pipefail

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

show_usage() {
  cat <<EOF
Usage: dev port [port_number] [options]

Inspect or terminate Linux processes listening on TCP/UDP ports.
Run without a port number in an interactive terminal to browse active ports.

Options:
  -l, --list       List all listening TCP/UDP ports
  -k, --kill       Terminate process(es) listening on <port_number> (SIGTERM -> SIGKILL)
  -f, --force      Send SIGKILL (-9) immediately without graceful SIGTERM wait
  -y, --yes        Skip interactive confirmation prompt when killing
  -h, --help       Show this help message
EOF
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    printf '%sError:%s Required system command '\''%s'\'' not found.\n' "$RED" "$RESET" "$1" >&2
    exit 1
  fi
}

# List all listening sockets formatted as: proto|bind_addr|port|pids_csv
list_listening_sockets() {
  ss -H -tulpn 2>/dev/null | awk '
    {
      proto = $1
      local_ep = $5
      idx = match(local_ep, /:[0-9]+$/)
      if (idx == 0) next
      addr = substr(local_ep, 1, idx - 1)
      port = substr(local_ep, idx + 1)

      rest = ""
      for (i = 7; i <= NF; i++) rest = rest " " $i

      pids = ""
      while (match(rest, /pid=[0-9]+/)) {
        pid = substr(rest, RSTART + 4, RLENGTH - 4)
        if (index("," pids ",", "," pid ",") == 0) {
          pids = (pids == "" ? pid : pids "," pid)
        }
        rest = substr(rest, RSTART + RLENGTH)
      }
      print proto "|" addr "|" port "|" pids
    }
  ' | sort -t'|' -k3,3n -k1,1 -u
}

print_all_ports() {
  local rows
  rows="$(list_listening_sockets)"
  if [[ -z "$rows" ]]; then
    printf 'No listening TCP/UDP sockets found.\n'
    return 0
  fi

  printf '%s%-7s %-24s %-8s %-14s %s%s\n' "$BOLD" "PROTO" "BIND ADDRESS" "PORT" "PID(S)" "PROCESS" "$RESET"
  local proto addr port pids first_pid proc_name
  while IFS='|' read -r proto addr port pids; do
    [[ -n "$port" ]] || continue
    if [[ -n "$pids" ]]; then
      first_pid="${pids%%,*}"
      proc_name="$(ps -p "$first_pid" -o comm= 2>/dev/null || echo "unknown")"
    else
      pids="-"
      proc_name="${DIM}(requires sudo)${RESET}"
    fi
    printf '%-7s %-24s %s%-8s%s %-14s %s\n' "$proto" "$addr" "$CYAN" "$port" "$RESET" "$pids" "$proc_name"
  done <<<"$rows"
}

interactive_port_menu() {
  print_all_ports
  printf '\n%sEnter a port number to inspect/kill (or Enter to exit):%s ' "$BOLD" "$RESET"
  local chosen action
  read -r chosen || return 0
  [[ -n "$chosen" ]] || return 0

  if ! [[ "$chosen" =~ ^[0-9]+$ ]] || ((chosen < 1 || chosen > 65535)); then
    printf '%sError:%s Invalid port '\''%s'\''.\n' "$RED" "$RESET" "$chosen" >&2
    exit 1
  fi

  inspect_or_kill_port "$chosen" "false" "false" "false"
  printf '\n%sKill process(es) on port %s? [y/N]:%s ' "$YELLOW" "$chosen" "$RESET"
  read -r action || return 0
  if [[ "$action" =~ ^[Yy]$ ]]; then
    inspect_or_kill_port "$chosen" "true" "false" "true"
  fi
}

terminate_pid() {
  local pid="$1" force="$2"
  local comm
  comm="$(ps -p "$pid" -o comm= 2>/dev/null || echo "unknown")"

  if [[ "$force" == "true" ]]; then
    kill -9 "$pid"
    printf '%sSent SIGKILL to PID %s (%s).%s\n' "$GREEN" "$pid" "$comm" "$RESET"
    return 0
  fi

  kill -15 "$pid" 2>/dev/null || true
  local i
  for ((i = 0; i < 20; i++)); do
    if ! kill -0 "$pid" 2>/dev/null; then
      printf '%sGracefully stopped PID %s (%s) via SIGTERM.%s\n' "$GREEN" "$pid" "$comm" "$RESET"
      return 0
    fi
    sleep 0.1
  done

  kill -9 "$pid" 2>/dev/null || true
  printf '%sProcess %s (%s) ignored SIGTERM; sent SIGKILL.%s\n' "$YELLOW" "$pid" "$comm" "$RESET"
}

inspect_or_kill_port() {
  local target_port="$1" do_kill="$2" force="$3" auto_yes="$4"
  local matches
  matches="$(list_listening_sockets | awk -F'|' -v p="$target_port" '$3 == p')"

  if [[ -z "$matches" ]]; then
    printf 'No process found listening on port %s%s%s.\n' "$CYAN" "$target_port" "$RESET"
    return 0
  fi

  local all_pids=()
  local proto addr port pids pid
  printf '%sPort %s%s%s is active:%s\n' "$BOLD" "$CYAN" "$target_port" "$RESET$BOLD" "$RESET"

  while IFS='|' read -r proto addr port pids; do
    printf '  Socket : %s %s:%s\n' "${proto^^}" "$addr" "$port"
    if [[ -z "$pids" ]]; then
      printf '  %sOwner  : Hidden by kernel (re-run with sudo to inspect/kill)%s\n' "$YELLOW" "$RESET"
      continue
    fi
    IFS=',' read -ra pid_arr <<<"$pids"
    for pid in "${pid_arr[@]}"; do
      all_pids+=("$pid")
    done
  done <<<"$matches"

  local unique_pids=()
  if [[ ${#all_pids[@]} -gt 0 ]]; then
    mapfile -t unique_pids < <(printf '%s\n' "${all_pids[@]}" | sort -nu)
  fi

  if [[ ${#unique_pids[@]} -eq 0 ]]; then
    [[ "$do_kill" == "true" ]] && exit 1
    return 0
  fi

  for pid in "${unique_pids[@]}"; do
    local details
    details="$(ps -p "$pid" -o pid=,user=,etime=,args= 2>/dev/null || true)"
    if [[ -n "$details" ]]; then
      printf '  Process: %s\n' "$details"
    fi
  done

  if [[ "$do_kill" == "true" ]]; then
    if [[ "$auto_yes" != "true" && -t 0 ]]; then
      printf '\n%sTerminate PID(s) %s on port %s? [y/N]:%s ' "$YELLOW" "${unique_pids[*]}" "$target_port" "$RESET"
      local confirm
      read -r confirm || exit 130
      if [[ ! "$confirm" =~ ^[Yy]$ ]]; then
        printf 'Aborted.\n'
        exit 0
      fi
    fi

    for pid in "${unique_pids[@]}"; do
      terminate_pid "$pid" "$force"
    done
  fi
}

require_cmd ss
require_cmd awk
require_cmd ps

PORT=""
DO_LIST="false"
DO_KILL="false"
FORCE="false"
AUTO_YES="false"

while [[ $# -gt 0 ]]; do
  case "$1" in
  -h | --help)
    show_usage
    exit 0
    ;;
  -l | --list)
    DO_LIST="true"
    shift
    ;;
  -k | --kill)
    DO_KILL="true"
    shift
    ;;
  -f | --force)
    FORCE="true"
    DO_KILL="true"
    shift
    ;;
  -y | --yes)
    AUTO_YES="true"
    shift
    ;;
  -*)
    printf '%sError:%s Unknown option '\''%s'\''\n' "$RED" "$RESET" "$1" >&2
    show_usage >&2
    exit 1
    ;;
  *)
    if [[ -n "$PORT" ]]; then
      printf '%sError:%s Unexpected extra argument '\''%s'\''\n' "$RED" "$RESET" "$1" >&2
      exit 1
    fi
    PORT="$1"
    shift
    ;;
  esac
done

if [[ "$DO_LIST" == "true" ]]; then
  print_all_ports
  exit 0
fi

if [[ -z "$PORT" ]]; then
  if [[ -t 0 && -t 1 ]]; then
    interactive_port_menu
  else
    print_all_ports
  fi
  exit 0
fi

if ! [[ "$PORT" =~ ^[0-9]+$ ]] || ((PORT < 1 || PORT > 65535)); then
  printf '%sError:%s Port must be an integer between 1 and 65535.\n' "$RED" "$RESET" >&2
  exit 1
fi

inspect_or_kill_port "$PORT" "$DO_KILL" "$FORCE" "$AUTO_YES"
