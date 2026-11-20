# dev jwt

Offline JSON Web Token (JWT) decoder, claim timestamp inspector, and expiration verifier.

- **Source:** `commands/jwt.py`
- **Runtime:** Python 3 (standard library only)

---

## Overview

`dev jwt` decodes JWT headers and payloads completely offline without transmitting sensitive tokens over the network:

- **Flexible Input:** Accepts a positional token argument (automatically stripping a leading `Bearer ` prefix), piped `stdin`, or an interactive prompt when run without arguments in a terminal.
- **Human-Readable Timestamps:** Converts standard numeric claims (`iat`, `nbf`, `exp`, `auth_time`) into UTC, local system timezone, and relative time deltas (e.g., `in 1h 42m` or `3d 5h ago`).
- **Security & Format Checks:** Warns on unsecured tokens (`alg=none` or missing signature segments) and detects 5-segment encrypted JWE tokens.
- **CI / Script Support:** Provides `--json` for structured output and `--check` for automated expiration validation via exit codes.

---

## Usage

```bash
dev jwt [token] [options]
echo "$TOKEN" | dev jwt [options]
```

---

## Options

| Flag | Description |
| :--- | :--- |
| `token` | JWT string (with or without `Bearer ` prefix). Reads from `stdin` or prompts interactively if omitted. |
| `--json` | Output structured JSON (`status`, `header`, `payload`, `timestamps`, `warnings`) suitable for `jq`. |
| `--check` | Exit with status code `2` if the token is `EXPIRED` or `NOT_YET_VALID` (`nbf` in the future). |
| `--no-color` | Disable ANSI color output (also respects `NO_COLOR`). |
| `-h, --help` | Show help message and exit. |

---

## Examples

```bash
# Decode a JWT directly from the command line
dev jwt eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...

# Strip 'Bearer ' prefix automatically
dev jwt "Bearer eyJhbGciOiJSUzI1NiIs..."

# Pipe a token from clipboard or environment variable
echo "$ACCESS_TOKEN" | dev jwt

# Extract a specific claim with jq
dev jwt "$ACCESS_TOKEN" --json | jq -r '.payload.sub'

# Fail a CI script if a token is expired or not yet active
dev jwt "$ACCESS_TOKEN" --check
```

---

## Exit Codes

| Code | Meaning |
| :--- | :--- |
| `0` | Token decoded successfully (and valid if `--check` was passed). |
| `1` | No token provided, malformed Base64/JSON, or encrypted JWE token. |
| `2` | Token is `EXPIRED` or `NOT_YET_VALID` when `--check` is enabled. |
| `130` | Interactive prompt cancelled via `Ctrl+C` or `EOF`. |
